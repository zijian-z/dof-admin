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
func BuildDSN(host string, port int, user, password string) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=true&loc=Local",
		user, password, host, port)
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
	const q = "INSERT INTO `taiwan_cain_2nd`.`postal` " +
		"(occ_time, send_charac_name, receive_charac_no, amplify_option, amplify_value, " +
		" seperate_upgrade, seal_flag, item_id, add_info, `upgrade`, gold, letter_id) " +
		"VALUES (NOW(), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	res, err := g.db.Exec(q,
		m.SendCharacName, m.ReceiveCharacNo, m.AmplifyOption, m.AmplifyValue,
		m.SeperateUpgrade, boolToInt(m.Seal), m.ItemID, m.Count, m.Upgrade,
		m.Gold, m.LetterID)
	if err != nil {
		return 0, fmt.Errorf("发送邮件失败: %w", err)
	}
	return res.LastInsertId()
}

// MailRow 邮件列表中的一行（查询返回）。
type MailRow struct {
	PostalID        int64     `json:"postalId"`
	OccTime         time.Time `json:"occTime"`
	SendCharacName  string    `json:"sendCharacName"`
	ReceiveCharacNo string    `json:"receiveCharacNo"`
	ItemID          int64     `json:"itemId"`
	Count           int       `json:"count"`
	Upgrade         int       `json:"upgrade"`
	SeperateUpgrade int       `json:"seperateUpgrade"`
	Gold            int       `json:"gold"`
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
	if start != nil {
		where = append(where, "occ_time >= ?")
		args = append(args, *start)
	}
	if end != nil {
		where = append(where, "occ_time <= ?")
		args = append(args, *end)
	}
	q := "SELECT postal_id, occ_time, send_charac_name, receive_charac_no, item_id, " +
		"add_info, `upgrade`, seperate_upgrade, gold FROM `taiwan_cain_2nd`.`postal`"
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
		if err := rows.Scan(&r.PostalID, &r.OccTime, &r.SendCharacName, &r.ReceiveCharacNo,
			&r.ItemID, &r.Count, &r.Upgrade, &r.SeperateUpgrade, &r.Gold); err != nil {
			return nil, err
		}
		r.SendCharacName = TradToSimp(r.SendCharacName)
		out = append(out, r)
	}
	return out, rows.Err()
}

// =============================================================================
// 二、角色管理（taiwan_cain.charac_info）
// =============================================================================

// Charac 角色信息（对应 entity.CharacInfo）。
type Charac struct {
	Mid         int64     `json:"mid"`
	CharacNo    int       `json:"characNo"`
	CharacName  string    `json:"characName"`
	Job         int       `json:"job"`
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

// CharacQuery 角色查询条件（对应 CharacServiceImpl.list 的参数）。
type CharacQuery struct {
	LevMin   *int   // 默认 1
	LevMax   *int   // 默认 999
	Job      *int   // 职业，nil 不过滤
	Name     string // 角色名（简体，内部会按需转繁体匹配）
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

const characSelectCols = "SELECT m_id, charac_no, charac_name, job, lev, exp, HP, maxHP, maxMP, " +
	"phy_attack, phy_defense, mag_attack, mag_defense, attack_speed, cast_speed, move_speed, " +
	"hit_recovery, jump, fatigue, create_time "

// rowScanner 让 *sql.Row 与 *sql.Rows 共用扫描逻辑。
type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanCharac(s rowScanner) (Charac, error) {
	var c Charac
	err := s.Scan(&c.Mid, &c.CharacNo, &c.CharacName, &c.Job, &c.Lev, &c.Exp, &c.HP,
		&c.MaxHP, &c.MaxMP, &c.PhyAttack, &c.PhyDefense, &c.MagAttack, &c.MagDefense,
		&c.AttackSpeed, &c.CastSpeed, &c.MoveSpeed, &c.HitRecovery, &c.Jump, &c.Fatigue,
		&c.CreateTime)
	return c, err
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
