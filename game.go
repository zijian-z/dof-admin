// game.go —— DNF 服务端数据库侧功能：邮件发货 + 角色管理 + 角色创建限制重置。
//
// 对应原 Java 工程的:
//   - service/impl/PostalServiceImpl（邮件发货/列表）+ entity/Postal
//   - service/impl/CharacServiceImpl（角色查询/修改）+ entity/CharacInfo
//   - task/MemberTask.roleCreateLimitReset（每 3 秒 TRUNCATE 创建限制表）
//
// 直接以原生 SQL 操作 DNF 多个库（taiwan_cain_2nd / taiwan_cain / d_taiwan），
// 与原工程「root 直连 + SQL 显式写库名」的方式一致。
//
// 依赖：github.com/go-sql-driver/mysql（标准 database/sql 驱动）。
package dnfparser

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// GameDB 封装到 DNF 服务端 MySQL 的连接。
type GameDB struct {
	db *sql.DB
}

// OpenGameDB 打开数据库连接。
//
// dsn 形如:
//
//	root:password@tcp(100.71.173.105:3000)/?charset=utf8mb4&parseTime=true&loc=Local
//
// 不指定默认库，因为各功能 SQL 都显式写了库名（taiwan_cain_2nd.postal 等）。
func OpenGameDB(dsn string) (*GameDB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}
	return &GameDB{db: db}, nil
}

// BuildDSN 按主机/端口/账号/密码拼一个常用 DSN。
func BuildDSN(host string, port int, user, password, charset string) string {
	charset = strings.TrimSpace(charset)
	if charset == "" {
		charset = "utf8mb4"
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=%s&parseTime=true&loc=Local",
		user, password, host, port, charset)
}

// Close 关闭连接。
func (g *GameDB) Close() error { return g.db.Close() }

// =============================================================================
// 一、邮件发货（taiwan_cain_2nd.postal）
// =============================================================================

// Mail 一封要发送的物品邮件（对应 entity.Postal 的可写字段）。
type Mail struct {
	SendCharacName  string // 发件人昵称，留空则用 "DNF Manager"
	ReceiveCharacNo string // 收件人角色ID（charac_no）
	ItemID          int64  // 物品代码（附件物品ID）
	Count           int    // 物品数量（add_info），<=0 视为 1
	Upgrade         int    // 强化等级
	SeperateUpgrade int    // 锻造等级
	AmplifyOption   int    // 红字属性类型
	AmplifyValue    int    // 红字属性值
	Gold            int    // 金币数量
	Seal            bool   // 物品是否封装（SS 禁止封装）
	LetterID        int    // 信件ID，纯物品邮件传 0
	Message         string // 信件正文，非空时自动创建 letter
	Avatar          bool   // 时装邮件：先写 user_items，再把 ui_id 放入 add_info
	Creature        bool   // 宠物邮件：先写 creature_items，再把 ui_id 放入 add_info
	Endurance       int    // 耐久
}

// SendMail 向游戏角色发送一封物品邮件（对应 PostalServiceImpl.sendMail）。
// 原理：直接 INSERT 一行到 taiwan_cain_2nd.postal，玩家登录后即可在邮箱领取。
// 返回新邮件的 postal_id。
func (g *GameDB) SendMail(m Mail) (int64, error) {
	if strings.TrimSpace(m.SendCharacName) == "" {
		m.SendCharacName = "DNF Manager"
	}
	if m.Count <= 0 {
		m.Count = 1
	}
	if m.Avatar && m.Creature {
		return 0, fmt.Errorf("发送邮件失败: avatar and creature cannot both be true")
	}
	if strings.TrimSpace(m.Message) != "" && m.LetterID == 0 {
		letterID, err := g.CreateLetter(m.ReceiveCharacNo, m.SendCharacName, m.Message)
		if err != nil {
			return 0, err
		}
		m.LetterID = int(letterID)
	}

	occTime := time.Now()
	addInfo := int64(m.Count)
	if m.Avatar {
		uiID, err := g.createUserItem(m.ReceiveCharacNo, m.ItemID, occTime)
		if err != nil {
			return 0, err
		}
		addInfo = uiID
	} else if m.Creature {
		uiID, err := g.createCreatureItem(m.ReceiveCharacNo, m.ItemID, occTime)
		if err != nil {
			return 0, err
		}
		addInfo = uiID
	}

	const q = "INSERT INTO `taiwan_cain_2nd`.`postal` " +
		"(occ_time, send_charac_name, receive_charac_no, amplify_option, amplify_value, " +
		" seperate_upgrade, seal_flag, item_id, add_info, `upgrade`, gold, letter_id, " +
		" avata_flag, creature_flag, endurance, unlimit_flag) " +
		"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)"
	res, err := g.db.Exec(q,
		occTime, m.SendCharacName, m.ReceiveCharacNo, m.AmplifyOption, m.AmplifyValue,
		m.SeperateUpgrade, boolToInt(m.Seal), m.ItemID, addInfo, m.Upgrade,
		m.Gold, m.LetterID, boolToInt(m.Avatar), boolToInt(m.Creature), m.Endurance)
	if err != nil {
		return 0, fmt.Errorf("发送邮件失败: %w", err)
	}
	return res.LastInsertId()
}

// CreateLetter 创建一封信件正文，返回 letter_id。
func (g *GameDB) CreateLetter(characNo string, sender string, message string) (int64, error) {
	if strings.TrimSpace(sender) == "" {
		sender = "DNF Manager"
	}
	const q = "INSERT INTO `taiwan_cain_2nd`.`letter` " +
		"(charac_no, send_charac_no, send_charac_name, letter_text, reg_date, stat) " +
		"VALUES (?, 0, ?, ?, NOW(), 1)"
	res, err := g.db.Exec(q, characNo, sender, message)
	if err != nil {
		return 0, fmt.Errorf("创建信件失败: %w", err)
	}
	return res.LastInsertId()
}

func (g *GameDB) createUserItem(characNo string, itemID int64, occTime time.Time) (int64, error) {
	const insertQ = "INSERT INTO `taiwan_cain_2nd`.`user_items` " +
		"(charac_no, it_id, expire_date, obtain_from, reg_date, stat) " +
		"VALUES (?, ?, '9999-12-31 23:59:59', 1, ?, 2)"
	res, err := g.db.Exec(insertQ, characNo, itemID, occTime)
	if err != nil {
		return 0, fmt.Errorf("创建时装附件失败: %w", err)
	}
	if id, err := res.LastInsertId(); err == nil && id > 0 {
		return id, nil
	}
	var id int64
	const selectQ = "SELECT ui_id FROM `taiwan_cain_2nd`.`user_items` " +
		"WHERE charac_no=? AND it_id=? AND reg_date=? ORDER BY ui_id DESC LIMIT 1"
	if err := g.db.QueryRow(selectQ, characNo, itemID, occTime).Scan(&id); err != nil {
		return 0, fmt.Errorf("查询时装附件失败: %w", err)
	}
	return id, nil
}

func (g *GameDB) createCreatureItem(characNo string, itemID int64, occTime time.Time) (int64, error) {
	const insertQ = "INSERT INTO `taiwan_cain_2nd`.`creature_items` " +
		"(charac_no, it_id, expire_date, reg_date, stat, item_lock_key, creature_type, stomach) " +
		"VALUES (?, ?, '9999-12-31 23:59:59', ?, 0, 0, 1, 100)"
	res, err := g.db.Exec(insertQ, characNo, itemID, occTime)
	if err != nil {
		return 0, fmt.Errorf("创建宠物附件失败: %w", err)
	}
	if id, err := res.LastInsertId(); err == nil && id > 0 {
		return id, nil
	}
	var id int64
	const selectQ = "SELECT ui_id FROM `taiwan_cain_2nd`.`creature_items` " +
		"WHERE charac_no=? AND it_id=? AND reg_date=? ORDER BY ui_id DESC LIMIT 1"
	if err := g.db.QueryRow(selectQ, characNo, itemID, occTime).Scan(&id); err != nil {
		return 0, fmt.Errorf("查询宠物附件失败: %w", err)
	}
	return id, nil
}

// MailRow 邮件列表中的一行（查询返回）。
type MailRow struct {
	PostalID        int64     `json:"postalId"`
	OccTime         time.Time `json:"occTime"`
	SendCharacName  string    `json:"sendCharacName"`
	ReceiveCharacNo string    `json:"receiveCharacNo"`
	ItemID          int64     `json:"itemId"`
	AddInfo         int64     `json:"addInfo"`
	Count           int       `json:"count"`
	Upgrade         int       `json:"upgrade"`
	SeperateUpgrade int       `json:"seperateUpgrade"`
	AmplifyOption   int       `json:"amplifyOption"`
	AmplifyValue    int       `json:"amplifyValue"`
	Gold            int       `json:"gold"`
	LetterID        int       `json:"letterId"`
	Avatar          bool      `json:"avatar"`
	Creature        bool      `json:"creature"`
	DeleteFlag      int       `json:"deleteFlag"`
}

// ListMail 分页查询邮件（对应 PostalServiceImpl.list），按 postal_id 倒序。
// receiveCharacNo 为空表示不过滤收件人；start/end 为 nil 表示不过滤时间。
func (g *GameDB) ListMail(receiveCharacNo string, start, end *time.Time, page, pageSize int) ([]MailRow, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	var (
		where []string
		args  []interface{}
	)
	if strings.TrimSpace(receiveCharacNo) != "" {
		where = append(where, "receive_charac_no = ?")
		args = append(args, receiveCharacNo)
	}
	where = append(where, "delete_flag = 0")
	if start != nil {
		where = append(where, "occ_time >= ?")
		args = append(args, *start)
	}
	if end != nil {
		where = append(where, "occ_time <= ?")
		args = append(args, *end)
	}
	q := "SELECT postal_id, occ_time, send_charac_name, receive_charac_no, item_id, " +
		"add_info, `upgrade`, seperate_upgrade, amplify_option, amplify_value, gold, letter_id, avata_flag, creature_flag, delete_flag " +
		"FROM `taiwan_cain_2nd`.`postal`"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY postal_id DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := g.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MailRow
	for rows.Next() {
		var r MailRow
		var avatarFlag, creatureFlag int
		var addInfo int64
		if err := rows.Scan(&r.PostalID, &r.OccTime, &r.SendCharacName, &r.ReceiveCharacNo,
			&r.ItemID, &addInfo, &r.Upgrade, &r.SeperateUpgrade, &r.AmplifyOption, &r.AmplifyValue, &r.Gold, &r.LetterID,
			&avatarFlag, &creatureFlag, &r.DeleteFlag); err != nil {
			return nil, err
		}
		r.AddInfo = addInfo
		r.Avatar = avatarFlag == 1
		r.Creature = creatureFlag == 1
		r.Count = int(addInfo)
		if r.Avatar || r.Creature {
			r.Count = 1
		}
		r.SendCharacName = TradToSimp(r.SendCharacName)
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteMail 逻辑删除单封邮件；若是时装/宠物附件，同步删除临时附件记录。
func (g *GameDB) DeleteMail(postalID int64) error {
	var avatarFlag, creatureFlag int
	var addInfo int64
	err := g.db.QueryRow("SELECT avata_flag, creature_flag, add_info FROM `taiwan_cain_2nd`.`postal` WHERE postal_id = ?", postalID).
		Scan(&avatarFlag, &creatureFlag, &addInfo)
	if err == sql.ErrNoRows {
		return fmt.Errorf("邮件不存在: %d", postalID)
	}
	if err != nil {
		return fmt.Errorf("查询邮件失败: %w", err)
	}
	if avatarFlag == 1 && addInfo > 0 {
		_, _ = g.db.Exec("DELETE FROM `taiwan_cain_2nd`.`user_items` WHERE ui_id = ?", addInfo)
	}
	if creatureFlag == 1 && addInfo > 0 {
		_, _ = g.db.Exec("DELETE FROM `taiwan_cain_2nd`.`creature_items` WHERE ui_id = ?", addInfo)
	}
	if _, err := g.db.Exec("UPDATE `taiwan_cain_2nd`.`postal` SET delete_flag = 1 WHERE postal_id = ?", postalID); err != nil {
		return fmt.Errorf("删除邮件失败: %w", err)
	}
	return nil
}

// DeleteMailByCharac 逻辑删除角色全部未删除邮件。
func (g *GameDB) DeleteMailByCharac(characNo string) error {
	rows, err := g.db.Query("SELECT postal_id FROM `taiwan_cain_2nd`.`postal` WHERE receive_charac_no = ? AND delete_flag = 0", characNo)
	if err != nil {
		return fmt.Errorf("查询角色邮件失败: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if err := g.DeleteMail(id); err != nil {
			return err
		}
	}
	return nil
}

// =============================================================================
// 二、角色管理（taiwan_cain.charac_info）
// =============================================================================

// Charac 角色信息（对应 entity.CharacInfo）。
type Charac struct {
	Mid         int64     `json:"mid"`
	AccountName string    `json:"accountName,omitempty"`
	CharacNo    int       `json:"characNo"`
	CharacName  string    `json:"characName"`
	Job         int       `json:"job"`
	GrowType    int       `json:"growType"`
	DeleteFlag  int       `json:"deleteFlag"`
	ExpertJob   int       `json:"expertJob"`
	Lev         int       `json:"lev"`
	Exp         int       `json:"exp"`
	HP          int       `json:"hp"`
	MaxHP       int       `json:"maxHp"`
	MaxMP       int       `json:"maxMp"`
	PhyAttack   int       `json:"phyAttack"`
	PhyDefense  int       `json:"phyDefense"`
	MagAttack   int       `json:"magAttack"`
	MagDefense  int       `json:"magDefense"`
	AttackSpeed int       `json:"attackSpeed"`
	CastSpeed   int       `json:"castSpeed"`
	MoveSpeed   int       `json:"moveSpeed"`
	HitRecovery int       `json:"hitRecovery"`
	Jump        int       `json:"jump"`
	Fatigue     int       `json:"fatigue"`
	CreateTime  time.Time `json:"createTime"`
}

// AccountResources 汇总账号/角色常用运营资源。
type AccountResources struct {
	UID              int64  `json:"uid"`
	AccountName      string `json:"accountName,omitempty"`
	CharacNo         int    `json:"characNo,omitempty"`
	CharacName       string `json:"characName,omitempty"`
	Job              int    `json:"job"`
	GrowType         int    `json:"growType"`
	ExpertJob        int    `json:"expertJob"`
	Lev              int    `json:"lev"`
	Cera             int64  `json:"cera"`
	CeraPoint        int64  `json:"ceraPoint"`
	AccountMoney     int64  `json:"accountMoney"`
	CharacMoney      int64  `json:"characMoney,omitempty"`
	AvatarCoin       int64  `json:"avatarCoin"`
	SP               int64  `json:"sp"`
	SP2              int64  `json:"sp2"`
	TP               int64  `json:"tp"`
	TP2              int64  `json:"tp2"`
	QP               int64  `json:"qp"`
	PayCoin          int64  `json:"payCoin"`
	PVPGrade         int    `json:"pvpGrade"`
	PVPWin           int    `json:"pvpWin"`
	PVPPoint         int    `json:"pvpPoint"`
	PVPWinPoint      int    `json:"pvpWinPoint"`
	CreateLimitCount int64  `json:"createLimitCount"`
	Banned           bool   `json:"banned"`
	BanEndTime       string `json:"banEndTime,omitempty"`
	BanReason        string `json:"banReason,omitempty"`
}

// ResourcePatch 表示一次资源调整。Mode 支持 add/set/clear。
type ResourcePatch struct {
	Target string
	Mode   string
	Value  int64
}

type PVPInfo struct {
	Grade    int `json:"grade"`
	Win      int `json:"win"`
	Point    int `json:"point"`
	WinPoint int `json:"winPoint"`
}

// CharacQuery 角色查询条件（对应 CharacServiceImpl.list 的参数）。
type CharacQuery struct {
	LevMin   *int   // 默认 1
	LevMax   *int   // 默认 999
	Job      *int   // 职业，nil 不过滤
	Name     string // 角色名（简体，内部会按需转繁体匹配）
	Account  string // 账号名，按 d_taiwan.accounts.accountname LIKE 过滤
	Mid      *int64 // 限定账号UID(m_id)，nil 不过滤
	Page     int
	PageSize int
}

// SimpToTrad 用于把简体角色名转回台服存储的繁体后再做 LIKE 匹配。
// 默认原样返回，调用方可替换为自己的转换器。
var SimpToTrad = func(s string) string { return s }

// ListCharac 分页查询角色（对应 CharacServiceImpl.list 的核心查询部分）。
// 注意：原工程的多级代理权限隔离（按登录用户的 parent_uid 过滤）属于 Web 鉴权层，
// 这里通过 CharacQuery.Mid 提供等价的「按账号UID限定」能力，由调用方决定是否设置。
func (g *GameDB) ListCharac(q CharacQuery) (list []Charac, total int, err error) {
	levMin, levMax := 1, 999
	if q.LevMin != nil {
		levMin = *q.LevMin
	}
	if q.LevMax != nil {
		levMax = *q.LevMax
	}
	page, pageSize := q.Page, q.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	where := []string{"lev >= ?", "lev <= ?"}
	args := []interface{}{levMin, levMax}
	if q.Job != nil {
		where = append(where, "job = ?")
		args = append(args, *q.Job)
	}
	if strings.TrimSpace(q.Name) != "" {
		where = append(where, "charac_name LIKE ?")
		args = append(args, "%"+SimpToTrad(q.Name)+"%")
	}
	if strings.TrimSpace(q.Account) != "" {
		where = append(where, "m_id IN (SELECT UID FROM `d_taiwan`.`accounts` WHERE accountname LIKE ?)")
		args = append(args, "%"+strings.TrimSpace(q.Account)+"%")
	}
	if q.Mid != nil {
		where = append(where, "m_id = ?")
		args = append(args, *q.Mid)
	}
	whereSQL := " WHERE " + strings.Join(where, " AND ")

	// 总数
	if err = g.db.QueryRow("SELECT COUNT(*) FROM `taiwan_cain`.`charac_info`"+whereSQL, args...).
		Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []Charac{}, 0, nil
	}

	listSQL := characSelectCols + "FROM `taiwan_cain`.`charac_info`" + whereSQL +
		" ORDER BY charac_no DESC LIMIT ? OFFSET ?"
	listArgs := append(append([]interface{}{}, args...), pageSize, (page-1)*pageSize)
	rows, err := g.db.Query(listSQL, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		c, e := scanCharac(rows)
		if e != nil {
			return nil, 0, e
		}
		list = append(list, c)
	}
	if err := g.fillCharacAccountNames(list); err != nil {
		return nil, 0, err
	}
	return list, total, rows.Err()
}

// GetCharac 按角色ID查询单个角色（对应 characInfoDao.get）。
func (g *GameDB) GetCharac(characNo int) (*Charac, error) {
	row := g.db.QueryRow(characSelectCols+
		"FROM `taiwan_cain`.`charac_info` WHERE charac_no = ?", characNo)
	c, err := scanCharac(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.CharacName = TradToSimp(c.CharacName)
	c.AccountName = g.accountNameByUID(c.Mid)
	return &c, nil
}

// ListCharacByAccountUid 查询某账号UID下的全部角色（对应 CharacServiceImpl.list(String account)）。
func (g *GameDB) ListCharacByAccountUid(uid int64) ([]Charac, error) {
	rows, err := g.db.Query(characSelectCols+
		"FROM `taiwan_cain`.`charac_info` WHERE m_id = ?", uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Charac
	for rows.Next() {
		c, e := scanCharac(rows)
		if e != nil {
			return nil, e
		}
		c.CharacName = TradToSimp(c.CharacName)
		list = append(list, c)
	}
	if err := g.fillCharacAccountNames(list); err != nil {
		return nil, err
	}
	return list, rows.Err()
}

// UpdateCharac 修改角色属性（对应 CharacServiceImpl.update）。
// 仅更新可编辑字段：三速、疲劳、红蓝、四攻防、硬直、跳跃；以 CharacNo 定位。
func (g *GameDB) UpdateCharac(c Charac) error {
	const q = "UPDATE `taiwan_cain`.`charac_info` SET " +
		"attack_speed=?, cast_speed=?, move_speed=?, fatigue=?, maxHP=?, maxMP=?, " +
		"mag_attack=?, mag_defense=?, phy_attack=?, phy_defense=?, hit_recovery=?, jump=? " +
		"WHERE charac_no=?"
	_, err := g.db.Exec(q,
		c.AttackSpeed, c.CastSpeed, c.MoveSpeed, c.Fatigue, c.MaxHP, c.MaxMP,
		c.MagAttack, c.MagDefense, c.PhyAttack, c.PhyDefense, c.HitRecovery, c.Jump,
		c.CharacNo)
	if err != nil {
		return fmt.Errorf("更新角色失败: %w", err)
	}
	return nil
}

const characSelectCols = "SELECT m_id, charac_no, charac_name, job, grow_type, delete_flag, expert_job, lev, exp, HP, maxHP, maxMP, " +
	"phy_attack, phy_defense, mag_attack, mag_defense, attack_speed, cast_speed, move_speed, " +
	"hit_recovery, jump, fatigue, create_time "

// rowScanner 让 *sql.Row 与 *sql.Rows 共用扫描逻辑。
type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanCharac(s rowScanner) (Charac, error) {
	var c Charac
	err := s.Scan(&c.Mid, &c.CharacNo, &c.CharacName, &c.Job, &c.GrowType, &c.DeleteFlag, &c.ExpertJob, &c.Lev, &c.Exp, &c.HP,
		&c.MaxHP, &c.MaxMP, &c.PhyAttack, &c.PhyDefense, &c.MagAttack, &c.MagDefense,
		&c.AttackSpeed, &c.CastSpeed, &c.MoveSpeed, &c.HitRecovery, &c.Jump, &c.Fatigue,
		&c.CreateTime)
	c.CharacName = TradToSimp(c.CharacName)
	return c, err
}

// FindAccountByName 按账号名定位账号。先精确匹配，未命中再按 LIKE 匹配；LIKE 多命中会返回错误。
func (g *GameDB) FindAccountByName(account string) (uid int64, accountName string, err error) {
	account = strings.TrimSpace(account)
	if account == "" {
		return 0, "", fmt.Errorf("账号名不能为空")
	}
	if err := g.db.QueryRow("SELECT UID, accountname FROM `d_taiwan`.`accounts` WHERE accountname = ? LIMIT 1", account).
		Scan(&uid, &accountName); err == nil {
		return uid, accountName, nil
	} else if err != sql.ErrNoRows {
		return 0, "", fmt.Errorf("查询账号失败: %w", err)
	}

	rows, err := g.db.Query("SELECT UID, accountname FROM `d_taiwan`.`accounts` WHERE accountname LIKE ? ORDER BY UID DESC LIMIT 2", "%"+account+"%")
	if err != nil {
		return 0, "", fmt.Errorf("查询账号失败: %w", err)
	}
	defer rows.Close()
	var matches []struct {
		uid  int64
		name string
	}
	for rows.Next() {
		var m struct {
			uid  int64
			name string
		}
		if err := rows.Scan(&m.uid, &m.name); err != nil {
			return 0, "", err
		}
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return 0, "", err
	}
	if len(matches) == 0 {
		return 0, "", fmt.Errorf("账号不存在: %s", account)
	}
	if len(matches) > 1 {
		return 0, "", fmt.Errorf("账号名不唯一，请使用账号 UID 或更精确的账号名")
	}
	return matches[0].uid, matches[0].name, nil
}

// FindCharacByName 按角色名定位角色。先精确匹配，未命中再按 LIKE 匹配；LIKE 多命中会返回错误。
func (g *GameDB) FindCharacByName(name string) (*Charac, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("角色名不能为空")
	}
	tradName := SimpToTrad(name)
	scanOne := func(q string, arg string) (*Charac, error) {
		row := g.db.QueryRow(characSelectCols+"FROM `taiwan_cain`.`charac_info` "+q, arg)
		c, err := scanCharac(row)
		if err == sql.ErrNoRows {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		c.AccountName = g.accountNameByUID(c.Mid)
		return &c, nil
	}
	if c, err := scanOne("WHERE charac_name = ? LIMIT 1", tradName); err != nil || c != nil {
		return c, err
	}

	rows, err := g.db.Query(characSelectCols+
		"FROM `taiwan_cain`.`charac_info` WHERE charac_name LIKE ? ORDER BY charac_no DESC LIMIT 2", "%"+tradName+"%")
	if err != nil {
		return nil, fmt.Errorf("查询角色失败: %w", err)
	}
	defer rows.Close()
	var matches []Charac
	for rows.Next() {
		c, err := scanCharac(rows)
		if err != nil {
			return nil, err
		}
		matches = append(matches, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("角色不存在: %s", name)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("角色名不唯一，请使用角色 ID 或更精确的角色名")
	}
	c := matches[0]
	c.AccountName = g.accountNameByUID(c.Mid)
	return &c, nil
}

func (g *GameDB) accountNameByUID(uid int64) string {
	if uid <= 0 {
		return ""
	}
	var name sql.NullString
	if err := g.db.QueryRow("SELECT accountname FROM `d_taiwan`.`accounts` WHERE UID = ? LIMIT 1", uid).Scan(&name); err == nil && name.Valid {
		return name.String
	}
	return ""
}

func (g *GameDB) fillCharacAccountNames(list []Charac) error {
	if len(list) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(list))
	seen := make(map[int64]struct{})
	for _, c := range list {
		if c.Mid <= 0 {
			continue
		}
		if _, ok := seen[c.Mid]; ok {
			continue
		}
		seen[c.Mid] = struct{}{}
		ids = append(ids, c.Mid)
	}
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	rows, err := g.db.Query("SELECT UID, accountname FROM `d_taiwan`.`accounts` WHERE UID IN ("+strings.Join(placeholders, ",")+")", args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	names := make(map[int64]string)
	for rows.Next() {
		var uid int64
		var name string
		if err := rows.Scan(&uid, &name); err != nil {
			return err
		}
		names[uid] = name
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range list {
		list[i].AccountName = names[list[i].Mid]
	}
	return nil
}

// RenameCharac 修改角色名。调用方负责传入正确编码/繁简后的名称。
func (g *GameDB) RenameCharac(characNo int, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("角色名不能为空")
	}
	var existing int
	err := g.db.QueryRow("SELECT charac_no FROM `taiwan_cain`.`charac_info` WHERE charac_name = ? AND charac_no <> ? LIMIT 1", name, characNo).Scan(&existing)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("检查角色名失败: %w", err)
	}
	if existing != 0 {
		return fmt.Errorf("角色名已存在")
	}
	if _, err := g.db.Exec("UPDATE `taiwan_cain`.`charac_info` SET charac_name = ? WHERE charac_no = ?", name, characNo); err != nil {
		return fmt.Errorf("修改角色名失败: %w", err)
	}
	return nil
}

// SetCharacLevel 修改角色等级；若 expTable 提供对应等级经验，同步 charac_stat.exp。
func (g *GameDB) SetCharacLevel(characNo int, level int, expTable []int64) error {
	if level < 1 || level > 999 {
		return fmt.Errorf("等级超出范围")
	}
	if _, err := g.db.Exec("UPDATE `taiwan_cain`.`charac_info` SET lev = ? WHERE charac_no = ?", level, characNo); err != nil {
		return fmt.Errorf("修改等级失败: %w", err)
	}
	if exp := expForLevel(level, expTable); exp >= 0 {
		if _, err := g.db.Exec("UPDATE `taiwan_cain`.`charac_stat` SET exp = ? WHERE charac_no = ?", exp, characNo); err != nil {
			return fmt.Errorf("同步等级经验失败: %w", err)
		}
	}
	return nil
}

func expForLevel(level int, expTable []int64) int64 {
	if len(expTable) == 0 {
		return -1
	}
	idx := level - 1
	if idx < 0 || idx >= len(expTable) {
		return -1
	}
	return expTable[idx] + 1
}

// SetCharacJob 修改职业/转职/副职业字段。
func (g *GameDB) SetCharacJob(characNo int, job *int, growType *int, expertJob *int) error {
	sets := make([]string, 0, 3)
	args := make([]interface{}, 0, 4)
	if job != nil {
		sets = append(sets, "job = ?")
		args = append(args, *job)
	}
	if growType != nil {
		sets = append(sets, "grow_type = ?")
		args = append(args, *growType)
	}
	if expertJob != nil {
		sets = append(sets, "expert_job = ?")
		args = append(args, *expertJob)
	}
	if len(sets) == 0 {
		return fmt.Errorf("没有可更新的职业字段")
	}
	args = append(args, characNo)
	q := "UPDATE `taiwan_cain`.`charac_info` SET " + strings.Join(sets, ", ") + " WHERE charac_no = ?"
	if _, err := g.db.Exec(q, args...); err != nil {
		return fmt.Errorf("修改职业失败: %w", err)
	}
	return nil
}

// SetCharacDeleted 标记删除或恢复角色。
func (g *GameDB) SetCharacDeleted(characNo int, deleted bool) error {
	if _, err := g.db.Exec("UPDATE `taiwan_cain`.`charac_info` SET delete_flag = ? WHERE charac_no = ?", boolToInt(deleted), characNo); err != nil {
		return fmt.Errorf("更新角色删除状态失败: %w", err)
	}
	return nil
}

// MoveCharacToAccount 将角色移动到指定账号 UID。
func (g *GameDB) MoveCharacToAccount(characNo int, uid int64) error {
	if uid <= 0 {
		return fmt.Errorf("账号 UID 无效")
	}
	if _, err := g.db.Exec("UPDATE `taiwan_cain`.`charac_info` SET m_id = ? WHERE charac_no = ?", uid, characNo); err != nil {
		return fmt.Errorf("移动角色失败: %w", err)
	}
	return nil
}

// BanAccount 封禁账号。
func (g *GameDB) BanAccount(uid int64, days int, reason string) error {
	if uid <= 0 {
		return fmt.Errorf("账号 UID 无效")
	}
	if days <= 0 {
		days = 365
	}
	now := time.Now()
	end := now.AddDate(0, 0, days)
	const q = "REPLACE INTO `d_taiwan`.`member_punish_info` " +
		"(m_id, punish_type, occ_time, punish_value, apply_flag, start_time, end_time, reason) " +
		"VALUES (?, 1, ?, 101, 2, ?, ?, ?)"
	if _, err := g.db.Exec(q, uid, now, now, end, reason); err != nil {
		return fmt.Errorf("封禁账号失败: %w", err)
	}
	return nil
}

// UnbanAccount 解封账号。
func (g *GameDB) UnbanAccount(uid int64) error {
	if _, err := g.db.Exec("DELETE FROM `d_taiwan`.`member_punish_info` WHERE m_id = ?", uid); err != nil {
		return fmt.Errorf("解封账号失败: %w", err)
	}
	return nil
}

// ResetCreateLimitForAccount 将指定账号的建角限制计数清零。
func (g *GameDB) ResetCreateLimitForAccount(uid int64) error {
	res, err := g.db.Exec("UPDATE `d_taiwan`.`limit_create_character` SET count = 0 WHERE m_id = ?", uid)
	if err != nil {
		return fmt.Errorf("重置账号创建限制失败: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		_, err = g.db.Exec("INSERT INTO `d_taiwan`.`limit_create_character` (m_id, count) VALUES (?, 0)", uid)
		if err != nil {
			return fmt.Errorf("初始化账号创建限制失败: %w", err)
		}
	}
	return nil
}

// GetAccountResources 汇总账号与可选角色资源。
func (g *GameDB) GetAccountResources(uid int64, characNo int) (AccountResources, error) {
	out := AccountResources{UID: uid, CharacNo: characNo}
	var c *Charac
	var err error
	if uid <= 0 && characNo > 0 {
		c, err = g.GetCharac(characNo)
		if err != nil {
			return out, err
		}
		if c == nil {
			return out, fmt.Errorf("角色不存在: %d", characNo)
		}
		uid = c.Mid
		out.UID = uid
	}
	if uid <= 0 {
		return out, fmt.Errorf("账号 UID 或角色 ID 至少提供一个")
	}
	out.AccountName = g.accountNameByUID(uid)
	if characNo > 0 {
		if c == nil {
			c, err = g.GetCharac(characNo)
			if err != nil {
				return out, err
			}
		}
		if c == nil {
			return out, fmt.Errorf("角色不存在: %d", characNo)
		}
		out.CharacName = c.CharacName
		out.Job = c.Job
		out.GrowType = c.GrowType
		out.ExpertJob = c.ExpertJob
		out.Lev = c.Lev
		if out.AccountName == "" {
			out.AccountName = c.AccountName
		}
	}

	out.Cera = g.scalarInt64("SELECT cera FROM `taiwan_billing`.`cash_cera` WHERE account = ?", uid)
	out.CeraPoint = g.scalarInt64("SELECT cera_point FROM `taiwan_billing`.`cash_cera_point` WHERE account = ?", uid)
	out.AccountMoney = g.scalarInt64("SELECT money FROM `taiwan_cain`.`account_cargo` WHERE m_id = ?", uid)
	out.AvatarCoin = g.scalarInt64("SELECT avatar_coin FROM `taiwan_cain_2nd`.`member_avatar_coin` WHERE m_id = ?", uid)
	out.CreateLimitCount = g.scalarInt64("SELECT count FROM `d_taiwan`.`limit_create_character` WHERE m_id = ?", uid)
	if characNo > 0 {
		out.CharacMoney = g.scalarInt64Any([]string{
			"SELECT money FROM `taiwan_cain_2nd`.`inventory` WHERE charac_no = ?",
			"SELECT money FROM `taiwan_cain`.`inventory` WHERE charac_no = ?",
		}, characNo)
		out.PayCoin = g.scalarInt64Any([]string{
			"SELECT pay_coin FROM `taiwan_cain_2nd`.`inventory` WHERE charac_no = ?",
			"SELECT pay_coin FROM `taiwan_cain`.`inventory` WHERE charac_no = ?",
		}, characNo)
		out.QP = g.scalarInt64("SELECT qp FROM `taiwan_cain`.`charac_quest_shop` WHERE charac_no = ?", characNo)
		_ = g.querySkillPoints(characNo, &out.SP, &out.SP2, &out.TP, &out.TP2)
		_ = g.db.QueryRow("SELECT pvp_grade, win, pvp_point, win_point FROM `taiwan_cain`.`pvp_result` WHERE charac_no = ?", characNo).
			Scan(&out.PVPGrade, &out.PVPWin, &out.PVPPoint, &out.PVPWinPoint)
	}

	var endTime sql.NullString
	var reason sql.NullString
	if err := g.db.QueryRow("SELECT end_time, reason FROM `d_taiwan`.`member_punish_info` WHERE m_id = ? LIMIT 1", uid).
		Scan(&endTime, &reason); err == nil {
		out.Banned = true
		if endTime.Valid {
			out.BanEndTime = endTime.String
		}
		if reason.Valid {
			out.BanReason = reason.String
		}
	}
	return out, nil
}

func (g *GameDB) scalarInt64(q string, args ...interface{}) int64 {
	var v sql.NullInt64
	if err := g.db.QueryRow(q, args...).Scan(&v); err == nil && v.Valid {
		return v.Int64
	}
	return 0
}

// scalarInt64Any returns the first readable integer from fallback queries.
func (g *GameDB) scalarInt64Any(queries []string, args ...interface{}) int64 {
	for _, q := range queries {
		var v sql.NullInt64
		err := g.db.QueryRow(q, args...).Scan(&v)
		if err == sql.ErrNoRows {
			continue
		}
		if err == nil && v.Valid {
			return v.Int64
		}
	}
	return 0
}

func (g *GameDB) querySkillPoints(characNo int, sp, sp2, tp, tp2 *int64) error {
	queries := []string{
		"SELECT remain_sp, remain_sp_2nd, remain_sfp_1st, remain_sfp_2nd FROM `taiwan_cain_2nd`.`skill` WHERE charac_no = ?",
		"SELECT remain_sp, remain_sp_2nd, remain_sfp_1st, remain_sfp_2nd FROM `taiwan_cain`.`skill` WHERE charac_no = ?",
	}
	for _, q := range queries {
		err := g.db.QueryRow(q, characNo).Scan(sp, sp2, tp, tp2)
		if err == nil {
			return nil
		}
		if err == sql.ErrNoRows {
			continue
		}
	}
	return sql.ErrNoRows
}

// ApplyResourcePatch 执行一次资源调整。
func (g *GameDB) ApplyResourcePatch(uid int64, characNo int, patch ResourcePatch) error {
	target := strings.TrimSpace(strings.ToLower(patch.Target))
	mode := strings.TrimSpace(strings.ToLower(patch.Mode))
	if mode == "" {
		mode = "add"
	}
	if mode != "add" && mode != "set" && mode != "clear" {
		return fmt.Errorf("资源调整模式无效: %s", patch.Mode)
	}
	value := patch.Value
	if mode == "clear" {
		mode = "set"
		value = 0
	}

	switch target {
	case "cera":
		return g.upsertCashCera(uid, value, mode)
	case "cera_point", "cerapoint":
		return g.upsertCashCeraPoint(uid, value, mode)
	case "account_money":
		return g.updateNumeric("taiwan_cain", "account_cargo", "money", "m_id", uid, value, mode)
	case "avatar_coin":
		return g.upsertAvatarCoin(uid, value, mode)
	case "create_limit":
		if mode == "add" {
			return g.updateNumeric("d_taiwan", "limit_create_character", "count", "m_id", uid, value, mode)
		}
		return g.setCreateLimit(uid, value)
	case "charac_money":
		return g.updateInventoryNumeric(characNo, "money", value, mode)
	case "pay_coin":
		return g.updateInventoryNumeric(characNo, "pay_coin", value, mode)
	case "qp":
		return g.updateNumeric("taiwan_cain", "charac_quest_shop", "qp", "charac_no", int64(characNo), value, mode)
	case "sp":
		return g.updatePairedSkill(characNo, "remain_sp", "remain_sp_2nd", value, mode)
	case "tp":
		return g.updatePairedSkill(characNo, "remain_sfp_1st", "remain_sfp_2nd", value, mode)
	default:
		return fmt.Errorf("未知资源类型: %s", patch.Target)
	}
}

func (g *GameDB) upsertCashCera(uid int64, value int64, mode string) error {
	if mode == "add" {
		const q = "INSERT INTO `taiwan_billing`.`cash_cera` (account, cera, mod_date, reg_date) VALUES (?, ?, NOW(), NOW()) " +
			"ON DUPLICATE KEY UPDATE cera = cera + VALUES(cera), mod_date = NOW()"
		_, err := g.db.Exec(q, uid, value)
		return wrapExecErr("调整D币失败", err)
	}
	const q = "INSERT INTO `taiwan_billing`.`cash_cera` (account, cera, mod_date, reg_date) VALUES (?, ?, NOW(), NOW()) " +
		"ON DUPLICATE KEY UPDATE cera = VALUES(cera), mod_date = NOW()"
	_, err := g.db.Exec(q, uid, value)
	return wrapExecErr("设置D币失败", err)
}

func (g *GameDB) upsertCashCeraPoint(uid int64, value int64, mode string) error {
	if mode == "add" {
		const q = "INSERT INTO `taiwan_billing`.`cash_cera_point` (account, cera_point, mod_date, reg_date) VALUES (?, ?, NOW(), NOW()) " +
			"ON DUPLICATE KEY UPDATE cera_point = cera_point + VALUES(cera_point), mod_date = NOW()"
		_, err := g.db.Exec(q, uid, value)
		return wrapExecErr("调整D点失败", err)
	}
	const q = "INSERT INTO `taiwan_billing`.`cash_cera_point` (account, cera_point, mod_date, reg_date) VALUES (?, ?, NOW(), NOW()) " +
		"ON DUPLICATE KEY UPDATE cera_point = VALUES(cera_point), mod_date = NOW()"
	_, err := g.db.Exec(q, uid, value)
	return wrapExecErr("设置D点失败", err)
}

func (g *GameDB) upsertAvatarCoin(uid int64, value int64, mode string) error {
	if mode == "add" {
		const q = "INSERT INTO `taiwan_cain_2nd`.`member_avatar_coin` (m_id, avatar_coin) VALUES (?, ?) " +
			"ON DUPLICATE KEY UPDATE avatar_coin = avatar_coin + VALUES(avatar_coin)"
		_, err := g.db.Exec(q, uid, value)
		return wrapExecErr("调整时装硬币失败", err)
	}
	const q = "INSERT INTO `taiwan_cain_2nd`.`member_avatar_coin` (m_id, avatar_coin) VALUES (?, ?) " +
		"ON DUPLICATE KEY UPDATE avatar_coin = VALUES(avatar_coin)"
	_, err := g.db.Exec(q, uid, value)
	return wrapExecErr("设置时装硬币失败", err)
}

func (g *GameDB) setCreateLimit(uid int64, value int64) error {
	const q = "INSERT INTO `d_taiwan`.`limit_create_character` (m_id, count) VALUES (?, ?) " +
		"ON DUPLICATE KEY UPDATE count = VALUES(count)"
	_, err := g.db.Exec(q, uid, value)
	return wrapExecErr("设置建角限制失败", err)
}

func (g *GameDB) updateNumeric(dbName, table, column, key string, keyValue int64, value int64, mode string) error {
	if keyValue <= 0 {
		return fmt.Errorf("%s.%s 需要有效定位 ID", table, column)
	}
	op := column + " = ?"
	args := []interface{}{value, keyValue}
	if mode == "add" {
		op = column + " = " + column + " + ?"
	}
	q := fmt.Sprintf("UPDATE `%s`.`%s` SET %s WHERE %s = ?", dbName, table, op, key)
	res, err := g.db.Exec(q, args...)
	if err != nil {
		return fmt.Errorf("更新%s.%s失败: %w", table, column, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("更新%s.%s失败: 未找到记录", table, column)
	}
	return nil
}

func (g *GameDB) updateInventoryNumeric(characNo int, column string, value int64, mode string) error {
	if characNo <= 0 {
		return fmt.Errorf("character id is required")
	}
	if column != "money" && column != "pay_coin" {
		return fmt.Errorf("invalid inventory column: %s", column)
	}
	var lastErr error
	for _, dbName := range []string{"taiwan_cain_2nd", "taiwan_cain"} {
		err := g.updateNumeric(dbName, "inventory", column, "charac_no", int64(characNo), value, mode)
		if err == nil {
			return nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("update inventory failed")
}

func (g *GameDB) updatePairedSkill(characNo int, col1, col2 string, value int64, mode string) error {
	if characNo <= 0 {
		return fmt.Errorf("character id is required")
	}
	setClause := fmt.Sprintf("%s = ?, %s = ?", col1, col2)
	args := []interface{}{value, value, characNo}
	if mode == "add" {
		setClause = fmt.Sprintf("%s = %s + ?, %s = %s + ?", col1, col1, col2, col2)
	}
	for _, dbName := range []string{"taiwan_cain_2nd", "taiwan_cain"} {
		q := fmt.Sprintf("UPDATE `%s`.`skill` SET %s WHERE charac_no = ?", dbName, setClause)
		res, err := g.db.Exec(q, args...)
		if err != nil {
			continue
		}
		if n, _ := res.RowsAffected(); n > 0 {
			return nil
		}
	}
	return fmt.Errorf("update skill points failed: character skill row not found")
}

// SetPVP 修改 PVP 数据。
func (g *GameDB) SetPVP(characNo int, info PVPInfo) error {
	if characNo <= 0 {
		return fmt.Errorf("角色 ID 无效")
	}
	res, err := g.db.Exec("UPDATE `taiwan_cain`.`pvp_result` SET pvp_grade=?, win=?, pvp_point=?, win_point=? WHERE charac_no=?",
		info.Grade, info.Win, info.Point, info.WinPoint, characNo)
	if err != nil {
		return fmt.Errorf("修改 PVP 数据失败: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("修改 PVP 数据失败: 未找到记录")
	}
	return nil
}

// =============================================================================
// 三、角色创建限制重置（d_taiwan.limit_create_character）
// =============================================================================

// ResetCreateLimit 清空角色创建限制表（对应 MemberTask.roleCreateLimitReset）。
// 使玩家不受「每日创建角色数量限制」约束。
func (g *GameDB) ResetCreateLimit() error {
	_, err := g.db.Exec("TRUNCATE TABLE `d_taiwan`.`limit_create_character`")
	if err != nil {
		return fmt.Errorf("重置角色创建限制失败: %w", err)
	}
	return nil
}

// StartCreateLimitResetLoop 启动后台定时重置（对应 MemberTask 的 @Scheduled，每 3 秒）。
// interval<=0 时默认为 3 秒。返回 stop 函数用于停止循环。
func (g *GameDB) StartCreateLimitResetLoop(interval time.Duration, onErr func(error)) (stop func()) {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := g.ResetCreateLimit(); err != nil && onErr != nil {
					onErr(err)
				}
			}
		}
	}()
	return func() { close(done) }
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func wrapExecErr(message string, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", message, err)
	}
	return nil
}
