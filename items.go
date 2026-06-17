// items.go —— PVF/NPK 资源高层解析（装备 / 道具）。
//
// 对应原 Java 工程的:
//   - pvf/PvfManager.getEquipmentList / getStackableList
//   - entity/common/Item、entity/equipment/Equipment、entity/stackable/Stackable
//   - 以及 ItemType / EquipmentType / StackableType / AttachType / UsableJob 枚举
//
// 本文件只依赖同包内 dnfparser.go 暴露的 OpenPvf / Pvf.LoadScript / OrderedMap
// 与 OpenNpk / Npk.LoadImg（NPK 图标导出），不引入任何额外第三方依赖。
package dnfparser

import (
	"fmt"
	"strings"
)

// TradToSimp 用于把脚本中的繁体中文（台服 Big5）转换为简体。
// 原 Java 使用 opencc4j 的 ZhConverterUtil.toSimple；为保持零额外依赖，
// 这里默认原样返回（保留繁体）。调用方可在初始化时替换为自己的转换器，例如:
//
//	dnfparser.TradToSimp = func(s string) string { return myOpenCC(s) }
var TradToSimp = func(s string) string { return s }

// =============================================================================
// 物品类型枚举（对应 ItemType / EquipmentType / StackableType / AttachType / UsableJob）
// 统一用「标签 -> 中文描述」映射表表达，并提供 forXxx 兜底匹配逻辑。
// =============================================================================

// ItemType 物品大类。
const (
	ItemTypeEquipment = "equipment" // 装备
	ItemTypeStackable = "stackable" // 道具
	ItemTypeOther     = "other"     // 其他
)

// equipmentTypeDesc: [equipment type] 标签 -> 中文，bool 表示是否为时装(avatar)。
var equipmentTypeDesc = map[string]struct {
	desc   string
	avatar bool
}{
	"[weapon]":         {"武器", false},
	"[title name]":     {"称号", false},
	"[coat]":           {"上衣", false},
	"[shoulder]":       {"护肩", false},
	"[pants]":          {"裤子", false},
	"[shoes]":          {"鞋子", false},
	"[waist]":          {"腰带", false},
	"[amulet]":         {"项链", false},
	"[wrist]":          {"手镯", false},
	"[ring]":           {"戒指", false},
	"[creature]":       {"宠物", false},
	"[artifact red]":   {"宠物装备-红色", false},
	"[artifact green]": {"宠物装备-绿色", false},
	"[artifact blue]":  {"宠物装备-蓝色", false},
	"[support]":        {"辅助装备", false},
	"[magic stone]":    {"魔法石", false},
	"[weapon avatar]":  {"武器(装扮)", true},
	"[aurora avatar]":  {"光环(装扮)", true},
	"[hat avatar]":     {"帽子(装扮)", true},
	"[hair avatar]":    {"头发(装扮)", true},
	"[breast avatar]":  {"胸部(装扮)", true},
	"[face avatar]":    {"脸部(装扮)", true},
	"[waist avatar]":   {"腰部(装扮)", true},
	"[coat avatar]":    {"上衣(装扮)", true},
	"[pants avatar]":   {"裤子(装扮)", true},
	"[shoes avatar]":   {"鞋子(装扮)", true},
	"[skin avatar]":    {"皮肤(装扮)", true},
}

// equipmentTypeName 返回装备类型中文（找不到时回退到原始标签）。
func equipmentTypeName(tag string) (string, bool) {
	if v, ok := equipmentTypeDesc[tag]; ok {
		return v.desc, v.avatar
	}
	return tag, false
}

// attachTypeDesc: [attach type] 标签 -> 绑定/交易类型中文。
var attachTypeDesc = map[string]string{
	"[trade]":         "不可交易",
	"[trade limit]":   "有限制交易",
	"[free]":          "自由交易",
	"[sealing]":       "封装",
	"[account]":       "账号绑定",
	"[trade delete]":  "无法删除",
	"[sealing trade]": "封装且不可交易",
}

// attachTypeName 对应 AttachType.forType（含末尾多余 ] 修正，找不到回退原值）。
func attachTypeName(tag string) string {
	if strings.HasSuffix(tag, "]]") {
		tag = tag[:len(tag)-1]
	}
	if d, ok := attachTypeDesc[tag]; ok {
		return d
	}
	return "不可交易" // 与 Java 默认 trade 一致
}

// usableJobDesc: [usable job] 标签 -> 职业中文。
var usableJobDesc = map[string]string{
	"[all]":              "所有职业",
	"[swordman]":         "鬼剑士",
	"[fighter]":          "格斗家(女)",
	"[at fighter]":       "格斗家(男)",
	"[demonic swordman]": "体操王子",
	"[creator mage]":     "缔造者/魔法师",
	"[gunner]":           "神枪手(男)",
	"[at gunner]":        "神枪手(女)",
	"[mage]":             "魔法师(女)",
	"[at mage]":          "魔法师(男)",
	"[priest]":           "圣职者",
	"[thief]":            "盗贼",
	"[free]":             "自由职业",
}

func usableJobName(tag string) string {
	if d, ok := usableJobDesc[tag]; ok {
		return d
	}
	return tag
}

// stackableTypeName 对应 StackableType.forType（精确匹配 + 模糊兜底）。
func stackableTypeName(tag string) string {
	exact := map[string]string{
		"[waste]":               "消耗品",
		"[material]":            "材料",
		"[recipe]":              "设计图",
		"[material expert job]": "副职业",
		"[quest]":               "任务道具",
		"[booster]":             "礼盒",
		"[feed]":                "饲料",
		"[creature]":            "宠物",
		"[etc]":                 "杂物",
		"[throw]":               "投掷物",
		"[legacy]":              "罐子",
	}
	if d, ok := exact[tag]; ok {
		return d
	}
	switch {
	case strings.Contains(tag, "legacy"):
		return "罐子"
	case strings.Contains(tag, "expert_town_potion"), strings.Contains(tag, "contract"),
		strings.Contains(tag, "global effect"), strings.Contains(tag, "disguise"),
		strings.Contains(tag, "waste"), strings.Contains(tag, "expert town potion"),
		strings.Contains(tag, "unlimited"):
		return "消耗品"
	case strings.Contains(tag, "quest"):
		return "任务道具"
	case strings.Contains(tag, "avatar_emblem"), strings.Contains(tag, "recipe"),
		strings.Contains(tag, "avatar emblem"):
		return "材料"
	case strings.Contains(tag, "booster"):
		return "礼盒"
	default:
		return "杂物"
	}
}

// rarityName 稀有度名称（对应 Equipment.getRarityName）。
func rarityName(r int) string {
	switch r {
	case 0:
		return "普通"
	case 1:
		return "高级"
	case 2:
		return "稀有"
	case 3:
		return "神器"
	case 4:
		return "史诗"
	case 5:
		return "勇者"
	case 6:
		return "传说"
	case 7:
		return "神话"
	default:
		return "未知"
	}
}

// =============================================================================
// 物品模型（对应 Item / Equipment / Stackable）
// =============================================================================

// ItemIcon 物品图标（对应 ItemIcon）：NPK 图集路径 + 索引。
type ItemIcon struct {
	Path  string `json:"path"`
	Index int    `json:"index"`
}

// Item 物品通用字段（对应 entity.common.Item）。
type Item struct {
	ID           int       `json:"id"`
	Rarity       int       `json:"rarity"`
	RarityName   string    `json:"rarityName"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`       // equipment / stackable / other
	UsableJobs   []string  `json:"usableJobs"` // 职业中文
	AttachType   string    `json:"attachType"` // 绑定/交易类型中文
	MinimumLevel int       `json:"minimumLevel"`
	Description  string    `json:"description"`
	Explain      string    `json:"explain"`
	StackLimit   int       `json:"stackLimit"`
	Icon         *ItemIcon `json:"icon,omitempty"`
}

// Equipment 装备（对应 entity.equipment.Equipment，Item 内嵌）。
type Equipment struct {
	Item
	EquipmentType    string `json:"equipmentType"`    // 装备类型中文
	EquipmentTypeTag string `json:"equipmentTypeTag"` // 原始标签
	Avatar           bool   `json:"avatar"`           // 是否时装
	ItemGroup        string `json:"itemGroup"`        // 子类型中文

	PhysicalAttack  []int `json:"physicalAttack,omitempty"`
	MagicalAttack   []int `json:"magicalAttack,omitempty"`
	SeparateAttack  []int `json:"separateAttack,omitempty"`
	PhysicalDefense []int `json:"physicalDefense,omitempty"`
	MagicalDefense  []int `json:"magicalDefense,omitempty"`

	Strength     *int `json:"strength,omitempty"`     // 力量 [physical attack]
	Intelligence *int `json:"intelligence,omitempty"` // 智力 [magical attack]
	Vitality     *int `json:"vitality,omitempty"`     // 体力 [physical defense]
	Spirit       *int `json:"spirit,omitempty"`       // 精神 [magical defense]

	HpMax        *int `json:"hpMax,omitempty"`
	MpMax        *int `json:"mpMax,omitempty"`
	HpRegenSpeed *int `json:"hpRegenSpeed,omitempty"`
	MpRegenSpeed *int `json:"mpRegenSpeed,omitempty"`

	AttackSpeed         *int `json:"attackSpeed,omitempty"`
	CastSpeed           *int `json:"castSpeed,omitempty"`
	MoveSpeed           *int `json:"moveSpeed,omitempty"`
	PhysicalCriticalHit *int `json:"physicalCriticalHit,omitempty"`
	MagicalCriticalHit  *int `json:"magicalCriticalHit,omitempty"`

	HitRate        *int `json:"hitRate,omitempty"` // [stuck] 取相反数
	Dodge          *int `json:"dodge,omitempty"`
	HitRecovery    *int `json:"hitRecovery,omitempty"`
	JumpPower      *int `json:"jumpPower,omitempty"`
	InventoryLimit *int `json:"inventoryLimit,omitempty"`
	AntiEvil       *int `json:"antiEvil,omitempty"`

	FireElement            *int `json:"fireElement,omitempty"`
	WaterElement           *int `json:"waterElement,omitempty"`
	LightElement           *int `json:"lightElement,omitempty"`
	DarkElement            *int `json:"darkElement,omitempty"`
	FireAttack             *int `json:"fireAttack,omitempty"`
	WaterAttack            *int `json:"waterAttack,omitempty"`
	LightAttack            *int `json:"lightAttack,omitempty"`
	DarkAttack             *int `json:"darkAttack,omitempty"`
	AllElementalAttack     *int `json:"allElementalAttack,omitempty"`
	FireResistance         *int `json:"fireResistance,omitempty"`
	WaterResistance        *int `json:"waterResistance,omitempty"`
	LightResistance        *int `json:"lightResistance,omitempty"`
	DarkResistance         *int `json:"darkResistance,omitempty"`
	AllElementalResistance *int `json:"allElementalResistance,omitempty"`

	BlindResistance           *int `json:"blindResistance,omitempty"`
	LightningResistance       *int `json:"lightningResistance,omitempty"`
	BurnResistance            *int `json:"burnResistance,omitempty"`
	FreezeResistance          *int `json:"freezeResistance,omitempty"`
	HoldResistance            *int `json:"holdResistance,omitempty"`
	SleepResistance           *int `json:"sleepResistance,omitempty"`
	BleedingResistance        *int `json:"bleedingResistance,omitempty"`
	ConfuseResistance         *int `json:"confuseResistance,omitempty"`
	CurseResistance           *int `json:"curseResistance,omitempty"`
	StoneResistance           *int `json:"stoneResistance,omitempty"`
	AllActiveStatusResistance *int `json:"allActiveStatusResistance,omitempty"`

	Grade                 *int `json:"grade,omitempty"`
	Weight                *int `json:"weight,omitempty"`
	Durability            *int `json:"durability,omitempty"`
	Price                 *int `json:"price,omitempty"`
	RepairPrice           *int `json:"repairPrice,omitempty"`
	Value                 *int `json:"value,omitempty"`
	RoomListMoveSpeedRate *int `json:"roomListMoveSpeedRate,omitempty"`
}

// Stackable 道具（对应 entity.stackable.Stackable，Item 内嵌）。
type Stackable struct {
	Item
	StackableType string `json:"stackableType"` // 道具类型中文
}

// =============================================================================
// 取值辅助（在 LoadScript 返回的 OrderedMap 上操作）
// 默认解析器把每个标签的值存为 []interface{}，元素为 int32 / float32 / string。
// =============================================================================

func omArray(m *OrderedMap, key string) []interface{} {
	v, ok := m.Get(key)
	if !ok {
		return nil
	}
	if a, ok := v.([]interface{}); ok {
		return a
	}
	return nil
}

func toInt(v interface{}) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int32:
		return int(x), true
	case int64:
		return int(x), true
	case float32:
		return int(x), true
	case float64:
		return int(x), true
	}
	return 0, false
}

func toStr(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// firstInt 取标签数组第 0 个 int（不存在返回 0,false）。
func firstInt(m *OrderedMap, key string) (int, bool) {
	a := omArray(m, key)
	if len(a) == 0 {
		return 0, false
	}
	return toInt(a[0])
}

// intOrNil 对应 getIntOrNull：标签不存在/空时返回 nil。
func intOrNil(m *OrderedMap, key string) *int {
	if !m.Has(key) {
		return nil
	}
	a := omArray(m, key)
	if len(a) == 0 {
		return nil
	}
	if i, ok := toInt(a[0]); ok {
		return &i
	}
	return nil
}

// arrInt 对应 getArrayInt：取前 1~2 个 int（不存在/空返回 nil）。
func arrInt(m *OrderedMap, key string) []int {
	if !m.Has(key) {
		return nil
	}
	a := omArray(m, key)
	if len(a) == 0 {
		return nil
	}
	if len(a) == 1 {
		if v, ok := toInt(a[0]); ok {
			return []int{v}
		}
		return nil
	}
	v0, _ := toInt(a[0])
	v1, _ := toInt(a[1])
	return []int{v0, v1}
}

// firstStr 取标签数组第 0 个字符串（不存在返回 def）。
func firstStr(m *OrderedMap, key, def string) string {
	a := omArray(m, key)
	if len(a) == 0 {
		return def
	}
	return toStr(a[0])
}

// =============================================================================
// 解析：Item 通用字段（对应 Item.parseForScript）
// =============================================================================

func (it *Item) parseForScript(s *OrderedMap) {
	if v, ok := firstInt(s, "[rarity]"); ok {
		it.Rarity = v
	}
	it.RarityName = rarityName(it.Rarity)

	// 物品大类
	switch {
	case s.Has("[equipment type]"):
		it.Type = ItemTypeEquipment
	case s.Has("[stackable type]"):
		it.Type = ItemTypeStackable
	default:
		it.Type = ItemTypeOther
	}

	// 绑定/交易类型
	if s.Has("[attach type]") {
		it.AttachType = attachTypeName(firstStr(s, "[attach type]", "[trade]"))
	} else {
		it.AttachType = "不可交易"
	}

	if v, ok := firstInt(s, "[minimum level]"); ok {
		it.MinimumLevel = v
	}

	// 携带上限，默认 1
	if v, ok := firstInt(s, "[stack limit]"); ok {
		it.StackLimit = v
	} else {
		it.StackLimit = 1
	}

	// 名称 / 描述 / 说明（繁->简）
	it.Name = TradToSimp(firstStr(s, "[name]", "未命名"))
	if strings.TrimSpace(it.Name) == "" {
		it.Name = "未命名"
	}
	it.Description = TradToSimp(firstStr(s, "[flavor text]", ""))
	it.Explain = TradToSimp(firstStr(s, "[detail explain]",
		firstStr(s, "[basic explain]", "")))

	// 可用职业
	if jobs := omArray(s, "[usable job]"); len(jobs) > 0 {
		it.UsableJobs = make([]string, 0, len(jobs))
		for _, j := range jobs {
			it.UsableJobs = append(it.UsableJobs, usableJobName(toStr(j)))
		}
	} else {
		it.UsableJobs = []string{"所有职业"}
	}

	// 图标
	if icon := omArray(s, "[icon]"); len(icon) > 0 {
		ic := &ItemIcon{Path: strings.ToLower(toStr(icon[0]))}
		if len(icon) >= 2 {
			if idx, ok := toInt(icon[1]); ok {
				ic.Index = idx
			}
		}
		it.Icon = ic
	}
}

// =============================================================================
// 解析：Equipment（对应 Equipment.forScript）
// =============================================================================

// EquipmentForScript 把单个装备脚本解析为 Equipment。
func EquipmentForScript(s *OrderedMap) *Equipment {
	e := &Equipment{}
	e.parseForScript(s)

	tag := firstStr(s, "[equipment type]", "[weapon]")
	e.EquipmentTypeTag = tag
	e.EquipmentType, e.Avatar = equipmentTypeName(tag)

	if s.Has("[item group name]") {
		e.ItemGroup = itemGroupName(firstStr(s, "[item group name]", ""))
	}

	e.PhysicalAttack = arrInt(s, "[equipment physical attack]")
	e.MagicalAttack = arrInt(s, "[equipment magical attack]")
	e.SeparateAttack = arrInt(s, "[separate attack]")
	e.PhysicalDefense = arrInt(s, "[equipment physical defense]")
	e.MagicalDefense = arrInt(s, "[equipment magical defense]")

	e.Strength = intOrNil(s, "[physical attack]")
	e.Intelligence = intOrNil(s, "[magical attack]")
	e.Vitality = intOrNil(s, "[physical defense]")
	e.Spirit = intOrNil(s, "[magical defense]")

	e.HpMax = intOrNil(s, "[HP MAX]")
	e.MpMax = intOrNil(s, "[MP MAX]")
	e.HpRegenSpeed = intOrNil(s, "[HP regen speed]")
	e.MpRegenSpeed = intOrNil(s, "[MP regen speed]")

	e.AttackSpeed = intOrNil(s, "[attack speed]")
	e.CastSpeed = intOrNil(s, "[cast speed]")
	e.MoveSpeed = intOrNil(s, "[move speed]")
	e.PhysicalCriticalHit = intOrNil(s, "[physical critical hit]")
	e.MagicalCriticalHit = intOrNil(s, "[magical critical hit]")

	e.HitRate = intOrNil(s, "[stuck]")
	if e.HitRate != nil {
		neg := -*e.HitRate
		e.HitRate = &neg
	}
	e.Dodge = intOrNil(s, "[stuck resistance]")
	e.HitRecovery = intOrNil(s, "[hit recovery]")
	e.JumpPower = intOrNil(s, "[jump power]")
	e.InventoryLimit = intOrNil(s, "[inventory limit]")
	e.AntiEvil = intOrNil(s, "[anti evil]")

	e.FireElement = intOrNil(s, "[fire element]")
	e.WaterElement = intOrNil(s, "[water element]")
	e.LightElement = intOrNil(s, "[light element]")
	e.DarkElement = intOrNil(s, "[dark element]")
	e.FireAttack = intOrNil(s, "[fire attack]")
	e.WaterAttack = intOrNil(s, "[water attack]")
	e.LightAttack = intOrNil(s, "[light attack]")
	e.DarkAttack = intOrNil(s, "[dark attack]")
	e.AllElementalAttack = intOrNil(s, "[all elemental attack]")
	e.FireResistance = intOrNil(s, "[fire resistance]")
	e.WaterResistance = intOrNil(s, "[water resistance]")
	e.LightResistance = intOrNil(s, "[light resistance]")
	e.DarkResistance = intOrNil(s, "[dark resistance]")
	e.AllElementalResistance = intOrNil(s, "[all elemental resistance]")

	e.BlindResistance = intOrNil(s, "[blind resistance]")
	e.LightningResistance = intOrNil(s, "[lightning resistance]")
	e.BurnResistance = intOrNil(s, "[burn resistance]")
	e.FreezeResistance = intOrNil(s, "[freeze resistance]")
	e.HoldResistance = intOrNil(s, "[hold resistance]")
	e.SleepResistance = intOrNil(s, "[sleep resistance]")
	e.BleedingResistance = intOrNil(s, "[bleeding resistance]")
	e.ConfuseResistance = intOrNil(s, "[confuse resistance]")
	e.CurseResistance = intOrNil(s, "[curse resistance]")
	e.StoneResistance = intOrNil(s, "[stone resistance]")
	e.AllActiveStatusResistance = intOrNil(s, "[all activestatus resistance]")

	e.Grade = intOrNil(s, "[grade]")
	e.Weight = intOrNil(s, "[weight]")
	e.Durability = intOrNil(s, "[durability]")
	e.Price = intOrNil(s, "[price]")
	e.RepairPrice = intOrNil(s, "[repair price]")
	e.Value = intOrNil(s, "[value]")
	e.RoomListMoveSpeedRate = intOrNil(s, "[room list move speed rate]")

	return e
}

// StackableForScript 把单个道具脚本解析为 Stackable。
func StackableForScript(s *OrderedMap) *Stackable {
	sk := &Stackable{}
	sk.parseForScript(s)
	sk.StackableType = stackableTypeName(firstStr(s, "[stackable type]", "[etc]"))
	return sk
}

// =============================================================================
// 列表解析（对应 PvfManager.getEquipmentList / getStackableList）
// =============================================================================

// isInvalidName 过滤无效物品（对应 Java 中 未命名 / 找不到代码 / 空名 判断）。
// "找不到代码" 在繁体 Big5 中为 "找不到代碼"，这里用前缀 "找不到代" 同时覆盖简繁。
func isInvalidName(name string) bool {
	name = strings.TrimSpace(name)
	return name == "" || name == "未命名" || strings.Contains(name, "找不到代")
}

// GetEquipmentList 解析全部装备（对应 PvfManager.getEquipmentList）。
func (p *Pvf) GetEquipmentList() []*Equipment {
	list := make([]*Equipment, 0, 4096)
	lst := p.LoadScript("equipment/equipment.lst")
	for _, key := range lst.Keys() {
		str := firstStr(lst, key, "")
		if str == "" {
			if v, ok := lst.Get(key); ok {
				str = toStr(v)
			}
		}
		str = strings.TrimSpace(str)
		if strings.HasPrefix(str, "/") {
			str = str[1:]
		}
		script := p.LoadScript("equipment/" + str)
		eq := EquipmentForScript(script)
		if id, ok := atoiSafe(key); ok {
			eq.ID = id
		}
		if isInvalidName(eq.Name) {
			continue
		}
		list = append(list, eq)
	}
	return list
}

// GetStackableList 解析全部道具（对应 PvfManager.getStackableList）。
func (p *Pvf) GetStackableList() []*Stackable {
	list := make([]*Stackable, 0, 4096)
	lst := p.LoadScript("stackable/stackable.lst")
	for _, key := range lst.Keys() {
		str := firstStr(lst, key, "")
		if str == "" {
			if v, ok := lst.Get(key); ok {
				str = toStr(v)
			}
		}
		str = strings.TrimSpace(str)
		path := "stackable/" + str
		if strings.HasPrefix(str, "/") {
			path = "stackable" + str
		}
		script := p.LoadScript(path)
		sk := StackableForScript(script)
		if id, ok := atoiSafe(key); ok {
			sk.ID = id
		}
		if isInvalidName(sk.Name) {
			continue
		}
		list = append(list, sk)
	}
	return list
}

// GetExpTable 读取 character/exptable.tbl 中的等级经验表。
func (p *Pvf) GetExpTable() []int64 {
	content := p.getTreeContent("character/exptable.tbl")
	if content == nil {
		return nil
	}
	units := parseUnits(p, "character/exptable.tbl", content, false)
	out := make([]int64, 0, len(units))
	for _, unit := range units {
		if v, ok := toInt(unit.value); ok {
			out = append(out, int64(v))
		}
	}
	return out
}

func atoiSafe(s string) (int, bool) {
	n := 0
	if s == "" {
		return 0, false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, true
}

// =============================================================================
// 图标导出（对应 NpkManager.getImageBytes）
// =============================================================================

// ItemIconPng 根据物品图标(path+index)从 NPK 图集导出 PNG 字节。
// 对应 NpkCoder.loadImg(path).getTextures()[index].toPngBytes()。
func (n *Npk) ItemIconPng(icon *ItemIcon) ([]byte, error) {
	if icon == nil || icon.Path == "" {
		return nil, fmt.Errorf("空图标")
	}
	img, err := n.LoadImg(icon.Path)
	if err != nil {
		return nil, err
	}
	if icon.Index < 0 || icon.Index >= len(img.Textures) {
		return nil, fmt.Errorf("图标索引越界: %d (共 %d)", icon.Index, len(img.Textures))
	}
	return img.Textures[icon.Index].ToPngBytes()
}

// itemGroupName: [item group name] -> 子类型中文（对应 getItemGroupDisplayName）。
func itemGroupName(g string) string {
	switch g {
	case "amulet":
		return "项链"
	case "wrist":
		return "手镯"
	case "ring":
		return "戒指"
	case "support":
		return "辅助装备"
	case "magic stone":
		return "魔法石"
	case "title":
		return "称号"
	case "coat":
		return "上衣"
	case "pants":
		return "下装"
	case "shoulder":
		return "护肩"
	case "waist":
		return "腰带"
	case "shoes":
		return "靴子"
	case "cl coat":
		return "布甲上衣"
	case "cl pants":
		return "布甲下装"
	case "cl waist":
		return "布甲腰带"
	case "cl shoes":
		return "布甲鞋子"
	case "cl shoulder":
		return "布甲护肩"
	case "lt coat":
		return "皮甲上衣"
	case "lt pants":
		return "皮甲下装"
	case "lt waist":
		return "皮甲腰带"
	case "lt shoes":
		return "皮甲鞋子"
	case "lt shoulder":
		return "皮甲护肩"
	case "la coat":
		return "轻甲上衣"
	case "la pants":
		return "轻甲下装"
	case "la waist":
		return "轻甲腰带"
	case "la shoes":
		return "轻甲鞋子"
	case "la shoulder":
		return "轻甲护肩"
	case "ha coat":
		return "重甲上衣"
	case "ha pants":
		return "重甲下装"
	case "ha waist":
		return "重甲腰带"
	case "ha shoes":
		return "重甲鞋子"
	case "ha shoulder":
		return "重甲护肩"
	case "mt coat":
		return "板甲上衣"
	case "mt pants":
		return "板甲下装"
	case "mt waist":
		return "板甲腰带"
	case "mt shoes":
		return "板甲鞋子"
	case "mt shoulder":
		return "板甲护肩"
	case "hat avatar":
		return "帽子"
	case "hair avatar":
		return "发型"
	case "face avatar":
		return "脸部"
	case "breast avatar":
		return "颈部"
	case "coat avatar":
		return "上衣(装扮)"
	case "pants avatar":
		return "裤子(装扮)"
	case "waist avatar":
		return "腰带(装扮)"
	case "shoes avatar":
		return "鞋子(装扮)"
	case "skin avatar":
		return "皮肤"
	case "aurora avatar":
		return "光环"
	case "artifact blue":
		return "宠物装备(蓝)"
	case "artifact green":
		return "宠物装备(绿)"
	case "artifact red":
		return "宠物装备(红)"
	case "ssword":
		return "短剑"
	case "katana":
		return "太刀"
	case "club":
		return "钝器"
	case "lswd":
		return "巨剑"
	case "beamswd":
		return "光剑"
	case "knuckle":
		return "手套"
	case "claw":
		return "爪"
	case "tonfa":
		return "东方棍"
	case "gauntlet":
		return "臂铠"
	case "bglove":
		return "拳套"
	case "automatic":
		return "自动步枪"
	case "revolver":
		return "左轮"
	case "bowgun":
		return "手弩"
	case "musket":
		return "步枪"
	case "hcannon":
		return "手炮"
	case "rod":
		return "魔杖"
	case "staff":
		return "法杖"
	case "pole":
		return "棍棒"
	case "spear":
		return "矛"
	case "broom":
		return "扫把"
	case "wand":
		return "手杖"
	case "cross":
		return "十字架"
	case "rosary":
		return "念珠"
	case "totem":
		return "图腾"
	case "axe":
		return "斧头"
	case "scythe":
		return "镰刀"
	case "dagger":
		return "匕首"
	case "twinswd":
		return "双剑"
	default:
		return g
	}
}
