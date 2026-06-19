// Package dnfparser 是 ink.icoding.dnf:dnf-parser (PVF + NPK) 的纯 Go 单文件移植版本。
//
// 与原 Java 版本完全对齐：
//   - PVF：文件头解析、CRC 循环移位异或解密、文件树、stringtable/n_string 映射，
//     以及 bin/lst/str/ani/ui/default 六种脚本解析器，支持输出 JSON(map) 与还原源码文本。
//   - NPK：索引表解析、文件名固定 key 异或解密、V1/V2 贴图解码、ARGB(8888/1555/4444)
//     颜色还原、zlib 解压，以及导出 PNG 字节流。
//
// 唯一的第三方依赖：golang.org/x/text（用于 PVF / DB 文本编码转换）。
// 其余全部使用 Go 标准库（compress/zlib、image/png、encoding/binary 等）。
//
// 使用示例见文件末尾 Example* 函数。
package dnfparser

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	textencoding "golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

// =============================================================================
// 通用字节读取工具（对应 core 模块 BufferHelper / ByteHelper / StreamHelper）
// 全部按小端序，与 Java 端 ByteOrder.LITTLE_ENDIAN / hutool ByteUtil 默认一致。
// =============================================================================

// reader 是基于 []byte 的小端顺序读取器，等价于 Java 的 ByteBuffer(LITTLE_ENDIAN)。
type reader struct {
	data []byte
	pos  int
}

func newReader(data []byte) *reader { return &reader{data: data} }

func (r *reader) remaining() int { return len(r.data) - r.pos }

func (r *reader) hasRemaining() bool { return r.pos < len(r.data) }

func (r *reader) seek(pos int) { r.pos = pos }

func (r *reader) skip(n int) { r.pos += n }

func (r *reader) readBytes(n int) []byte {
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b
}

func (r *reader) readByte() byte {
	b := r.data[r.pos]
	r.pos++
	return b
}

func (r *reader) readInt() int32 {
	v := int32(binary.LittleEndian.Uint32(r.data[r.pos:]))
	r.pos += 4
	return v
}

func (r *reader) readUint16() uint16 {
	v := binary.LittleEndian.Uint16(r.data[r.pos:])
	r.pos += 2
	return v
}

// readShort 读取有符号 16 位（Java buffer.getShort() 返回 short，可为负）。
func (r *reader) readShort() int16 {
	return int16(r.readUint16())
}

func (r *reader) readFloat() float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(r.readBytes(4)))
}

// bytesToInt 等价 hutool ByteUtil.bytesToInt（小端）。
func bytesToInt(b []byte) int32 {
	return int32(binary.LittleEndian.Uint32(b))
}

func bytesToFloat(b []byte) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(b))
}

// =============================================================================
// 流式读取（对应 NPK 用到的 StreamHelper，基于 io.Reader）
// =============================================================================

type streamReader struct {
	r   io.Reader
	buf [8]byte
}

func newStreamReader(r io.Reader) *streamReader { return &streamReader{r: r} }

func (s *streamReader) readN(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(s.r, b); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *streamReader) readInt() (int32, error) {
	if _, err := io.ReadFull(s.r, s.buf[:4]); err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(s.buf[:4])), nil
}

// readCStr 读取以 0 结尾的字符串（对应 StreamHelper.readStr(InputStream)）。
func (s *streamReader) readCStr() (string, error) {
	var sb []byte
	one := make([]byte, 1)
	for {
		n, err := s.r.Read(one)
		if n == 0 || err != nil {
			break
		}
		if one[0] == 0 {
			break
		}
		sb = append(sb, one[0])
	}
	return string(sb), nil
}

func (s *streamReader) skip(n int64) error {
	if seeker, ok := s.r.(io.Seeker); ok {
		_, err := seeker.Seek(n, io.SeekCurrent)
		return err
	}
	_, err := io.CopyN(io.Discard, s.r, n)
	return err
}

// =============================================================================
// PVF 解密：crcDecrypt（最核心的位运算，必须逐位对齐）
// Java: key=0x81A79011; xor=key^crc32; val=anInt^xor;
//       decrypt = val >>> 6 | (int)((long)val << 26)
// 即对每 4 字节小端整数做 (val ^ xorKey) 后循环右移 6 位。
// =============================================================================

func crcDecrypt(data []byte, crc32 int32) {
	const key uint32 = 0x81A79011
	xor := key ^ uint32(crc32)
	n := len(data) - (len(data) % 4)
	for i := 0; i < n; i += 4 {
		anInt := binary.LittleEndian.Uint32(data[i:])
		val := anInt ^ xor
		// 32 位循环右移 6 位： (val >> 6) | (val << 26)
		decrypt := (val >> 6) | (val << 26)
		binary.LittleEndian.PutUint32(data[i:], decrypt)
	}
}

// =============================================================================
// 文本解码（PVF / DB 文本编码）
// =============================================================================

func DecodeBytes(b []byte, charset string) string {
	if len(b) == 0 {
		return ""
	}
	normalized := normalizeCharset(charset)
	if normalized == "" || normalized == "utf8" || normalized == "utf-8" {
		if utf8.Valid(b) {
			return string(b)
		}
		return string(b)
	}
	enc, ok := textEncoding(normalized)
	if !ok {
		return string(b)
	}
	out, _, err := transform.Bytes(enc.NewDecoder(), b)
	if err != nil {
		return string(b)
	}
	return string(out)
}

func normalizeCharset(charset string) string {
	return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(charset), "_", "-"), " ", ""))
}

func textEncoding(charset string) (textencoding.Encoding, bool) {
	switch normalizeCharset(charset) {
	case "big5", "big-5":
		return traditionalchinese.Big5, true
	case "gbk", "cp936":
		return simplifiedchinese.GBK, true
	case "gb18030", "gb-18030":
		return simplifiedchinese.GB18030, true
	default:
		return nil, false
	}
}

// =============================================================================
// PVF 数据结构
// =============================================================================

// PvfFile 对应 Java PvfFile，文件树中的一条索引。
type PvfFile struct {
	Number     int32
	PathLength int32
	Path       string
	Length     int32
	CRC32      int32
	Offset     int32
}

// pvfHeader 对应 Java PvfHeader。
type pvfHeader struct {
	guidLength int32
	guid       string
	version    int32
	treeLength int32
	treeCRC32  int32
	treeCount  int32
	tree       *reader // 已解密的目录树
}

// Pvf 对应 Java Pvf，是 PVF 解析的核心入口对象。
type Pvf struct {
	file        *os.File
	fileSize    int64
	contentBase int // 目录树之后的内容区起始偏移（对应 Java buffer.mark() 的位置）
	header      *pvfHeader
	treeDict    map[string][]*PvfFile // key 为目录前缀（小写），大小写不敏感匹配
	stringTable map[int32]string
	nString     map[string]map[string]string // 文件名 -> (key -> value)
	charset     string
}

// OpenPvf 打开并初始化一个 PVF 文件（对应 Java new Pvf(path, charset) + PvfCoder.initialize）。
func OpenPvf(path string) (*Pvf, error) {
	return OpenPvfWithCharset(path, "big5")
}

// OpenPvfWithCharset 打开并初始化一个 PVF 文件，charset 支持 big5 / gbk / gb18030 / utf-8。
func OpenPvfWithCharset(path string, charset string) (*Pvf, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	stat, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	p := &Pvf{
		file:     f,
		fileSize: stat.Size(),
		treeDict: make(map[string][]*PvfFile),
		charset:  charset,
	}
	if err := p.loadHeader(); err != nil {
		_ = p.Close()
		return nil, err
	}
	p.loadTree()
	if err := p.loadStringTable(); err != nil {
		_ = p.Close()
		return nil, err
	}
	if err := p.loadNString(); err != nil {
		_ = p.Close()
		return nil, err
	}
	return p, nil
}

func (p *Pvf) Close() error {
	if p == nil || p.file == nil {
		return nil
	}
	err := p.file.Close()
	p.file = nil
	return err
}

func (p *Pvf) decodeText(b []byte) string {
	return DecodeBytes(b, p.charset)
}

func (p *Pvf) loadHeader() error {
	if p.file == nil {
		return errors.New("PVF file is closed")
	}
	if _, err := p.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	r := newStreamReader(p.file)
	h := &pvfHeader{}
	var err error
	if h.guidLength, err = r.readInt(); err != nil {
		return err
	}
	if h.guidLength < 0 || int64(h.guidLength) > p.fileSize {
		return fmt.Errorf("invalid PVF guid length: %d", h.guidLength)
	}
	guidBytes, err := r.readN(int(h.guidLength))
	if err != nil {
		return err
	}
	h.guid = string(guidBytes)
	if h.version, err = r.readInt(); err != nil {
		return err
	}
	if h.treeLength, err = r.readInt(); err != nil {
		return err
	}
	if h.treeCRC32, err = r.readInt(); err != nil {
		return err
	}
	if h.treeCount, err = r.readInt(); err != nil {
		return err
	}
	if h.treeLength < 0 || int64(h.treeLength) > p.fileSize {
		return fmt.Errorf("invalid PVF tree length: %d", h.treeLength)
	}

	treeBytes := make([]byte, h.treeLength)
	if _, err := io.ReadFull(p.file, treeBytes); err != nil {
		return err
	}
	crcDecrypt(treeBytes, h.treeCRC32)
	h.tree = newReader(treeBytes)

	// Java 中 buffer.mark() 标记的位置：目录树之后即内容区起点。
	pos, err := p.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	p.contentBase = int(pos)
	p.header = h
	return nil
}

func (p *Pvf) loadTree() {
	t := p.header.tree
	for i := int32(0); i < p.header.treeCount; i++ {
		fileNumber := t.readInt()
		pathLength := t.readInt()
		filePath := normalizeResourcePath(string(t.readBytes(int(pathLength))))
		// (raw + 3) & 0xFFFFFFFC 做 4 字节对齐
		fileLength := (t.readInt() + 3) & ^int32(3)
		crc := t.readInt()
		offset := t.readInt()

		f := &PvfFile{
			Number:     fileNumber,
			PathLength: pathLength,
			Path:       filePath,
			Length:     fileLength,
			CRC32:      crc,
			Offset:     offset,
		}
		p.putTreeFile(f)
	}
}

func rootPathOf(path string) string {
	if strings.Index(path, "/") > 0 {
		return path[:strings.LastIndex(path, "/")]
	}
	return "/"
}

func (p *Pvf) putTreeFile(f *PvfFile) {
	root := strings.ToLower(rootPathOf(f.Path))
	p.treeDict[root] = append(p.treeDict[root], f)
}

func baseName(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

func normalizeResourcePath(path string) string {
	path = strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
	path = strings.TrimLeft(path, "/")
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}
	return strings.ToLower(path)
}

func (p *Pvf) getTreeFile(path string) *PvfFile {
	path = normalizeResourcePath(path)
	root := strings.ToLower(rootPathOf(path))
	files := p.treeDict[root]
	if len(files) == 0 {
		return nil
	}
	for _, f := range files {
		if strings.EqualFold(f.Path, path) {
			return f
		}
	}
	// 兼容 (r)/(f) 前缀重定向文件
	fileName := baseName(path)
	for _, f := range files {
		tn := baseName(f.Path)
		if strings.EqualFold(tn, "(r)"+fileName) || strings.EqualFold(tn, "(f)"+fileName) {
			return f
		}
	}
	return nil
}

// IsExist 判断脚本是否存在（对应 Java isExist，仅精确匹配，不含 (r)/(f) 兼容）。
func (p *Pvf) IsExist(path string) bool {
	path = normalizeResourcePath(path)
	root := strings.ToLower(rootPathOf(path))
	for _, f := range p.treeDict[root] {
		if strings.EqualFold(f.Path, path) {
			return true
		}
	}
	return false
}

// getTreeContent 读取并 CRC 解密某个脚本的原始字节。
func (p *Pvf) getTreeContent(path string) []byte {
	path = normalizeResourcePath(path)
	f := p.getTreeFile(path)
	if f == nil {
		return nil
	}
	if p.file == nil || f.Offset < 0 || f.Length < 0 {
		return nil
	}
	start := int64(p.contentBase) + int64(f.Offset)
	end := start + int64(f.Length)
	if start < 0 || end > p.fileSize {
		return nil
	}
	content := make([]byte, int(f.Length))
	if _, err := p.file.ReadAt(content, start); err != nil {
		return nil
	}
	crcDecrypt(content, f.CRC32)
	return content
}

func (p *Pvf) loadStringTable() error {
	content := p.getTreeContent("stringtable.bin")
	if content == nil {
		return errors.New("未找到 stringtable.bin")
	}
	dict := binParse(p, content)
	p.stringTable = make(map[int32]string, len(dict))
	for k, v := range dict {
		// bin 的 key 是字符串化的下标 "0","1"...
		var idx int32
		fmt.Sscanf(k, "%d", &idx)
		p.stringTable[idx] = v.(string)
	}
	return nil
}

func (p *Pvf) loadNString() error {
	content := p.getTreeContent("n_string.lst")
	if content == nil {
		return errors.New("未找到 n_string.lst")
	}
	lst := lstParse(p, content) // index -> str 文件路径
	p.nString = make(map[string]map[string]string)
	for _, v := range lst {
		fileName := normalizeResourcePath(v)
		strContent := p.getTreeContent(fileName)
		if strContent == nil {
			continue
		}
		p.nString[fileName] = strParse(p, strContent)
	}
	return nil
}

// getStringTable 对应 Java getStringTable。
func (p *Pvf) getStringTable(key int32) string {
	return p.stringTable[key]
}

// getNString 对应 Java getNString（按文件前缀精确匹配）。
func (p *Pvf) getNString(filePrefix string, key string) string {
	prefix := normalizeFilePrefix(filePrefix)
	if prefix == "" {
		return ""
	}
	// 为稳定输出，对 key 做有序遍历
	names := make([]string, 0, len(p.nString))
	for name := range p.nString {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !strings.HasPrefix(name, prefix+"/") {
			continue
		}
		if val, ok := p.nString[name][key]; ok {
			return val
		}
	}
	return ""
}

func normalizeFilePrefix(filePrefix string) string {
	if filePrefix == "" {
		return ""
	}
	prefix := strings.TrimSpace(strings.ToLower(strings.ReplaceAll(filePrefix, "\\", "/")))
	if i := strings.Index(prefix, "/"); i >= 0 {
		prefix = prefix[:i]
	}
	return prefix
}

// =============================================================================
// PVF 脚本类型分发（对应 ScriptType）
// =============================================================================

func suffixOf(path string) string {
	base := baseName(path)
	if i := strings.LastIndex(base, "."); i >= 0 {
		return base[i+1:]
	}
	return ""
}

// LoadScript 返回脚本的结构化数据（对应 Java loadScript，输出 JSONObject -> 此处为 *OrderedMap）。
func (p *Pvf) LoadScript(path string) *OrderedMap {
	path = normalizeResourcePath(path)
	content := p.getTreeContent(path)
	if content == nil {
		return NewOrderedMap()
	}
	switch suffixOf(path) {
	case "bin":
		return mapToOrdered(binParse(p, content))
	case "lst":
		return mapToOrdered(lstToIface(lstParse(p, content)))
	case "str":
		return mapToOrdered(strToIface(strParse(p, content)))
	case "ani":
		return aniParse(p, content)
	case "ui":
		return defaultParseDict(p, path, content, true)
	default:
		return defaultParseDict(p, path, content, false)
	}
}

// LoadScriptSource 返回还原后的脚本源码文本（对应 Java loadScriptSource）。
func (p *Pvf) LoadScriptSource(path string) string {
	path = normalizeResourcePath(path)
	content := p.getTreeContent(path)
	if content == nil {
		return ""
	}
	switch suffixOf(path) {
	case "str":
		return p.decodeText(content)
	case "ui":
		return defaultParseSource(p, path, content, true)
	case "bin", "lst", "ani":
		// 这些类型 Java 默认走 IParser.convertSource -> convert().toString()
		return p.LoadScript(path).String()
	default:
		return defaultParseSource(p, path, content, false)
	}
}

// =============================================================================
// bin 解析器（对应 BinParser）：偏移表 + PVF 文本编码切分
// =============================================================================

func binParse(p *Pvf, data []byte) map[string]interface{} {
	r := newReader(data)
	dict := make(map[string]interface{})

	tableSize := r.readInt()
	start := r.readInt()
	for i := int32(0); i < tableSize; i++ {
		end := r.readInt()
		ctx := data[start+4 : end+4]
		dict[fmt.Sprintf("%d", i)] = p.decodeText(ctx)
		start = end
	}
	return dict
}

// =============================================================================
// lst 解析器（对应 LstParser）：index -> stringTable 值(小写)
// =============================================================================

func lstParse(p *Pvf, data []byte) map[string]string {
	r := newReader(data)
	r.seek(2)
	dict := make(map[string]string)
	var index int32 = -1
	for r.hasRemaining() {
		if r.remaining()-5 < 0 {
			break
		}
		typ := r.readByte()
		switch typ {
		case 0x02:
			index = r.readInt()
		case 0x07:
			str := strings.ToLower(p.getStringTable(r.readInt()))
			dict[fmt.Sprintf("%d", index)] = str
		}
	}
	return dict
}

func lstToIface(m map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// =============================================================================
// str 解析器（对应 StrParser）：按 \r\n 分行，key>value
// =============================================================================

func strParse(p *Pvf, data []byte) map[string]string {
	content := p.decodeText(data)
	dict := make(map[string]string)
	for _, line := range strings.Split(content, "\r\n") {
		if strings.HasPrefix(line, "//") || strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, ">", 2)
		if len(parts) == 2 {
			dict[parts[0]] = parts[1]
		}
	}
	return dict
}

func strToIface(m map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// =============================================================================
// default / ui 解析器（对应 DefaultParser / UIParser + PvfData）
// =============================================================================

// pvfUnit 表示一个解析单元（unitType + value）。
type pvfUnit struct {
	unitType int
	value    interface{}
}

// parseUnits 把脚本字节流解析为单元序列。ui=true 时启用 UIParser 的特殊标签修正。
func parseUnits(p *Pvf, path string, data []byte, ui bool) []pvfUnit {
	r := newReader(data)
	r.seek(2)
	var units []pvfUnit

	for r.hasRemaining() {
		if r.remaining() < 5 {
			break
		}
		unitType := int(r.readByte())
		value := r.readBytes(4)

		switch unitType {
		case 1, 2, 3:
			units = append(units, pvfUnit{unitType, bytesToInt(value)})
		case 4:
			units = append(units, pvfUnit{unitType, bytesToFloat(value)})
		case 5:
			str := p.getStringTable(bytesToInt(value))
			ut := unitType
			if ui && isUITextTag(str) {
				ut = 7 // UIParser 把某些标签当字符串处理
			}
			units = append(units, pvfUnit{ut, str})
		case 6, 7, 8:
			units = append(units, pvfUnit{unitType, p.getStringTable(bytesToInt(value))})
		case 10:
			units = append(units, pvfUnit{unitType, p.getNString(path, p.getStringTable(bytesToInt(value)))})
		}
	}
	return units
}

func isUITextTag(s string) bool {
	return s == "[common action]" ||
		s == "[parent tab]" ||
		s == "[parent radio]" ||
		s == "[parent]" ||
		strings.Contains(s, "int option]")
}

func defaultParseDict(p *Pvf, path string, data []byte, ui bool) *OrderedMap {
	units := parseUnits(p, path, data, ui)
	return loadDict(units)
}

func defaultParseSource(p *Pvf, path string, data []byte, ui bool) string {
	units := parseUnits(p, path, data, ui)
	return buildSource(units)
}

// segmentKeysWithEndMark 收集所有带结束标记 [/xxx] 的段名（去掉 /）。
func segmentKeysWithEndMark(units []pvfUnit) map[string]bool {
	set := make(map[string]bool)
	for _, u := range units {
		if s, ok := u.value.(string); ok {
			if strings.HasPrefix(s, "[/") && strings.HasSuffix(s, "]") {
				set[strings.ReplaceAll(s, "/", "")] = true
			}
		}
	}
	return set
}

// loadDict 对应 PvfData.loadDict：把单元序列聚合成嵌套有序字典。
func loadDict(units []pvfUnit) *OrderedMap {
	endMarks := segmentKeysWithEndMark(units)
	return loadDictInner(units, endMarks)
}

func loadDictInner(units []pvfUnit, endMarks map[string]bool) *OrderedMap {
	res := NewOrderedMap()
	var segment []pvfUnit
	segmentKey := ""
	hasKey := false

	addSeg := func(key string, seg []pvfUnit) {
		oldKey := key
		if res.Has(key) {
			suffix := 1
			for res.Has(fmt.Sprintf("%s-%d", key, suffix)) {
				suffix++
			}
			key = fmt.Sprintf("%s-%d", key, suffix)
		}
		segHasType5 := false
		for _, s := range seg {
			if s.unitType == 5 {
				segHasType5 = true
				break
			}
		}
		if (endMarks[key] || endMarks[oldKey]) && segHasType5 {
			res.Set(key, loadDictInner(seg, endMarks))
		} else {
			vals := make([]interface{}, len(seg))
			for i, s := range seg {
				vals[i] = s.value
			}
			res.Set(key, vals)
		}
	}

	for _, u := range units {
		if u.unitType == 5 {
			str := u.value.(string)
			if !hasKey {
				if strings.Contains(str, "/") {
					segmentKey = ""
					hasKey = false
				} else {
					segmentKey = str
					hasKey = true
				}
				continue
			}
			// 已有 segmentKey
			if !endMarks[segmentKey] || strings.ReplaceAll(str, "/", "") == segmentKey {
				addSeg(segmentKey, segment)
				if strings.Contains(str, "/") {
					segmentKey = ""
					hasKey = false
				} else {
					segmentKey = str
					hasKey = true
				}
				segment = nil
				continue
			}
		}
		segment = append(segment, u)
	}

	if hasKey {
		addSeg(segmentKey, segment)
	}
	return res
}

// buildSource 对应 PvfData.getSource：按缩进还原脚本源码文本。
func buildSource(units []pvfUnit) string {
	endMarks := segmentKeysWithEndMark(units)
	var lineValues []string
	var sb strings.Builder
	indent := 0

	flush := func() {
		if len(lineValues) == 0 {
			return
		}
		sb.WriteString(strings.Repeat("    ", indent))
		sb.WriteString(strings.Join(lineValues, "\t"))
		sb.WriteString("\n")
		lineValues = lineValues[:0]
	}

	for _, u := range units {
		if u.unitType == 5 {
			if s, ok := u.value.(string); ok && strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
				flush()
				if strings.HasPrefix(s, "[/") {
					if indent > 0 {
						indent--
					}
				}
				sb.WriteString(strings.Repeat("    ", indent))
				sb.WriteString(s)
				sb.WriteString("\n")
				if !strings.HasPrefix(s, "[/") && endMarks[s] {
					indent++
				}
				continue
			}
		}
		lineValues = append(lineValues, renderValue(u.value))
	}
	flush()
	return strings.TrimRight(sb.String(), " \t\r\n")
}

func renderValue(value interface{}) string {
	if value == nil {
		return "null"
	}
	switch v := value.(type) {
	case []int32:
		parts := make([]string, len(v))
		for i, x := range v {
			parts[i] = fmt.Sprintf("%d", x)
		}
		return strings.Join(parts, "\t")
	case []int:
		parts := make([]string, len(v))
		for i, x := range v {
			parts[i] = fmt.Sprintf("%d", x)
		}
		return strings.Join(parts, "\t")
	case []byte:
		parts := make([]string, len(v))
		for i, x := range v {
			parts[i] = fmt.Sprintf("%d", int8(x))
		}
		return strings.Join(parts, "\t")
	case []interface{}:
		parts := make([]string, len(v))
		for i, x := range v {
			parts[i] = renderValue(x)
		}
		return strings.Join(parts, "\t")
	case float32:
		return fmt.Sprintf("%v", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// =============================================================================
// ani 解析器（对应 AniParser）
// =============================================================================

const aniSpectrumPayloadLen = 15

func aniParse(p *Pvf, data []byte) (dict *OrderedMap) {
	dict = NewOrderedMap()
	if len(data) == 0 {
		dict.Set("[PARSE ERROR]", "empty ani data")
		return dict
	}

	r := newReader(data)
	var frames []interface{}
	defer func() {
		if v := recover(); v != nil {
			if frames != nil && !dict.Has("[FRAME]") {
				dict.Set("[FRAME]", frames)
			}
			dict.Set("[PARSE ERROR]", fmt.Sprintf("%v at offset %d of %d", v, r.pos, len(data)))
		}
	}()

	frameCount := int(r.readShort())
	dict.Set("[FRAME MAX]", int32(frameCount))

	imgPathSize := int(r.readShort())
	if imgPathSize < 0 {
		dict.Set("[PARSE ERROR]", fmt.Sprintf("invalid image path count: %d", imgPathSize))
		return dict
	}
	imgPathList := make([]string, imgPathSize)
	for i := 0; i < imgPathSize; i++ {
		l := int(r.readInt())
		if l < 0 {
			dict.Set("[PARSE ERROR]", fmt.Sprintf("invalid image path length: %d", l))
			return dict
		}
		imgPathList[i] = string(r.readBytes(l))
	}

	globalParamCount := int(r.readShort())
	for i := 0; i < globalParamCount; i++ {
		key, val := aniParam(r, int(r.readShort()))
		dict.Set(key, val)
	}

	frames = make([]interface{}, 0, frameCount)
	for i := 0; i < frameCount; i++ {
		fd := NewOrderedMap()

		extendParamCount := int(r.readShort())
		for j := 0; j < extendParamCount; j++ {
			key, val := aniParam(r, int(r.readShort()))
			fd.Set(key, val)
		}

		// [IMAGE]
		img := ""
		mark := r.pos
		imgPathIndex := r.readShort()
		if imgPathIndex == -1 {
			r.seek(mark)
		} else if int(imgPathIndex) >= 0 && int(imgPathIndex) < len(imgPathList) {
			img = imgPathList[imgPathIndex]
		} else {
			img = fmt.Sprintf("[INVALID IMAGE INDEX %d]", imgPathIndex)
		}
		fd.Set("[IMAGE]", []interface{}{img, int32(r.readShort())})

		// [IMAGE POS]
		fd.Set("[IMAGE POS]", []int32{r.readInt(), r.readInt()})

		paramCount := int(r.readShort())
		for j := 0; j < paramCount; j++ {
			key, val := aniParam(r, int(r.readShort()))
			fd.Set(key, val)
		}

		frames = append(frames, fd)
	}
	dict.Set("[FRAME]", frames)
	return dict
}

// aniParam 对应 AniParser.Param 枚举的各参数读取。
func aniParam(r *reader, index int) (string, interface{}) {
	switch index {
	case 0x00:
		return "[LOOP]", int32(int8(r.readByte()))
	case 0x01:
		return "[SHADOW]", int32(int8(r.readByte()))
	case 0x03:
		return "[COORD]", int32(int8(r.readByte()))
	case 0x07:
		return "[IMAGE RATE]", []int32{r.readInt(), r.readInt()}
	case 0x08:
		return "[IMAGE ROTATE]", r.readFloat()
	case 0x09:
		return "[RGBA]", []byte{r.readByte(), r.readByte(), r.readByte(), r.readByte()}
	case 0x0A:
		return "[INTERPOLATION]", int32(int8(r.readByte()))
	case 0x0B:
		effect := r.readShort()
		if effect == 5 {
			return "[GRAPHIC EFFECT]", []int32{int32(effect), int32(r.readShort()), int32(r.readShort()), int32(r.readShort())}
		}
		return "[GRAPHIC EFFECT]", effect
	case 0x0C:
		return "[DELAY]", r.readInt()
	case 0x0D:
		return "[DAMAGE TYPE]", r.readShort()
	case 0x0E:
		return "[DAMAGE BOX]", []int32{r.readInt(), r.readInt(), r.readInt(), r.readInt(), r.readInt(), r.readInt()}
	case 0x0F:
		return "[ATTACK BOX]", []int32{r.readInt(), r.readInt(), r.readInt(), r.readInt(), r.readInt(), r.readInt()}
	case 0x10:
		l := int(r.readInt())
		return "[PLAY SOUND]", string(r.readBytes(l))
	case 0x12:
		return "[SPECTRUM]", r.readBytes(aniSpectrumPayloadLen)
	case 0x17:
		return "[SET FLAG]", r.readInt()
	case 0x18:
		return "[FLIP TYPE]", r.readShort()
	case 0x19:
		return "[LOOP START]", int32(1)
	case 0x1A:
		return "[LOOP END]", r.readShort()
	case 0x1B:
		return "[CLIP]", []int32{int32(r.readShort()), int32(r.readShort()), int32(r.readShort()), int32(r.readShort())}
	default:
		return "[UNDEFINED]", r.readShort()
	}
}

// =============================================================================
// OrderedMap：等价 hutool JSONObject(true) 的有序 map，保证输出顺序与 Java 一致。
// =============================================================================

// OrderedMap 是保持插入顺序的字符串键映射。
type OrderedMap struct {
	keys   []string
	values map[string]interface{}
}

func NewOrderedMap() *OrderedMap {
	return &OrderedMap{values: make(map[string]interface{})}
}

func (m *OrderedMap) Has(key string) bool {
	_, ok := m.values[key]
	return ok
}

func (m *OrderedMap) Set(key string, value interface{}) {
	if _, ok := m.values[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

func (m *OrderedMap) Get(key string) (interface{}, bool) {
	v, ok := m.values[key]
	return v, ok
}

func (m *OrderedMap) Keys() []string { return m.keys }

func mapToOrdered(m map[string]interface{}) *OrderedMap {
	om := NewOrderedMap()
	// 普通 map 无序，这里对 key 排序以保证稳定输出
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		om.Set(k, m[k])
	}
	return om
}

// String 输出类 JSON 文本（顺序保持），便于调试与与上层对接。
func (m *OrderedMap) String() string {
	var sb strings.Builder
	writeJSONValue(&sb, m)
	return sb.String()
}

func (m *OrderedMap) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}
	return []byte(m.String()), nil
}

func writeJSONValue(sb *strings.Builder, v interface{}) {
	switch val := v.(type) {
	case *OrderedMap:
		sb.WriteByte('{')
		for i, k := range val.keys {
			if i > 0 {
				sb.WriteByte(',')
			}
			writeJSONString(sb, k)
			sb.WriteByte(':')
			writeJSONValue(sb, val.values[k])
		}
		sb.WriteByte('}')
	case []interface{}:
		sb.WriteByte('[')
		for i, e := range val {
			if i > 0 {
				sb.WriteByte(',')
			}
			writeJSONValue(sb, e)
		}
		sb.WriteByte(']')
	case []int32:
		sb.WriteByte('[')
		for i, e := range val {
			if i > 0 {
				sb.WriteByte(',')
			}
			fmt.Fprintf(sb, "%d", e)
		}
		sb.WriteByte(']')
	case []int:
		sb.WriteByte('[')
		for i, e := range val {
			if i > 0 {
				sb.WriteByte(',')
			}
			fmt.Fprintf(sb, "%d", e)
		}
		sb.WriteByte(']')
	case []byte:
		sb.WriteByte('[')
		for i, e := range val {
			if i > 0 {
				sb.WriteByte(',')
			}
			fmt.Fprintf(sb, "%d", int8(e))
		}
		sb.WriteByte(']')
	case string:
		writeJSONString(sb, val)
	case nil:
		sb.WriteString("null")
	case float32:
		fmt.Fprintf(sb, "%v", val)
	default:
		fmt.Fprintf(sb, "%v", val)
	}
}

func writeJSONString(sb *strings.Builder, s string) {
	b, err := json.Marshal(s)
	if err != nil {
		sb.WriteString("\"\"")
		return
	}
	sb.Write(b)
}

// =============================================================================
// NPK 解析
// =============================================================================

const (
	npkMagicNumber = "NeoplePack_Bill"
	imageMagic1    = "Neople Image File"
	imageMagic2    = "Neople Img File"
	npkMagicLen    = 16
	npkImgNameLen  = 256
)

// npkDecryptKey 是 NPK 文件名固定异或密钥（256 字节）。
var npkDecryptKey = func() []byte {
	s := "puchikon@neople dungeon and fighter " +
		strings.Repeat("DNFDNFDNFDNFDNFDNFDNFDNFDNFDNF", 7) +
		"DNFDNFDNF\x00"
	return []byte(s)
}()

func decryptNpkName(data []byte) string {
	dec := make([]byte, npkImgNameLen)
	for i := 0; i < len(data) && i < npkImgNameLen; i++ {
		dec[i] = data[i] ^ npkDecryptKey[i]
	}
	return normalizeResourcePath(strings.ReplaceAll(string(dec), "\x00", ""))
}

// 颜色位
type colorBit int

const (
	colorUnknown  colorBit = 0x00
	colorARGB1555 colorBit = 0x0e
	colorARGB4444 colorBit = 0x0f
	colorARGB8888 colorBit = 0x10
	colorDXT1     colorBit = 0x12
	colorDXT3     colorBit = 0x13
	colorDXT5     colorBit = 0x14
)

func colorBitOf(v int32) colorBit {
	switch colorBit(v) {
	case colorARGB1555, colorARGB4444, colorARGB8888, colorDXT1, colorDXT3, colorDXT5:
		return colorBit(v)
	}
	return colorUnknown
}

// 压缩模式
type compressMode int

const (
	compressUnknown compressMode = 0x01
	compressNone    compressMode = 0x05
	compressZlib    compressMode = 0x06
	compressDDSZlib compressMode = 0x07
)

func compressModeOf(v int32) compressMode {
	switch compressMode(v) {
	case compressNone, compressZlib, compressDDSZlib:
		return compressMode(v)
	}
	return compressUnknown
}

// NpkImgTable 对应 Java NpkImgTable。
type NpkImgTable struct {
	Offset int32
	Length int32
	Name   string
}

// NpkTexture 对应 Java NpkTexture。
type NpkTexture struct {
	img          *NpkImg
	Index        int
	IsLink       bool
	linkTarget   *NpkTexture
	ColorBit     colorBit
	CompressMode compressMode
	width        int32
	height       int32
	Length       int32
	x            int32
	y            int32
	FrameWidth   int32
	FrameHeight  int32
	Data         []byte
}

func (t *NpkTexture) Width() int32 {
	if t.IsLink && t.linkTarget != nil {
		return t.linkTarget.Width()
	}
	return t.width
}

func (t *NpkTexture) Height() int32 {
	if t.IsLink && t.linkTarget != nil {
		return t.linkTarget.Height()
	}
	return t.height
}

func (t *NpkTexture) X() int32 {
	if t.IsLink && t.linkTarget != nil {
		return t.linkTarget.X()
	}
	return t.x
}

func (t *NpkTexture) Y() int32 {
	if t.IsLink && t.linkTarget != nil {
		return t.linkTarget.Y()
	}
	return t.y
}

// BgraData 返回 BGRA 像素数组（对应 getBgraData）。
func (t *NpkTexture) BgraData() ([]byte, error) {
	if t.IsLink {
		if t.linkTarget == nil {
			return nil, errors.New("link target 为空")
		}
		return t.linkTarget.BgraData()
	}
	return convertTextureData(t)
}

// ToPngBytes 把贴图转为 PNG 字节流（对应 toPngBytes）。
func (t *NpkTexture) ToPngBytes() ([]byte, error) {
	w := int(t.Width())
	h := int(t.Height())
	bgra, err := t.BgraData()
	if err != nil {
		return nil, err
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	offset := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			b := bgra[offset]
			g := bgra[offset+1]
			rr := bgra[offset+2]
			a := bgra[offset+3]
			i := img.PixOffset(x, y)
			img.Pix[i] = rr
			img.Pix[i+1] = g
			img.Pix[i+2] = b
			img.Pix[i+3] = a
			offset += 4
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// NpkImg 对应 Java NpkImg。
type NpkImg struct {
	MagicNumber string
	IndexSize   int32
	Reserve     int32
	Version     int32
	IndexCount  int32
	Textures    []*NpkTexture
}

// Npk 是 NPK 解析入口（对应 NpkCoder 的静态状态，此处实例化）。
type Npk struct {
	rootPath  string
	nameTable map[string]string      // imgName -> 所属 npk 文件名
	indexTab  map[string]NpkImgTable // imgName -> 索引项
	mu        sync.Mutex
}

// OpenNpk 初始化 ImagePacks2 目录（对应 NpkCoder.initialize）。
func OpenNpk(rootPath string) (*Npk, error) {
	n := &Npk{
		rootPath:  rootPath,
		nameTable: make(map[string]string),
		indexTab:  make(map[string]NpkImgTable),
	}
	files, err := loopNpkFiles(rootPath)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if err := n.readNpkCache(f); err != nil {
			return nil, fmt.Errorf("读取 %s 失败: %w", f, err)
		}
	}
	return n, nil
}

func loopNpkFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.EqualFold(filepath.Ext(path), ".npk") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func (n *Npk) readNpkCache(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return err
	}

	tables, err := readImgTables(f, stat.Size())
	if err != nil {
		return err
	}
	base := filepath.Base(path)
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, t := range tables {
		t.Name = normalizeResourcePath(t.Name)
		n.registerImgTable(t.Name, base, t)
		if trimmed, ok := strings.CutPrefix(t.Name, "sprite/"); ok {
			n.registerImgTable(trimmed, base, t)
		}
	}
	return nil
}

func (n *Npk) registerImgTable(name string, npkFile string, table NpkImgTable) {
	name = normalizeResourcePath(name)
	if name == "" {
		return
	}
	if _, ok := n.nameTable[name]; ok {
		return
	}
	n.nameTable[name] = npkFile
	n.indexTab[name] = table
}

func readImgTables(r io.Reader, fileSize int64) ([]NpkImgTable, error) {
	sr := newStreamReader(r)
	magic, err := sr.readN(npkMagicLen)
	if err != nil {
		return nil, err
	}
	magicText := strings.TrimRight(string(magic), "\x00")
	if magicText != npkMagicNumber {
		return nil, nil
	}
	// 注：原 Java 在魔数相等时反而打印 error（疑似笔误），此处不做强校验，保持兼容。

	imgSize, err := sr.readInt()
	if err != nil {
		return nil, err
	}
	const imgTableEntryLen = 4 + 4 + npkImgNameLen
	maxImgSize := (fileSize - npkMagicLen - 4) / imgTableEntryLen
	if imgSize < 0 || int64(imgSize) > maxImgSize {
		return nil, fmt.Errorf("invalid npk img table size: %d (max %d)", imgSize, maxImgSize)
	}
	tables := make([]NpkImgTable, 0, imgSize)
	for i := int32(0); i < imgSize; i++ {
		offset, err := sr.readInt()
		if err != nil {
			return nil, err
		}
		length, err := sr.readInt()
		if err != nil {
			return nil, err
		}
		nameBytes, err := sr.readN(npkImgNameLen)
		if err != nil {
			return nil, err
		}
		tables = append(tables, NpkImgTable{
			Offset: offset,
			Length: length,
			Name:   decryptNpkName(nameBytes),
		})
	}
	return tables, nil
}

// LoadImg 加载一个 IMG（对应 NpkCoder.loadImg）。
func (n *Npk) LoadImg(name string) (*NpkImg, error) {
	name = normalizeResourcePath(name)
	npkFile, ok := n.nameTable[name]
	if !ok {
		return nil, fmt.Errorf("未找到 img: %s", name)
	}
	table := n.indexTab[name]
	f, err := os.Open(filepath.Join(n.rootPath, npkFile))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readImg(f, table)
}

// ImgNames 返回所有可用的 IMG 名（便于遍历导出）。
func (n *Npk) ImgNames() []string {
	names := make([]string, 0, len(n.nameTable))
	for k := range n.nameTable {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func readImg(rs io.ReadSeeker, table NpkImgTable) (*NpkImg, error) {
	if _, err := rs.Seek(int64(table.Offset), io.SeekStart); err != nil {
		return nil, err
	}
	sr := newStreamReader(rs)
	magic, err := sr.readCStr()
	if err != nil {
		return nil, err
	}

	img := &NpkImg{MagicNumber: magic}

	switch {
	case strings.HasPrefix(magic, imageMagic1):
		img.IndexSize, _ = sr.readInt()
		sr.readN(2)
		img.Version, _ = sr.readInt()
		img.IndexCount, _ = sr.readInt()
	case strings.HasPrefix(magic, imageMagic2):
		img.IndexSize, _ = sr.readInt()
		img.Reserve, _ = sr.readInt()
		img.Version, _ = sr.readInt()
		img.IndexCount, _ = sr.readInt()
	default:
		// 音频等其它文件，未解析
		img.Version = 0x00
		return img, nil
	}

	switch img.Version {
	case 0x01:
		if err := readImgV1(sr, img); err != nil {
			return nil, err
		}
	case 0x02:
		if err := readImgV2(sr, img); err != nil {
			return nil, err
		}
	default:
		return img, fmt.Errorf("不支持的 img 版本: %d", img.Version)
	}
	return img, nil
}

func readImgV1(sr *streamReader, img *NpkImg) error {
	count := int(img.IndexCount)
	textures := make([]*NpkTexture, count)
	img.Textures = textures
	targetMap := make(map[int]int)

	for i := 0; i < count; i++ {
		tex := &NpkTexture{img: img, Index: i}
		textures[i] = tex

		indexType, err := sr.readInt()
		if err != nil {
			return err
		}
		if indexType == 0x11 {
			tex.IsLink = true
			tgt, _ := sr.readInt()
			targetMap[i] = int(tgt)
			continue
		}
		tex.ColorBit = colorBitOf(indexType)
		cm, _ := sr.readInt()
		tex.CompressMode = compressModeOf(cm)
		w, _ := sr.readInt()
		tex.width = w
		h, _ := sr.readInt()
		tex.height = h
		l, _ := sr.readInt()
		tex.Length = l
		x, _ := sr.readInt()
		tex.x = x
		y, _ := sr.readInt()
		tex.y = y
		fw, _ := sr.readInt()
		tex.FrameWidth = fw
		fh, _ := sr.readInt()
		tex.FrameHeight = fh

		if tex.CompressMode == compressNone {
			tex.Length = tex.width * tex.height * bytesPerPixel(tex.ColorBit)
		}
		data, err := sr.readN(int(tex.Length))
		if err != nil {
			return err
		}
		tex.Data = data
	}

	linkTextures(textures, targetMap)
	return nil
}

func readImgV2(sr *streamReader, img *NpkImg) error {
	count := int(img.IndexCount)
	textures := make([]*NpkTexture, count)
	img.Textures = textures
	targetMap := make(map[int]int)

	// 先读全部索引项
	for i := 0; i < count; i++ {
		tex := &NpkTexture{img: img, Index: i}
		textures[i] = tex

		indexType, err := sr.readInt()
		if err != nil {
			return err
		}
		if indexType == 0x11 {
			tex.IsLink = true
			tgt, _ := sr.readInt()
			targetMap[i] = int(tgt)
			continue
		}
		tex.ColorBit = colorBitOf(indexType)
		cm, _ := sr.readInt()
		tex.CompressMode = compressModeOf(cm)
		w, _ := sr.readInt()
		tex.width = w
		h, _ := sr.readInt()
		tex.height = h
		l, _ := sr.readInt()
		tex.Length = l
		x, _ := sr.readInt()
		tex.x = x
		y, _ := sr.readInt()
		tex.y = y
		fw, _ := sr.readInt()
		tex.FrameWidth = fw
		fh, _ := sr.readInt()
		tex.FrameHeight = fh
	}

	// 再统一读数据
	for _, tex := range textures {
		if tex.IsLink {
			continue
		}
		if tex.CompressMode == compressNone {
			tex.Length = tex.width * tex.height * bytesPerPixel(tex.ColorBit)
		}
		data, err := sr.readN(int(tex.Length))
		if err != nil {
			return err
		}
		tex.Data = data
	}

	linkTextures(textures, targetMap)
	return nil
}

func linkTextures(textures []*NpkTexture, targetMap map[int]int) {
	for _, tex := range textures {
		if !tex.IsLink {
			continue
		}
		if tgt, ok := targetMap[tex.Index]; ok && tgt >= 0 && tgt < len(textures) && tgt != tex.Index {
			tex.linkTarget = textures[tgt]
		}
	}
}

func bytesPerPixel(c colorBit) int32 {
	if c == colorARGB8888 {
		return 4
	}
	return 2
}

func convertTextureData(t *NpkTexture) ([]byte, error) {
	data := t.Data
	if t.CompressMode == compressZlib {
		dec, err := unZlib(data)
		if err != nil {
			return nil, err
		}
		data = dec
	}
	return readBgraBytes(data, int(t.width), int(t.height), t.ColorBit), nil
}

func unZlib(data []byte) ([]byte, error) {
	zr, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

// readBgraBytes 把任意 ARGB 数据转为 BGRA 字节数组（对应 ColorHelper）。
func readBgraBytes(data []byte, width, height int, c colorBit) []byte {
	content := make([]byte, width*height*4)
	r := newReader(data)
	for i := 0; i < len(content); i += 4 {
		readBgra(r, c, content, i)
	}
	return content
}

// readBgra 解码单个像素，逐位与 Java ColorHelper.readBgra 对齐。
func readBgra(r *reader, c colorBit, target []byte, offset int) {
	var a, rr, g, b byte

	switch c {
	case colorARGB8888:
		bs := r.readBytes(4)
		b = bs[0]
		g = bs[1]
		rr = bs[2]
		a = bs[3]
	case colorARGB1555:
		bs := r.readBytes(2)
		pixel := binary.LittleEndian.Uint16(bs)
		rr = byte((pixel & 0x7C00) >> 7)
		g = byte((pixel & 0x03E0) >> 2)
		b = byte((pixel & 0x001F) << 3)
		// Java: a = (byte)(bs[1] >> 7)，bs[1] 为有符号 byte 的算术右移
		a = byte(int8(bs[1]) >> 7)
	case colorARGB4444:
		bs := r.readBytes(2)
		a = bs[1] & 0xf0
		rr = (bs[1] & 0x0f) << 4
		g = bs[0] & 0xf0
		b = (bs[0] & 0x0f) << 4
	}

	if a == 0x00 {
		b, g, rr = 0, 0, 0
	}

	target[offset] = b
	target[offset+1] = g
	target[offset+2] = rr
	target[offset+3] = a
}
