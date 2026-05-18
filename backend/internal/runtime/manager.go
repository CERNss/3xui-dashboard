package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/netsafe"
)

// ErrNodeNotFound is returned by Manager.Get / ForEach when the
// dashboard has no node row for the given id.
var ErrNodeNotFound = errors.New("runtime: node not found")

// ErrNodeDisabled is returned when the requested node is in the DB
// but its enabled flag is false. Callers can choose whether to skip
// it (traffic-collection job) or surface it (admin probe).
var ErrNodeDisabled = errors.New("runtime: node disabled")

// NodeLoader is the slice of repository access Manager needs. It is
// kept as a tiny interface so tests can inject fakes without pulling
// in the real *gorm.DB.
type NodeLoader interface {
	GetNode(ctx context.Context, id int64) (*model.Node, error)
	ListEnabledNodes(ctx context.Context) ([]model.Node, error)
}

// Manager owns one Remote per node id and recycles them when the
// underlying node row changes. Callers go through Manager rather
// than constructing Remote directly so cache invalidation stays in
// one place.
type Manager struct {
	loader NodeLoader
	http   *http.Client
	log    *slog.Logger

	mu     sync.Mutex
	cache  map[int64]*Remote
	loaded map[int64]model.Node // last-known node row, used for change detection
}

// NewManager constructs a Manager. The same *http.Client is shared
// across every Remote — its transport is built once with the
// SSRF-guarded dialer.
func NewManager(loader NodeLoader, lg *slog.Logger) *Manager {
	hc := &http.Client{
		Transport: netsafe.NewHTTPTransport(netsafe.DialerOptions{Timeout: 10 * time.Second}),
		Timeout:   30 * time.Second,
	}
	return &Manager{
		loader: loader,
		http:   hc,
		log:    lg.With(slog.String("component", "node-manager")),
		cache:  make(map[int64]*Remote),
		loaded: make(map[int64]model.Node),
	}
}

// Get returns a *Remote for node id, building one on first request.
// If the node row has changed since the cached Remote was built, the
// stale entry is evicted and rebuilt.
func (m *Manager) Get(ctx context.Context, id int64) (*Remote, error) {
	m.mu.Lock()
	if r, ok := m.cache[id]; ok {
		m.mu.Unlock()
		return r, nil
	}
	m.mu.Unlock()

	node, err := m.loader.GetNode(ctx, id)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, fmt.Errorf("%w: id=%d", ErrNodeNotFound, id)
	}
	if !node.Enabled {
		return nil, fmt.Errorf("%w: id=%d name=%q", ErrNodeDisabled, id, node.Name)
	}

	r := NewRemote(node, m.http, m.log)
	m.mu.Lock()
	m.cache[id] = r
	m.loaded[id] = *node
	m.mu.Unlock()
	return r, nil
}

// InvalidateNode evicts the cached Remote for node id. Callers should
// invoke this after any persisted change to the node row (host /
// port / api_token / base_path / scheme / enable). The next Get
// rebuilds.
func (m *Manager) InvalidateNode(id int64) {
	m.mu.Lock()
	delete(m.cache, id)
	delete(m.loaded, id)
	m.mu.Unlock()
}

// ForEach calls fn with a Remote for every enabled node, sequentially.
// Errors per-node are collected and returned as a multi-error so a
// single dead node doesn't abort the fleet walk.
func (m *Manager) ForEach(ctx context.Context, fn func(context.Context, *Remote) error) error {
	nodes, err := m.loader.ListEnabledNodes(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for i := range nodes {
		n := nodes[i]
		r, err := m.Get(ctx, n.ID)
		if err != nil {
			errs = append(errs, fmt.Errorf("node %d %q: %w", n.ID, n.Name, err))
			continue
		}
		if err := fn(ctx, r); err != nil {
			errs = append(errs, fmt.Errorf("node %d %q: %w", n.ID, n.Name, err))
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

// ---------------------------------------------------------------------------
// A minimal repository.NodeLoader implementation lives here as a
// convenience so cmd/dashboard doesn't have to wire it manually.
// Feature-specific node CRUD lives in internal/service/node.
// ---------------------------------------------------------------------------

// GormNodeLoader implements NodeLoader by reading from gorm.DB.
type GormNodeLoader struct{ DB *gorm.DB }

func (g *GormNodeLoader) GetNode(ctx context.Context, id int64) (*model.Node, error) {
	var n model.Node
	if err := g.DB.WithContext(ctx).First(&n, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &n, nil
}

func (g *GormNodeLoader) ListEnabledNodes(ctx context.Context) ([]model.Node, error) {
	var nodes []model.Node
	if err := g.DB.WithContext(ctx).Where("enabled = TRUE").Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}
