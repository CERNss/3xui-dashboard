// Package node is the service layer for nodes: CRUD, on-demand
// probing, and the metrics ring buffer. The periodic probe job lives
// in internal/job/probe.go; it talks to this package, not directly
// to runtime.
package node

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/runtime"
)

// Errors callers branch on.
var (
	ErrInvalidInput  = errors.New("node: invalid input")
	ErrNotFound      = errors.New("node: not found")
	ErrDuplicateName = errors.New("node: name already exists")
)

var areaAliases = map[string]string{
	"jp": "jp", "jpn": "jp", "japan": "jp", "tokyo": "jp", "tyo": "jp", "osaka": "jp", "日本": "jp", "东京": "jp", "大阪": "jp",
	"sg": "sg", "sin": "sg", "singapore": "sg", "新加坡": "sg",
	"hk": "hk", "hkg": "hk", "hong kong": "hk", "hong-kong": "hk", "hongkong": "hk", "香港": "hk",
	"tw": "tw", "twn": "tw", "taiwan": "tw", "taipei": "tw", "台湾": "tw", "台北": "tw",
	"us": "us", "usa": "us", "united states": "us", "america": "us", "new york": "us", "los angeles": "us", "la": "us", "sfo": "us", "seattle": "us", "美国": "us", "洛杉矶": "us", "纽约": "us",
	"gb": "gb", "uk": "gb", "gbr": "gb", "united kingdom": "gb", "london": "gb", "英国": "gb", "伦敦": "gb",
	"de": "de", "deu": "de", "germany": "de", "frankfurt": "de", "德国": "de", "法兰克福": "de",
	"fr": "fr", "fra": "fr", "france": "fr", "paris": "fr", "法国": "fr", "巴黎": "fr",
	"nl": "nl", "nld": "nl", "netherlands": "nl", "amsterdam": "nl", "荷兰": "nl", "阿姆斯特丹": "nl",
	"ca": "ca", "canada": "ca", "toronto": "ca", "vancouver": "ca", "加拿大": "ca", "多伦多": "ca", "温哥华": "ca",
	"au": "au", "aus": "au", "australia": "au", "sydney": "au", "melbourne": "au", "澳大利亚": "au", "悉尼": "au", "墨尔本": "au",
	"kr": "kr", "kor": "kr", "korea": "kr", "seoul": "kr", "韩国": "kr", "首尔": "kr",
	"in": "in", "ind": "in", "india": "in", "mumbai": "in", "delhi": "in", "印度": "in", "孟买": "in", "德里": "in",
	"th": "th", "tha": "th", "thailand": "th", "bangkok": "th", "泰国": "th", "曼谷": "th",
	"vn": "vn", "vnm": "vn", "vietnam": "vn", "ho chi minh": "vn", "hochiminh": "vn", "越南": "vn", "胡志明": "vn",
	"other": "unknown", "other region": "unknown", "其他": "unknown", "其他地区": "unknown",
}

var areaDetectionHints = []struct {
	match string
	area  string
}{
	{"hong kong", "hk"}, {"hong-kong", "hk"}, {"hongkong", "hk"}, {"香港", "hk"}, {"hkg", "hk"}, {"hk", "hk"},
	{"united states", "us"}, {"los angeles", "us"}, {"new york", "us"}, {"san francisco", "us"}, {"california", "us"}, {"seattle", "us"}, {"america", "us"}, {"美国", "us"}, {"洛杉矶", "us"}, {"纽约", "us"}, {"usa", "us"}, {"sfo", "us"}, {"us", "us"},
	{"redmond", "us"}, {"雷德蒙德", "us"},
	{"united kingdom", "gb"}, {"london", "gb"}, {"英国", "gb"}, {"伦敦", "gb"}, {"gbr", "gb"}, {"uk", "gb"}, {"gb", "gb"},
	{"singapore", "sg"}, {"新加坡", "sg"}, {"sin", "sg"}, {"sg", "sg"},
	{"japan", "jp"}, {"tokyo", "jp"}, {"osaka", "jp"}, {"日本", "jp"}, {"东京", "jp"}, {"大阪", "jp"}, {"jpn", "jp"}, {"tyo", "jp"}, {"jp", "jp"},
	{"taiwan", "tw"}, {"taipei", "tw"}, {"台湾", "tw"}, {"台北", "tw"}, {"twn", "tw"}, {"tw", "tw"},
	{"germany", "de"}, {"frankfurt", "de"}, {"德国", "de"}, {"法兰克福", "de"}, {"deu", "de"}, {"de", "de"},
	{"france", "fr"}, {"paris", "fr"}, {"法国", "fr"}, {"巴黎", "fr"}, {"fra", "fr"}, {"fr", "fr"},
	{"netherlands", "nl"}, {"amsterdam", "nl"}, {"荷兰", "nl"}, {"阿姆斯特丹", "nl"}, {"nld", "nl"}, {"nl", "nl"},
	{"canada", "ca"}, {"toronto", "ca"}, {"vancouver", "ca"}, {"加拿大", "ca"}, {"多伦多", "ca"}, {"温哥华", "ca"}, {"ca", "ca"},
	{"australia", "au"}, {"sydney", "au"}, {"melbourne", "au"}, {"澳大利亚", "au"}, {"悉尼", "au"}, {"墨尔本", "au"}, {"aus", "au"}, {"au", "au"},
	{"korea", "kr"}, {"seoul", "kr"}, {"韩国", "kr"}, {"首尔", "kr"}, {"kor", "kr"}, {"kr", "kr"},
	{"india", "in"}, {"mumbai", "in"}, {"delhi", "in"}, {"印度", "in"}, {"孟买", "in"}, {"德里", "in"}, {"ind", "in"}, {"in", "in"},
	{"thailand", "th"}, {"bangkok", "th"}, {"泰国", "th"}, {"曼谷", "th"}, {"tha", "th"}, {"th", "th"},
	{"vietnam", "vn"}, {"ho chi minh", "vn"}, {"hochiminh", "vn"}, {"越南", "vn"}, {"胡志明", "vn"}, {"vnm", "vn"}, {"vn", "vn"},
}

var provinceAliases = []struct {
	match    string
	area     string
	province string
}{
	{"tokyo", "jp", "Tokyo"}, {"tyo", "jp", "Tokyo"}, {"东京", "jp", "Tokyo"},
	{"osaka", "jp", "Osaka"}, {"大阪", "jp", "Osaka"},
	{"taipei", "tw", "Taipei"}, {"台北", "tw", "Taipei"},
	{"new york", "us", "New York"}, {"nyc", "us", "New York"}, {"纽约", "us", "New York"},
	{"los angeles", "us", "California"}, {"la", "us", "California"}, {"sfo", "us", "California"}, {"san francisco", "us", "California"}, {"california", "us", "California"}, {"洛杉矶", "us", "California"},
	{"seattle", "us", "Washington"},
	{"redmond", "us", "Redmond"}, {"雷德蒙德", "us", "雷德蒙德"},
	{"london", "gb", "England"}, {"伦敦", "gb", "England"},
	{"frankfurt", "de", "Hesse"}, {"法兰克福", "de", "Hesse"},
	{"paris", "fr", "Ile-de-France"}, {"巴黎", "fr", "Ile-de-France"},
	{"amsterdam", "nl", "North Holland"}, {"阿姆斯特丹", "nl", "North Holland"},
	{"toronto", "ca", "Ontario"}, {"多伦多", "ca", "Ontario"},
	{"vancouver", "ca", "British Columbia"}, {"温哥华", "ca", "British Columbia"},
	{"sydney", "au", "New South Wales"}, {"悉尼", "au", "New South Wales"},
	{"melbourne", "au", "Victoria"}, {"墨尔本", "au", "Victoria"},
	{"seoul", "kr", "Seoul"}, {"首尔", "kr", "Seoul"},
	{"mumbai", "in", "Maharashtra"}, {"孟买", "in", "Maharashtra"},
	{"delhi", "in", "Delhi"}, {"德里", "in", "Delhi"},
	{"bangkok", "th", "Bangkok"}, {"曼谷", "th", "Bangkok"},
	{"ho chi minh", "vn", "Ho Chi Minh"}, {"hochiminh", "vn", "Ho Chi Minh"}, {"胡志明", "vn", "Ho Chi Minh"},
}

// Service depends on the DB, the runtime manager (for cache
// invalidation + Probe), and the metrics ring buffer.
type Service struct {
	db      *gorm.DB
	rt      *runtime.Manager
	metrics *MetricsStore
	log     *slog.Logger
}

// ListFilter is the optional backend filter for the admin node list.
type ListFilter struct {
	Query    string
	Area     string
	Province string
	Scheme   string
	Status   string
}

// New constructs the service.
func New(db *gorm.DB, rt *runtime.Manager, metrics *MetricsStore, lg *slog.Logger) *Service {
	return &Service{
		db:      db,
		rt:      rt,
		metrics: metrics,
		log:     lg.With(slog.String("component", "service.node")),
	}
}

// Input is the create-or-update payload, post-normalization. Empty
// APIToken on update means "keep the existing value".
type Input struct {
	Name     string `json:"name"`
	Area     string `json:"area"`
	Province string `json:"province"`
	Scheme   string `json:"scheme"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	BasePath string `json:"base_path"`
	APIToken string `json:"api_token"`
	Enabled  bool   `json:"enabled"`
}

// ---- CRUD ------------------------------------------------------------------

// Create persists a new node row. APIToken is required on create.
func (s *Service) Create(ctx context.Context, in Input) (*model.Node, error) {
	if err := s.normalize(&in); err != nil {
		return nil, err
	}
	if in.APIToken == "" {
		return nil, fmt.Errorf("%w: api_token is required", ErrInvalidInput)
	}

	row := &model.Node{
		Name:     in.Name,
		Area:     in.Area,
		Province: in.Province,
		Scheme:   in.Scheme,
		Host:     in.Host,
		Port:     in.Port,
		BasePath: in.BasePath,
		APIToken: in.APIToken,
		Enabled:  in.Enabled,
		Status:   model.NodeStatusUnknown,
	}
	// Select("*") so zero-value bool Enabled (from Input{Enabled:false})
	// lands as false; without it gorm lets the column default override.
	if err := s.db.WithContext(ctx).Select("*").Omit("ID", "CreatedAt", "UpdatedAt").Create(row).Error; err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateName, in.Name)
		}
		return nil, fmt.Errorf("node.Create: %w", err)
	}
	return row, nil
}

// Update applies the (already-normalized) input to an existing node.
// APIToken == "" means the token is left untouched. Always evicts
// the runtime cache so the next call rebuilds Remote.
func (s *Service) Update(ctx context.Context, id int64, in Input) (*model.Node, error) {
	if err := s.normalize(&in); err != nil {
		return nil, err
	}
	updates := map[string]any{
		"name":      in.Name,
		"area":      in.Area,
		"province":  in.Province,
		"scheme":    in.Scheme,
		"host":      in.Host,
		"port":      in.Port,
		"base_path": in.BasePath,
		"enabled":   in.Enabled,
	}
	if in.APIToken != "" {
		updates["api_token"] = in.APIToken
	}
	res := s.db.WithContext(ctx).Model(&model.Node{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		if isUniqueViolation(res.Error) {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateName, in.Name)
		}
		return nil, fmt.Errorf("node.Update: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	s.rt.InvalidateNode(id)
	return s.Get(ctx, id)
}

// Delete removes the node and any in-memory caches keyed off it.
func (s *Service) Delete(ctx context.Context, id int64) error {
	res := s.db.WithContext(ctx).Delete(&model.Node{}, id)
	if res.Error != nil {
		return fmt.Errorf("node.Delete: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	s.rt.InvalidateNode(id)
	s.metrics.Drop(id)
	return nil
}

// SetEnabled flips the enable bit and evicts the runtime cache. A
// disabled node stops being probed by the periodic job.
func (s *Service) SetEnabled(ctx context.Context, id int64, enabled bool) error {
	res := s.db.WithContext(ctx).Model(&model.Node{}).Where("id = ?", id).Update("enabled", enabled)
	if res.Error != nil {
		return fmt.Errorf("node.SetEnabled: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	s.rt.InvalidateNode(id)
	return nil
}

// Get returns one node by id. ErrNotFound on miss.
func (s *Service) Get(ctx context.Context, id int64) (*model.Node, error) {
	var n model.Node
	if err := s.db.WithContext(ctx).First(&n, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &n, nil
}

// List returns nodes matching the optional admin filters, ordered by id ascending.
func (s *Service) List(ctx context.Context, filter ListFilter) ([]model.Node, error) {
	var nodes []model.Node
	q := s.db.WithContext(ctx).Model(&model.Node{})
	filter.Area = strings.TrimSpace(filter.Area)
	if filter.Area != "" {
		filter.Area = normalizeAreaAlias(filter.Area)
		if filter.Area == "" {
			return nodes, nil
		}
		q = q.Where("area = ?", filter.Area)
	}
	filter.Province = strings.TrimSpace(filter.Province)
	if filter.Province != "" {
		if isUnknownProvince(filter.Province) {
			q = q.Where("province = ?", "unknown")
		} else {
			q = q.Where("LOWER(province) LIKE ?", "%"+strings.ToLower(filter.Province)+"%")
		}
	}
	filter.Scheme = strings.ToLower(strings.TrimSpace(filter.Scheme))
	if filter.Scheme != "" {
		if filter.Scheme != "http" && filter.Scheme != "https" {
			return nodes, nil
		}
		q = q.Where("scheme = ?", filter.Scheme)
	}
	filter.Status = strings.ToLower(strings.TrimSpace(filter.Status))
	if filter.Status != "" {
		switch filter.Status {
		case "online", "offline", "unknown":
			q = q.Where("enabled = TRUE AND status = ?", filter.Status)
		case "disabled":
			q = q.Where("enabled = FALSE")
		default:
			return nodes, nil
		}
	}
	filter.Query = strings.TrimSpace(filter.Query)
	if filter.Query != "" {
		like := "%" + strings.ToLower(filter.Query) + "%"
		area := normalizeAreaAlias(filter.Query)
		if area == "" {
			q = q.Where(
				"LOWER(name) LIKE ? OR LOWER(host) LIKE ? OR LOWER(area) LIKE ? OR LOWER(province) LIKE ? OR CAST(id AS TEXT) LIKE ?",
				like,
				like,
				like,
				like,
				like,
			)
		} else {
			q = q.Where(
				"LOWER(name) LIKE ? OR LOWER(host) LIKE ? OR area = ? OR LOWER(province) LIKE ? OR CAST(id AS TEXT) LIKE ?",
				like,
				like,
				area,
				like,
				like,
			)
		}
	}
	if err := q.Order("id ASC").Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

// MetricsRaw returns every metric sample for a node within [from,to].
func (s *Service) MetricsRaw(nodeID int64, from, to time.Time) []MetricSample {
	return s.metrics.Raw(nodeID, from, to)
}

// MetricsBucketed returns bucket-averaged metric samples.
func (s *Service) MetricsBucketed(nodeID int64, from, to time.Time, bucket time.Duration) []MetricSample {
	return s.metrics.Bucketed(nodeID, from, to, bucket)
}

// ListEnabled returns only the enabled nodes — used by the probe and
// traffic-collection jobs.
func (s *Service) ListEnabled(ctx context.Context) ([]model.Node, error) {
	var nodes []model.Node
	if err := s.db.WithContext(ctx).Where("enabled = TRUE").Order("id ASC").Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

// ---- Probe -----------------------------------------------------------------

// ProbeResult captures the outcome of a single Probe call so callers
// (notably the periodic job) can compare prior and new state.
type ProbeResult struct {
	NodeID      int64
	PriorStatus string
	Status      *runtime.Status
	Err         error
}

// Probe runs runtime.Probe and applies the heartbeat to the DB row.
// Returns the ProbeResult so the caller can emit transition events
// based on (PriorStatus, new Status).
func (s *Service) Probe(ctx context.Context, id int64) (*ProbeResult, error) {
	row, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	res := &ProbeResult{NodeID: id, PriorStatus: row.Status}

	r, err := s.rt.Get(ctx, id)
	if err != nil {
		res.Err = err
		_ = s.applyHeartbeatErr(ctx, id)
		return res, err
	}

	status, err := r.Probe(ctx)
	res.Status = status
	res.Err = err

	if err != nil {
		_ = s.applyHeartbeatErr(ctx, id)
		return res, err
	}
	if err := s.applyHeartbeatOK(ctx, id, status); err != nil {
		s.log.Warn("apply heartbeat failed",
			slog.Int64("node_id", id),
			slog.String("error", err.Error()),
		)
	}
	s.metrics.Append(id, time.Now().UTC(), status.CPU, status.MemPercent())
	return res, nil
}

func (s *Service) applyHeartbeatOK(ctx context.Context, id int64, status *runtime.Status) error {
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Model(&model.Node{}).Where("id = ?", id).Updates(map[string]any{
		"status":       model.NodeStatusOnline,
		"last_seen_at": now,
		"cpu_pct":      status.CPU,
		"mem_pct":      status.MemPercent(),
		"xray_version": status.Xray.Version,
		"uptime_s":     status.Uptime,
	}).Error
}

func (s *Service) applyHeartbeatErr(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Model(&model.Node{}).Where("id = ?", id).Update("status", model.NodeStatusOffline).Error
}

// ---- Validation / normalization -------------------------------------------

func (s *Service) normalize(in *Input) error { return Normalize(in) }

// Normalize mutates in to enforce field shape (trim, lowercase
// scheme, basePath leading+trailing slash) and validates the result.
// Returns ErrInvalidInput on any required-field failure.
func Normalize(in *Input) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Area = strings.TrimSpace(in.Area)
	in.Province = strings.TrimSpace(in.Province)
	in.Scheme = strings.ToLower(strings.TrimSpace(in.Scheme))
	in.Host = strings.TrimSpace(in.Host)
	in.BasePath = normalizeBasePath(strings.TrimSpace(in.BasePath))
	in.APIToken = strings.TrimSpace(in.APIToken)

	areaInput := normalizeAreaAlias(in.Area)
	in.Area = areaInput
	if in.Area == "" || in.Area == "unknown" {
		in.Area = detectArea(in.Name + " " + in.Host + " " + in.Province)
	}
	if in.Area == "" {
		in.Area = "unknown"
	}
	if in.Province == "" || isUnknownProvince(in.Province) {
		in.Province = detectProvince(in.Area, in.Name+" "+in.Host)
	}
	if in.Province == "" {
		in.Province = "unknown"
	}

	if in.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	if in.Scheme != "http" && in.Scheme != "https" {
		return fmt.Errorf("%w: scheme must be http or https", ErrInvalidInput)
	}
	if in.Host == "" {
		return fmt.Errorf("%w: host is required", ErrInvalidInput)
	}
	if in.Port < 1 || in.Port > 65535 {
		return fmt.Errorf("%w: port must be in 1..65535", ErrInvalidInput)
	}
	return nil
}

// normalizeBasePath returns p with a leading "/" and trailing "/".
// Empty input is normalized to the empty string (stored as-is so the
// runtime layer's normalizer can apply its own default). We could
// store "/" but it's redundant.
func normalizeBasePath(p string) string {
	if p == "" || p == "/" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if !strings.HasSuffix(p, "/") {
		p = p + "/"
	}
	return p
}

func normalizeAreaAlias(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return ""
	}
	if v == "unknown" || v == "未知" {
		return "unknown"
	}
	if area, ok := areaAliases[v]; ok {
		return area
	}
	return ""
}

func detectArea(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return ""
	}
	for _, hint := range areaDetectionHints {
		if textMatchesHint(text, hint.match) {
			return hint.area
		}
	}
	return ""
}

func detectProvince(area, text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" || area == "" || area == "unknown" {
		return ""
	}
	for _, hint := range provinceAliases {
		if hint.area == area && textMatchesHint(text, hint.match) {
			return hint.province
		}
	}
	return ""
}

func isUnknownProvince(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "unknown" || v == "未知"
}

func textMatchesHint(text, hint string) bool {
	hint = strings.ToLower(strings.TrimSpace(hint))
	if text == "" || hint == "" {
		return false
	}
	if hasNonASCII(hint) || strings.ContainsAny(hint, " -") {
		return strings.Contains(text, hint)
	}
	if len(hint) <= 3 {
		for _, token := range strings.FieldsFunc(text, func(r rune) bool {
			return r < '0' || (r > '9' && r < 'a') || r > 'z'
		}) {
			if token == hint {
				return true
			}
		}
		return false
	}
	return strings.Contains(text, hint)
}

func hasNonASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return true
		}
	}
	return false
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// pq / pgx surfaces unique violations under different sentinel
	// types; matching the SQLSTATE in the message is robust enough.
	return strings.Contains(err.Error(), "SQLSTATE 23505") ||
		strings.Contains(err.Error(), "duplicate key value")
}
