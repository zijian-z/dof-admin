package dnfparser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

type verifyReport struct {
	Input              string                `json:"input"`
	InputSizeBytes     int64                 `json:"input_size_bytes"`
	StartedAt          string                `json:"started_at"`
	Duration           string                `json:"duration"`
	GUID               string                `json:"guid"`
	Version            int32                 `json:"version"`
	TreeLength         int32                 `json:"tree_length"`
	TreeCRC32          int32                 `json:"tree_crc32"`
	HeaderTreeCount    int32                 `json:"header_tree_count"`
	ParsedTreeFiles    int                   `json:"parsed_tree_files"`
	ContentBase        int                   `json:"content_base"`
	StringTableEntries int                   `json:"string_table_entries"`
	NStringFiles       int                   `json:"n_string_files"`
	SuffixCounts       map[string]int        `json:"suffix_counts"`
	BoundsErrors       []string              `json:"bounds_errors,omitempty"`
	Parse              parseSummary          `json:"parse"`
	Samples            []sampleSummary       `json:"samples"`
	GeneratedFiles     []string              `json:"generated_files"`
	Notes              []string              `json:"notes,omitempty"`
	SuffixParseStats   map[string]suffixStat `json:"suffix_parse_stats"`
}

type parseSummary struct {
	TotalFiles         int            `json:"total_files"`
	SuccessFiles       int            `json:"success_files"`
	EmptyOutputs       int            `json:"empty_outputs"`
	ParseErrorOutputs  int            `json:"parse_error_outputs"`
	PanicFiles         int            `json:"panic_files"`
	FailureListFile    string         `json:"failure_list_file,omitempty"`
	ParseErrorListFile string         `json:"parse_error_list_file,omitempty"`
	FirstFailures      []parseFailure `json:"first_failures,omitempty"`
	FirstParseErrors   []parseFailure `json:"first_parse_errors,omitempty"`
	TotalJSONBytes     int64          `json:"total_json_bytes"`
	MaxJSONBytes       int            `json:"max_json_bytes"`
	MaxJSONOutputPath  string         `json:"max_json_output_path,omitempty"`
}

type suffixStat struct {
	Total        int `json:"total"`
	Success      int `json:"success"`
	EmptyOutputs int `json:"empty_outputs"`
	ParseErrors  int `json:"parse_errors"`
	Panic        int `json:"panic"`
}

type parseFailure struct {
	Path   string `json:"path"`
	Suffix string `json:"suffix"`
	Error  string `json:"error"`
}

type sampleSummary struct {
	Label          string `json:"label"`
	Path           string `json:"path"`
	Suffix         string `json:"suffix"`
	JSONFile       string `json:"json_file"`
	SourceFile     string `json:"source_file"`
	JSONBytes      int    `json:"json_bytes"`
	SourceBytes    int    `json:"source_bytes"`
	TopLevelKeys   int    `json:"top_level_keys"`
	JSONLooksValid bool   `json:"json_looks_valid"`
}

func TestVerifyExtractedScriptPVF(t *testing.T) {
	start := time.Now()
	const input = "extracted/Script.pvf"
	const outDir = "dnfparser_verify_output"

	info, err := os.Stat(input)
	if os.IsNotExist(err) {
		t.Skipf("verification input %s is not present", input)
	}
	if err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := cleanVerifyOutput(outDir); err != nil {
		t.Fatal(err)
	}

	pvf, err := OpenPvf(input)
	if err != nil {
		t.Fatal(err)
	}

	files := collectPvfFiles(pvf)
	report := verifyReport{
		Input:              input,
		InputSizeBytes:     info.Size(),
		StartedAt:          start.Format(time.RFC3339),
		GUID:               pvf.header.guid,
		Version:            pvf.header.version,
		TreeLength:         pvf.header.treeLength,
		TreeCRC32:          pvf.header.treeCRC32,
		HeaderTreeCount:    pvf.header.treeCount,
		ParsedTreeFiles:    len(files),
		ContentBase:        pvf.contentBase,
		StringTableEntries: len(pvf.stringTable),
		NStringFiles:       len(pvf.nString),
		SuffixCounts:       countSuffixes(files),
		SuffixParseStats:   make(map[string]suffixStat),
	}

	for _, f := range files {
		end := int64(pvf.contentBase) + int64(f.Offset) + int64(f.Length)
		if f.Length < 0 || f.Offset < 0 || end > pvf.fileSize {
			report.BoundsErrors = append(report.BoundsErrors, fmt.Sprintf("%s offset=%d length=%d end=%d data=%d", f.Path, f.Offset, f.Length, end, pvf.fileSize))
		}
	}

	failures, parseErrors, failureFile, parseErrorFile := parseAllScripts(pvf, files, outDir, &report)
	if failureFile != "" {
		report.Parse.FailureListFile = failureFile
		report.GeneratedFiles = append(report.GeneratedFiles, failureFile)
	}
	if parseErrorFile != "" {
		report.Parse.ParseErrorListFile = parseErrorFile
		report.GeneratedFiles = append(report.GeneratedFiles, parseErrorFile)
	}
	report.Parse.PanicFiles = len(failures)
	if len(failures) > 0 {
		limit := len(failures)
		if limit > 20 {
			limit = 20
		}
		report.Parse.FirstFailures = append(report.Parse.FirstFailures, failures[:limit]...)
	}
	report.Parse.ParseErrorOutputs = len(parseErrors)
	if len(parseErrors) > 0 {
		limit := len(parseErrors)
		if limit > 20 {
			limit = 20
		}
		report.Parse.FirstParseErrors = append(report.Parse.FirstParseErrors, parseErrors[:limit]...)
	}

	samples := selectSamplePaths(files)
	for i, sample := range samples {
		summary, generated, err := writeSampleOutput(pvf, outDir, i+1, sample.label, sample.path)
		if err != nil {
			failures = append(failures, parseFailure{Path: sample.path, Suffix: suffixKey(sample.path), Error: err.Error()})
			continue
		}
		report.Samples = append(report.Samples, summary)
		report.GeneratedFiles = append(report.GeneratedFiles, generated...)
	}

	if int32(len(files)) != pvf.header.treeCount {
		report.Notes = append(report.Notes, fmt.Sprintf("parsed tree file count %d differs from header tree count %d", len(files), pvf.header.treeCount))
	}
	if !pvf.IsExist("stringtable.bin") {
		report.Notes = append(report.Notes, "stringtable.bin not found by IsExist")
	}
	if !pvf.IsExist("n_string.lst") {
		report.Notes = append(report.Notes, "n_string.lst not found by IsExist")
	}

	report.Duration = time.Since(start).String()
	reportFile := filepath.Join(outDir, "dnfparser_verify_report.json")
	if err := writeJSON(reportFile, report); err != nil {
		t.Fatal(err)
	}
	report.GeneratedFiles = append(report.GeneratedFiles, reportFile)
	if err := writeJSON(reportFile, report); err != nil {
		t.Fatal(err)
	}

	t.Logf("wrote verification report: %s", reportFile)
	t.Logf("files=%d stringTable=%d nString=%d parseSuccess=%d parsePanics=%d",
		len(files), len(pvf.stringTable), len(pvf.nString), report.Parse.SuccessFiles, report.Parse.PanicFiles)

	if len(report.BoundsErrors) > 0 || report.Parse.PanicFiles > 0 {
		t.Fatalf("PVF verification failed: bounds_errors=%d parse_panics=%d; see %s", len(report.BoundsErrors), report.Parse.PanicFiles, reportFile)
	}
}

type labeledPath struct {
	label string
	path  string
}

func collectPvfFiles(pvf *Pvf) []*PvfFile {
	files := make([]*PvfFile, 0, int(pvf.header.treeCount))
	for _, group := range pvf.treeDict {
		files = append(files, group...)
	}
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Path) < strings.ToLower(files[j].Path)
	})
	return files
}

func cleanVerifyOutput(outDir string) error {
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "dnfparser_") ||
			(strings.HasPrefix(name, "0") && (strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".source.txt"))) {
			if err := os.Remove(filepath.Join(outDir, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func countSuffixes(files []*PvfFile) map[string]int {
	counts := make(map[string]int)
	for _, f := range files {
		counts[suffixKey(f.Path)]++
	}
	return counts
}

func parseAllScripts(pvf *Pvf, files []*PvfFile, outDir string, report *verifyReport) ([]parseFailure, []parseFailure, string, string) {
	var failures []parseFailure
	var parseErrors []parseFailure
	var failureLines []string
	var parseErrorLines []string

	for _, f := range files {
		path := strings.ToLower(f.Path)
		sfx := suffixKey(path)
		stat := report.SuffixParseStats[sfx]
		stat.Total++
		report.Parse.TotalFiles++

		keys, jsonBytes, parseErrorText, panicText := safeScriptJSON(pvf, path)
		if panicText != "" {
			stat.Panic++
			fail := parseFailure{Path: path, Suffix: sfx, Error: panicText}
			failures = append(failures, fail)
			failureLines = append(failureLines, fmt.Sprintf("%s\t%s\t%s", fail.Suffix, fail.Path, fail.Error))
		} else {
			stat.Success++
			report.Parse.SuccessFiles++
			report.Parse.TotalJSONBytes += int64(jsonBytes)
			if jsonBytes > report.Parse.MaxJSONBytes {
				report.Parse.MaxJSONBytes = jsonBytes
				report.Parse.MaxJSONOutputPath = path
			}
			if keys == 0 {
				stat.EmptyOutputs++
				report.Parse.EmptyOutputs++
			}
			if parseErrorText != "" {
				stat.ParseErrors++
				parseErr := parseFailure{Path: path, Suffix: sfx, Error: parseErrorText}
				parseErrors = append(parseErrors, parseErr)
				parseErrorLines = append(parseErrorLines, fmt.Sprintf("%s\t%s\t%s", parseErr.Suffix, parseErr.Path, parseErr.Error))
			}
		}
		report.SuffixParseStats[sfx] = stat
	}

	parseErrorFile := ""
	if len(parseErrorLines) > 0 {
		parseErrorFile = filepath.Join(outDir, "dnfparser_parse_errors.tsv")
		_ = os.WriteFile(parseErrorFile, []byte(strings.Join(parseErrorLines, "\n")+"\n"), 0o644)
	}

	if len(failureLines) == 0 {
		return failures, parseErrors, "", parseErrorFile
	}
	failureFile := filepath.Join(outDir, "dnfparser_parse_failures.tsv")
	_ = os.WriteFile(failureFile, []byte(strings.Join(failureLines, "\n")+"\n"), 0o644)
	return failures, parseErrors, failureFile, parseErrorFile
}

func safeScriptJSON(pvf *Pvf, path string) (keys int, jsonBytes int, parseErrorText string, panicText string) {
	defer func() {
		if v := recover(); v != nil {
			panicText = fmt.Sprint(v)
		}
	}()

	om := pvf.LoadScript(path)
	if om == nil {
		return 0, 0, "", ""
	}
	keys = len(om.Keys())
	jsonBytes = len(om.String())
	if val, ok := om.Get("[PARSE ERROR]"); ok {
		parseErrorText = fmt.Sprint(val)
	}
	return keys, jsonBytes, parseErrorText, ""
}

func safeScriptOutput(pvf *Pvf, path string) (jsonText string, sourceText string, keys int, panicText string) {
	defer func() {
		if v := recover(); v != nil {
			panicText = fmt.Sprint(v)
		}
	}()

	om := pvf.LoadScript(path)
	if om != nil {
		keys = len(om.Keys())
		jsonText = om.String()
	}
	sourceText = pvf.LoadScriptSource(path)
	return jsonText, sourceText, keys, ""
}

func selectSamplePaths(files []*PvfFile) []labeledPath {
	var samples []labeledPath
	seen := make(map[string]bool)
	add := func(label, path string) {
		path = strings.ToLower(path)
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		samples = append(samples, labeledPath{label: label, path: path})
	}

	add("required-bin-stringtable", "stringtable.bin")
	add("required-lst-n-string", "n_string.lst")

	priorities := []struct {
		label    string
		suffixes []string
	}{
		{"str-table", []string{"str"}},
		{"ani-animation", []string{"ani"}},
		{"ui-layout", []string{"ui"}},
		{"lst-index", []string{"lst"}},
		{"default-script", []string{"stk", "equ", "etc", "dgn", "chr", "skl", "co", "cre", "exp", "map"}},
	}

	for _, item := range priorities {
		for _, suffix := range item.suffixes {
			if path := firstBySuffix(files, suffix, seen); path != "" {
				add(item.label+"-"+suffix, path)
				break
			}
		}
	}

	return samples
}

func firstBySuffix(files []*PvfFile, suffix string, seen map[string]bool) string {
	for _, f := range files {
		path := strings.ToLower(f.Path)
		if seen[path] {
			continue
		}
		if suffixKey(path) == suffix {
			return path
		}
	}
	return ""
}

func writeSampleOutput(pvf *Pvf, outDir string, index int, label string, path string) (sampleSummary, []string, error) {
	jsonText, sourceText, keys, panicText := safeScriptOutput(pvf, path)
	if panicText != "" {
		return sampleSummary{}, nil, fmt.Errorf("panic while exporting sample: %s", panicText)
	}

	prefix := fmt.Sprintf("%02d_%s_%s", index, sanitizeName(label), sanitizeName(path))
	jsonFile := filepath.Join(outDir, prefix+".json")
	sourceFile := filepath.Join(outDir, prefix+".source.txt")
	if err := os.WriteFile(jsonFile, []byte(jsonText), 0o644); err != nil {
		return sampleSummary{}, nil, err
	}
	if err := os.WriteFile(sourceFile, []byte(sourceText), 0o644); err != nil {
		return sampleSummary{}, nil, err
	}

	return sampleSummary{
		Label:          label,
		Path:           path,
		Suffix:         suffixKey(path),
		JSONFile:       jsonFile,
		SourceFile:     sourceFile,
		JSONBytes:      len(jsonText),
		SourceBytes:    len(sourceText),
		TopLevelKeys:   keys,
		JSONLooksValid: json.Valid([]byte(jsonText)),
	}, []string{jsonFile, sourceFile}, nil
}

func writeJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func suffixKey(path string) string {
	sfx := strings.ToLower(suffixOf(path))
	if sfx == "" {
		return "(none)"
	}
	return sfx
}

func sanitizeName(s string) string {
	s = strings.ToLower(strings.ReplaceAll(s, "\\", "/"))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if len(out) > 96 {
		out = out[:96]
	}
	if out == "" {
		return "sample"
	}
	return out
}
