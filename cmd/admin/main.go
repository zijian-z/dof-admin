package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	dnfparser "dofadmin"
)

//go:embed static/*
var staticFiles embed.FS

type server struct {
	game             *dnfparser.GameDB
	gameConfigured   bool
	gameConnectError error

	pvfPath string
	items   itemCache

	npkRoot string
	npkMu   sync.Mutex
	npk     *dnfparser.Npk
	npkErr  error
}

var (
	version   = "dev"
	commit    = "none"
	buildTime = "unknown"
)

type itemCache struct {
	once       sync.Once
	mu         sync.RWMutex
	loaded     bool
	err        error
	loadedAt   time.Time
	equipment  []*dnfparser.Equipment
	stackables []*dnfparser.Stackable
	facets     itemFacets
}

type itemFacets struct {
	Rarities       []rarityFacet `json:"rarities"`
	EquipmentTypes []string      `json:"equipmentTypes"`
	ItemGroups     []string      `json:"itemGroups"`
	StackableTypes []string      `json:"stackableTypes"`
}

type rarityFacet struct {
	Value int    `json:"value"`
	Label string `json:"label"`
}

type listResponse[T any] struct {
	Items    []T `json:"items"`
	Total    int `json:"total,omitempty"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

type mailListResponse struct {
	Items    []dnfparser.MailRow `json:"items"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"pageSize"`
	HasMore  bool                `json:"hasMore"`
}

type itemListResponse struct {
	Items    []itemSummary `json:"items"`
	Total    int           `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
	Facets   itemFacets    `json:"facets"`
	LoadedAt string        `json:"loadedAt,omitempty"`
}

type itemSummary struct {
	ID             int                 `json:"id"`
	Name           string              `json:"name"`
	Type           string              `json:"type"`
	TypeName       string              `json:"typeName"`
	Rarity         int                 `json:"rarity"`
	RarityName     string              `json:"rarityName"`
	AttachType     string              `json:"attachType"`
	MinimumLevel   int                 `json:"minimumLevel"`
	StackLimit     int                 `json:"stackLimit"`
	UsableJobs     []string            `json:"usableJobs,omitempty"`
	Icon           *dnfparser.ItemIcon `json:"icon,omitempty"`
	EquipmentType  string              `json:"equipmentType,omitempty"`
	ItemGroup      string              `json:"itemGroup,omitempty"`
	Avatar         bool                `json:"avatar,omitempty"`
	StackableType  string              `json:"stackableType,omitempty"`
	Description    string              `json:"description,omitempty"`
	ExplainPreview string              `json:"explainPreview,omitempty"`
}

type serviceState struct {
	Configured bool   `json:"configured"`
	Ready      bool   `json:"ready"`
	Detail     string `json:"detail,omitempty"`
	Error      string `json:"error,omitempty"`
}

type itemsState struct {
	Configured     bool   `json:"configured"`
	Loaded         bool   `json:"loaded"`
	PvfPath        string `json:"pvfPath,omitempty"`
	EquipmentCount int    `json:"equipmentCount"`
	StackableCount int    `json:"stackableCount"`
	LoadedAt       string `json:"loadedAt,omitempty"`
	Error          string `json:"error,omitempty"`
}

type healthResponse struct {
	Database   serviceState `json:"database"`
	Items      itemsState   `json:"items"`
	Npk        serviceState `json:"npk"`
	Build      buildState   `json:"build"`
	ServerTime string       `json:"serverTime"`
}

type buildState struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`
}

type mailPayload struct {
	SendCharacName  string `json:"sendCharacName"`
	ReceiveCharacNo string `json:"receiveCharacNo"`
	ItemID          int64  `json:"itemId"`
	Count           int    `json:"count"`
	Upgrade         int    `json:"upgrade"`
	SeperateUpgrade int    `json:"seperateUpgrade"`
	AmplifyOption   int    `json:"amplifyOption"`
	AmplifyValue    int    `json:"amplifyValue"`
	Gold            int    `json:"gold"`
	Seal            bool   `json:"seal"`
	LetterID        int    `json:"letterId"`
}

type characPatch struct {
	AttackSpeed *int `json:"attackSpeed"`
	CastSpeed   *int `json:"castSpeed"`
	MoveSpeed   *int `json:"moveSpeed"`
	Fatigue     *int `json:"fatigue"`
	MaxHP       *int `json:"maxHp"`
	MaxMP       *int `json:"maxMp"`
	MagAttack   *int `json:"magAttack"`
	MagDefense  *int `json:"magDefense"`
	PhyAttack   *int `json:"phyAttack"`
	PhyDefense  *int `json:"phyDefense"`
	HitRecovery *int `json:"hitRecovery"`
	Jump        *int `json:"jump"`
}

func main() {
	_ = mime.AddExtensionType(".js", "application/javascript; charset=utf-8")
	_ = mime.AddExtensionType(".css", "text/css; charset=utf-8")

	s := &server{
		pvfPath: resolvePVFPath(),
		npkRoot: strings.TrimSpace(os.Getenv("DNF_NPK_ROOT")),
	}
	s.openGameDB()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/characters", s.handleCharacters)
	mux.HandleFunc("/api/characters/", s.handleCharacter)
	mux.HandleFunc("/api/mail", s.handleMail)
	mux.HandleFunc("/api/reset-create-limit", s.handleResetCreateLimit)
	mux.HandleFunc("/api/items/icon", s.handleItemIcon)
	mux.HandleFunc("/api/items", s.handleItems)
	mux.HandleFunc("/api/items/", s.handleItemDetail)
	mux.HandleFunc("/", s.handleStatic)

	addr := envDefault("DNF_ADMIN_ADDR", ":8080")
	log.Printf("DNF admin UI listening on %s", addr)
	log.Printf("build version=%s commit=%s buildTime=%s", version, commit, buildTime)
	log.Printf("database configured=%t pvf=%q npkRoot=%q", s.gameConfigured, s.pvfPath, s.npkRoot)
	if err := http.ListenAndServe(addr, logRequest(mux)); err != nil {
		log.Fatal(err)
	}
}

func (s *server) openGameDB() {
	dsn := strings.TrimSpace(os.Getenv("DNF_DSN"))
	if dsn == "" {
		host := strings.TrimSpace(os.Getenv("DNF_DB_HOST"))
		if host == "" {
			return
		}
		port := intValue(os.Getenv("DNF_DB_PORT"), 3306)
		user := envDefault("DNF_DB_USER", "root")
		password := os.Getenv("DNF_DB_PASSWORD")
		if password == "" {
			password = os.Getenv("DNF_DB_PASS")
		}
		dsn = dnfparser.BuildDSN(host, port, user, password)
	}

	s.gameConfigured = true
	game, err := dnfparser.OpenGameDB(dsn)
	if err != nil {
		s.gameConnectError = err
		log.Printf("open game db failed: %v", err)
		return
	}
	s.game = game
}

func resolvePVFPath() string {
	if p := strings.TrimSpace(os.Getenv("DNF_PVF")); p != "" {
		return p
	}
	if _, err := os.Stat("extracted/Script.pvf"); err == nil {
		return "extracted/Script.pvf"
	}
	return ""
}

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	s.items.mu.RLock()
	items := itemsState{
		Configured:     s.pvfPath != "",
		Loaded:         s.items.loaded,
		PvfPath:        s.pvfPath,
		EquipmentCount: len(s.items.equipment),
		StackableCount: len(s.items.stackables),
	}
	if !s.items.loadedAt.IsZero() {
		items.LoadedAt = s.items.loadedAt.Format(time.RFC3339)
	}
	if s.items.err != nil {
		items.Error = s.items.err.Error()
	}
	s.items.mu.RUnlock()

	db := serviceState{Configured: s.gameConfigured, Ready: s.game != nil}
	switch {
	case s.game != nil:
		db.Detail = "connected"
	case s.gameConnectError != nil:
		db.Error = s.gameConnectError.Error()
	case !s.gameConfigured:
		db.Detail = "set DNF_DSN or DNF_DB_HOST to enable database APIs"
	}

	npk := serviceState{Configured: s.npkRoot != "", Ready: s.npk != nil, Detail: s.npkRoot}
	if s.npkErr != nil {
		npk.Error = s.npkErr.Error()
	}

	writeJSON(w, http.StatusOK, healthResponse{
		Database:   db,
		Items:      items,
		Npk:        npk,
		Build:      buildState{Version: version, Commit: commit, BuildTime: buildTime},
		ServerTime: time.Now().Format(time.RFC3339),
	})
}

func (s *server) handleCharacters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	game, ok := s.requireGame(w)
	if !ok {
		return
	}

	q := r.URL.Query()
	query := dnfparser.CharacQuery{
		LevMin:   optionalInt(q.Get("levMin")),
		LevMax:   optionalInt(q.Get("levMax")),
		Job:      optionalInt(q.Get("job")),
		Name:     strings.TrimSpace(q.Get("name")),
		Mid:      optionalInt64(q.Get("mid")),
		Page:     pageValue(q.Get("page")),
		PageSize: pageSizeValue(q.Get("pageSize"), 20, 100),
	}
	items, total, err := game.ListCharac(query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, listResponse[dnfparser.Charac]{
		Items:    items,
		Total:    total,
		Page:     normalizedPage(query.Page),
		PageSize: normalizedPageSize(query.PageSize, 20, 100),
	})
}

func (s *server) handleCharacter(w http.ResponseWriter, r *http.Request) {
	game, ok := s.requireGame(w)
	if !ok {
		return
	}
	id, ok := pathInt(strings.TrimPrefix(r.URL.Path, "/api/characters/"))
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid character id")
		return
	}

	switch r.Method {
	case http.MethodGet:
		c, err := game.GetCharac(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if c == nil {
			writeError(w, http.StatusNotFound, "character not found")
			return
		}
		writeJSON(w, http.StatusOK, c)
	case http.MethodPut:
		current, err := game.GetCharac(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if current == nil {
			writeError(w, http.StatusNotFound, "character not found")
			return
		}
		var patch characPatch
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
			return
		}
		applyCharacPatch(current, patch)
		if err := game.UpdateCharac(*current); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		updated, err := game.GetCharac(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, updated)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func applyCharacPatch(c *dnfparser.Charac, p characPatch) {
	if p.AttackSpeed != nil {
		c.AttackSpeed = *p.AttackSpeed
	}
	if p.CastSpeed != nil {
		c.CastSpeed = *p.CastSpeed
	}
	if p.MoveSpeed != nil {
		c.MoveSpeed = *p.MoveSpeed
	}
	if p.Fatigue != nil {
		c.Fatigue = *p.Fatigue
	}
	if p.MaxHP != nil {
		c.MaxHP = *p.MaxHP
	}
	if p.MaxMP != nil {
		c.MaxMP = *p.MaxMP
	}
	if p.MagAttack != nil {
		c.MagAttack = *p.MagAttack
	}
	if p.MagDefense != nil {
		c.MagDefense = *p.MagDefense
	}
	if p.PhyAttack != nil {
		c.PhyAttack = *p.PhyAttack
	}
	if p.PhyDefense != nil {
		c.PhyDefense = *p.PhyDefense
	}
	if p.HitRecovery != nil {
		c.HitRecovery = *p.HitRecovery
	}
	if p.Jump != nil {
		c.Jump = *p.Jump
	}
}

func (s *server) handleMail(w http.ResponseWriter, r *http.Request) {
	game, ok := s.requireGame(w)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query()
		start, err := optionalTime(q.Get("start"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid start time: "+err.Error())
			return
		}
		end, err := optionalTime(q.Get("end"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid end time: "+err.Error())
			return
		}
		page := pageValue(q.Get("page"))
		pageSize := pageSizeValue(q.Get("pageSize"), 20, 100)
		items, err := game.ListMail(strings.TrimSpace(q.Get("receiveCharacNo")), start, end, page, pageSize)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, mailListResponse{
			Items:    items,
			Page:     page,
			PageSize: pageSize,
			HasMore:  len(items) == pageSize,
		})
	case http.MethodPost:
		var payload mailPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json: "+err.Error())
			return
		}
		if strings.TrimSpace(payload.ReceiveCharacNo) == "" {
			writeError(w, http.StatusBadRequest, "receiveCharacNo is required")
			return
		}
		if payload.ItemID <= 0 && payload.Gold <= 0 && payload.LetterID <= 0 {
			writeError(w, http.StatusBadRequest, "itemId, gold or letterId is required")
			return
		}
		postalID, err := game.SendMail(dnfparser.Mail{
			SendCharacName:  payload.SendCharacName,
			ReceiveCharacNo: payload.ReceiveCharacNo,
			ItemID:          payload.ItemID,
			Count:           payload.Count,
			Upgrade:         payload.Upgrade,
			SeperateUpgrade: payload.SeperateUpgrade,
			AmplifyOption:   payload.AmplifyOption,
			AmplifyValue:    payload.AmplifyValue,
			Gold:            payload.Gold,
			Seal:            payload.Seal,
			LetterID:        payload.LetterID,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]int64{"postalId": postalID})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *server) handleResetCreateLimit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	game, ok := s.requireGame(w)
	if !ok {
		return
	}
	if err := game.ResetCreateLimit(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *server) handleItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := s.ensureItems(); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	q := r.URL.Query()
	filter := itemFilter{
		Kind:          strings.TrimSpace(q.Get("type")),
		Keyword:       strings.TrimSpace(q.Get("q")),
		Rarity:        optionalInt(q.Get("rarity")),
		MinLevel:      optionalInt(q.Get("minLevel")),
		MaxLevel:      optionalInt(q.Get("maxLevel")),
		EquipmentType: strings.TrimSpace(q.Get("equipmentType")),
		ItemGroup:     strings.TrimSpace(q.Get("itemGroup")),
		StackableType: strings.TrimSpace(q.Get("stackableType")),
		Avatar:        optionalBool(q.Get("avatar")),
	}
	if filter.Kind == "" {
		filter.Kind = "all"
	}

	page := pageValue(q.Get("page"))
	pageSize := pageSizeValue(q.Get("pageSize"), 40, 200)
	items, total, facets, loadedAt := s.queryItems(filter, page, pageSize)
	loadedAtText := ""
	if !loadedAt.IsZero() {
		loadedAtText = loadedAt.Format(time.RFC3339)
	}

	writeJSON(w, http.StatusOK, itemListResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Facets:   facets,
		LoadedAt: loadedAtText,
	})
}

func (s *server) handleItemDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := s.ensureItems(); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/items/"), "/"), "/")
	if len(parts) != 2 {
		writeError(w, http.StatusBadRequest, "expected /api/items/{equipment|stackable}/{id}")
		return
	}
	kind := parts[0]
	id, err := strconv.Atoi(parts[1])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid item id")
		return
	}

	s.items.mu.RLock()
	defer s.items.mu.RUnlock()
	switch kind {
	case "equipment":
		for _, item := range s.items.equipment {
			if item.ID == id {
				writeJSON(w, http.StatusOK, item)
				return
			}
		}
	case "stackable":
		for _, item := range s.items.stackables {
			if item.ID == id {
				writeJSON(w, http.StatusOK, item)
				return
			}
		}
	default:
		writeError(w, http.StatusBadRequest, "invalid item type")
		return
	}
	writeError(w, http.StatusNotFound, "item not found")
}

func (s *server) handleItemIcon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	iconPath := strings.TrimSpace(r.URL.Query().Get("path"))
	if iconPath == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	index := intValue(r.URL.Query().Get("index"), 0)
	npk, err := s.ensureNpk()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	pngBytes, err := npk.ItemIconPng(&dnfparser.ItemIcon{Path: iconPath, Index: index})
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(pngBytes)
}

func (s *server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "api not found")
		return
	}

	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" {
		name = "index.html"
	}
	if _, err := fs.Stat(sub, name); err != nil {
		name = "index.html"
	}
	data, err := fs.ReadFile(sub, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if strings.HasSuffix(name, ".html") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	http.ServeContent(w, r, name, time.Time{}, strings.NewReader(string(data)))
}

func (s *server) requireGame(w http.ResponseWriter) (*dnfparser.GameDB, bool) {
	if s.game != nil {
		return s.game, true
	}
	if s.gameConnectError != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable: "+s.gameConnectError.Error())
		return nil, false
	}
	writeError(w, http.StatusServiceUnavailable, "database is not configured; set DNF_DSN or DNF_DB_HOST")
	return nil, false
}

func (s *server) ensureItems() error {
	if strings.TrimSpace(s.pvfPath) == "" {
		return errors.New("PVF is not configured; set DNF_PVF or place extracted/Script.pvf under the working directory")
	}

	s.items.once.Do(func() {
		equipment, stackables, facets, loadedAt, err := loadItems(s.pvfPath)
		s.items.mu.Lock()
		defer s.items.mu.Unlock()
		s.items.err = err
		if err == nil {
			s.items.loaded = true
			s.items.loadedAt = loadedAt
			s.items.equipment = equipment
			s.items.stackables = stackables
			s.items.facets = facets
		}
	})

	s.items.mu.RLock()
	defer s.items.mu.RUnlock()
	return s.items.err
}

func loadItems(pvfPath string) (equipment []*dnfparser.Equipment, stackables []*dnfparser.Stackable, facets itemFacets, loadedAt time.Time, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("load PVF items failed: %v", recovered)
		}
	}()

	pvf, err := dnfparser.OpenPvf(pvfPath)
	if err != nil {
		return nil, nil, itemFacets{}, time.Time{}, err
	}
	equipment = pvf.GetEquipmentList()
	stackables = pvf.GetStackableList()
	facets = buildFacets(equipment, stackables)
	return equipment, stackables, facets, time.Now(), nil
}

func (s *server) ensureNpk() (*dnfparser.Npk, error) {
	if strings.TrimSpace(s.npkRoot) == "" {
		return nil, errors.New("NPK root is not configured; set DNF_NPK_ROOT to enable item icons")
	}
	s.npkMu.Lock()
	defer s.npkMu.Unlock()
	if s.npk != nil || s.npkErr != nil {
		return s.npk, s.npkErr
	}
	s.npk, s.npkErr = dnfparser.OpenNpk(s.npkRoot)
	return s.npk, s.npkErr
}

type itemFilter struct {
	Kind          string
	Keyword       string
	Rarity        *int
	MinLevel      *int
	MaxLevel      *int
	EquipmentType string
	ItemGroup     string
	StackableType string
	Avatar        *bool
}

func (s *server) queryItems(f itemFilter, page, pageSize int) ([]itemSummary, int, itemFacets, time.Time) {
	s.items.mu.RLock()
	defer s.items.mu.RUnlock()

	out := make([]itemSummary, 0, pageSize)
	total := 0
	start := (page - 1) * pageSize
	add := func(item itemSummary) {
		if total >= start && len(out) < pageSize {
			out = append(out, item)
		}
		total++
	}

	kind := strings.ToLower(f.Kind)
	if kind == "all" || kind == "equipment" {
		for _, item := range s.items.equipment {
			if matchesEquipment(item, f) {
				add(equipmentSummary(item))
			}
		}
	}
	if kind == "all" || kind == "stackable" {
		for _, item := range s.items.stackables {
			if matchesStackable(item, f) {
				add(stackableSummary(item))
			}
		}
	}
	return out, total, s.items.facets, s.items.loadedAt
}

func matchesEquipment(item *dnfparser.Equipment, f itemFilter) bool {
	if !matchesCommon(item.ID, item.Name, item.Rarity, item.MinimumLevel, f) {
		return false
	}
	if f.EquipmentType != "" && item.EquipmentType != f.EquipmentType {
		return false
	}
	if f.ItemGroup != "" && item.ItemGroup != f.ItemGroup {
		return false
	}
	if f.StackableType != "" {
		return false
	}
	if f.Avatar != nil && item.Avatar != *f.Avatar {
		return false
	}
	return true
}

func matchesStackable(item *dnfparser.Stackable, f itemFilter) bool {
	if !matchesCommon(item.ID, item.Name, item.Rarity, item.MinimumLevel, f) {
		return false
	}
	if f.StackableType != "" && item.StackableType != f.StackableType {
		return false
	}
	if f.EquipmentType != "" || f.ItemGroup != "" || f.Avatar != nil {
		return false
	}
	return true
}

func matchesCommon(id int, name string, rarity int, minimumLevel int, f itemFilter) bool {
	if f.Keyword != "" {
		keyword := strings.ToLower(f.Keyword)
		if !strings.Contains(strings.ToLower(name), keyword) && !strings.Contains(strconv.Itoa(id), keyword) {
			return false
		}
	}
	if f.Rarity != nil && rarity != *f.Rarity {
		return false
	}
	if f.MinLevel != nil && minimumLevel < *f.MinLevel {
		return false
	}
	if f.MaxLevel != nil && minimumLevel > *f.MaxLevel {
		return false
	}
	return true
}

func equipmentSummary(item *dnfparser.Equipment) itemSummary {
	return itemSummary{
		ID:             item.ID,
		Name:           item.Name,
		Type:           "equipment",
		TypeName:       "装备",
		Rarity:         item.Rarity,
		RarityName:     item.RarityName,
		AttachType:     item.AttachType,
		MinimumLevel:   item.MinimumLevel,
		StackLimit:     item.StackLimit,
		UsableJobs:     item.UsableJobs,
		Icon:           item.Icon,
		EquipmentType:  item.EquipmentType,
		ItemGroup:      item.ItemGroup,
		Avatar:         item.Avatar,
		Description:    item.Description,
		ExplainPreview: previewText(item.Explain),
	}
}

func stackableSummary(item *dnfparser.Stackable) itemSummary {
	return itemSummary{
		ID:             item.ID,
		Name:           item.Name,
		Type:           "stackable",
		TypeName:       "道具",
		Rarity:         item.Rarity,
		RarityName:     item.RarityName,
		AttachType:     item.AttachType,
		MinimumLevel:   item.MinimumLevel,
		StackLimit:     item.StackLimit,
		UsableJobs:     item.UsableJobs,
		Icon:           item.Icon,
		StackableType:  item.StackableType,
		Description:    item.Description,
		ExplainPreview: previewText(item.Explain),
	}
}

func previewText(text string) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if len([]rune(text)) <= 80 {
		return text
	}
	return string([]rune(text)[:80])
}

func buildFacets(equipment []*dnfparser.Equipment, stackables []*dnfparser.Stackable) itemFacets {
	rarityMap := make(map[int]string)
	equipmentTypes := make(map[string]struct{})
	itemGroups := make(map[string]struct{})
	stackableTypes := make(map[string]struct{})

	for _, item := range equipment {
		rarityMap[item.Rarity] = item.RarityName
		if item.EquipmentType != "" {
			equipmentTypes[item.EquipmentType] = struct{}{}
		}
		if item.ItemGroup != "" {
			itemGroups[item.ItemGroup] = struct{}{}
		}
	}
	for _, item := range stackables {
		rarityMap[item.Rarity] = item.RarityName
		if item.StackableType != "" {
			stackableTypes[item.StackableType] = struct{}{}
		}
	}

	rarityValues := make([]int, 0, len(rarityMap))
	for value := range rarityMap {
		rarityValues = append(rarityValues, value)
	}
	sort.Ints(rarityValues)
	rarities := make([]rarityFacet, 0, len(rarityValues))
	for _, value := range rarityValues {
		rarities = append(rarities, rarityFacet{Value: value, Label: rarityMap[value]})
	}

	return itemFacets{
		Rarities:       rarities,
		EquipmentTypes: sortedKeys(equipmentTypes),
		ItemGroups:     sortedKeys(itemGroups),
		StackableTypes: sortedKeys(stackableTypes),
	}
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func optionalInt(raw string) *int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return nil
	}
	return &v
}

func optionalInt64(raw string) *int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil
	}
	return &v
}

func optionalBool(raw string) *bool {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return nil
	}
	v := raw == "1" || raw == "true" || raw == "yes"
	return &v
}

func optionalTime(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	var lastErr error
	for _, layout := range layouts {
		t, err := time.ParseInLocation(layout, raw, time.Local)
		if err == nil {
			return &t, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func pathInt(raw string) (int, bool) {
	raw = strings.Trim(raw, "/")
	if raw == "" || strings.Contains(raw, "/") {
		return 0, false
	}
	v, err := strconv.Atoi(raw)
	return v, err == nil
}

func pageValue(raw string) int {
	page := intValue(raw, 1)
	if page < 1 {
		return 1
	}
	return page
}

func pageSizeValue(raw string, def, max int) int {
	size := intValue(raw, def)
	return normalizedPageSize(size, def, max)
}

func normalizedPage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func normalizedPageSize(size, def, max int) int {
	if size < 1 {
		return def
	}
	if size > max {
		return max
	}
	return size
}

func intValue(raw string, def int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}

func envDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.RequestURI(), time.Since(start).Round(time.Millisecond))
	})
}
