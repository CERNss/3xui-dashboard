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
	ErrInvalidInput     = errors.New("node: invalid input")
	ErrNotFound         = errors.New("node: not found")
	ErrDuplicateName    = errors.New("node: name already exists")
)

// Service depends on the DB, the runtime manager (for cache
// invalidation + Probe), and the metrics ring buffer.
type Service struct {
	db      *gorm.DB
	rt      *runtime.Manager
	metrics *MetricsStore
	log     *slog.Logger
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
		Scheme:   in.Scheme,
		Host:     in.Host,
		Port:     in.Port,
		BasePath: in.BasePath,
		APIToken: in.APIToken,
		Enabled:  in.Enabled,
		Status:   model.NodeStatusUnknown,
	}
	if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
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

// List returns every node, ordered by id ascending.
func (s *Service) List(ctx context.Context) ([]model.Node, error) {
	var nodes []model.Node
	if err := s.db.WithContext(ctx).Order("id ASC").Find(&nodes).Error; err != nil {
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
	NodeID       int64
	PriorStatus  string
	Status       *runtime.Status
	Err          error
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
		"status":        model.NodeStatusOnline,
		"last_seen_at":  now,
		"cpu_pct":       status.CPU,
		"mem_pct":       status.MemPercent(),
		"xray_version":  status.Xray.Version,
		"uptime_s":      status.Uptime,
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
	in.Scheme = strings.ToLower(strings.TrimSpace(in.Scheme))
	in.Host = strings.TrimSpace(in.Host)
	in.BasePath = normalizeBasePath(strings.TrimSpace(in.BasePath))
	in.APIToken = strings.TrimSpace(in.APIToken)

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

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// pq / pgx surfaces unique violations under different sentinel
	// types; matching the SQLSTATE in the message is robust enough.
	return strings.Contains(err.Error(), "SQLSTATE 23505") ||
		strings.Contains(err.Error(), "duplicate key value")
}
