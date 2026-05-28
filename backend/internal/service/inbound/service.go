// Package inbound is the service layer wrapping runtime.Manager for
// inbound-level operations: per-node CRUD + fleet-wide list with
// per-node error collection.
package inbound

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/runtime"
	"github.com/cern/3xui-dashboard/internal/service/wgcrypto"
)

// NodeListSource is the subset of repository / service.node access
// the inbound service needs: a way to enumerate enabled nodes for
// fleet walks. Kept tiny so tests can inject fakes.
type NodeListSource interface {
	ListEnabledNodes(ctx context.Context) ([]NodeRef, error)
}

// NodeRef is the minimal node identification fleet-wide ops need.
// Defined here (instead of importing model.Node) so the inbound
// package doesn't grow heavy dependencies.
type NodeRef struct {
	ID   int64
	Name string
}

// Service composes the runtime manager and a node enumerator. Most
// methods are thin wrappers around runtime.Remote; the value-add is
// ListAll which fans out across the fleet.
type Service struct {
	rt    *runtime.Manager
	nodes NodeListSource
	log   *slog.Logger

	// FleetConcurrency caps parallel node calls during ListAll. Zero
	// uses a sensible default (8).
	FleetConcurrency int
}

// New constructs the service.
func New(rt *runtime.Manager, nodes NodeListSource, lg *slog.Logger) *Service {
	return &Service{
		rt:    rt,
		nodes: nodes,
		log:   lg.With(slog.String("component", "service.inbound")),
	}
}

// ---- Per-node ops ---------------------------------------------------------

// List returns every inbound on one node.
func (s *Service) List(ctx context.Context, nodeID int64) ([]runtime.Inbound, error) {
	r, err := s.rt.Get(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	return r.ListInbounds(ctx)
}

// Get returns one inbound by tag.
func (s *Service) Get(ctx context.Context, nodeID int64, tag string) (*runtime.Inbound, error) {
	r, err := s.rt.Get(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	return r.GetInbound(ctx, tag)
}

// Add creates an inbound on a node. The returned inbound carries the
// panel-assigned id.
//
// WireGuard inbounds get a server keypair filled in here when the
// caller left `secretKey` empty: the production fork (T1 verified
// 2026-05-21) stores empty strings verbatim and the inbound stays
// non-functional until the first peer provision lazily fills it.
// Filling at create-time means an admin who creates a WG inbound
// from the UI gets a working tunnel without having to provision
// at least one peer first.
func (s *Service) Add(ctx context.Context, nodeID int64, in *runtime.Inbound) (*runtime.Inbound, error) {
	if in != nil && in.IsWireguard() {
		if err := ensureWGSecretKey(in); err != nil {
			return nil, err
		}
	}
	r, err := s.rt.Get(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	// Intent resolution: replace any `_intent` markers (added by the
	// dashboard template editor) with concrete keys / certs / random
	// short IDs sourced from the target node's panel. The wire payload
	// xray-core ends up seeing has no dashboard-only fields.
	if err := resolveIntent(ctx, r, in); err != nil {
		return nil, err
	}
	return r.AddInbound(ctx, in)
}

// BuildTemplateInbound materializes an inbound template into the
// runtime wire shape expected by 3x-ui. Port and tag are supplied by
// the provisioning pool allocator.
func BuildTemplateInbound(t *model.InboundTemplate, port int, tag string) *runtime.Inbound {
	if t == nil {
		return nil
	}
	remark := strings.TrimSpace(t.Remark)
	if remark == "" {
		remark = strings.TrimSpace(t.Name)
	}
	return &runtime.Inbound{
		Total:          t.Total,
		Remark:         remark,
		Enable:         true,
		ExpiryTime:     t.ExpiryTime,
		TrafficReset:   t.TrafficReset,
		Listen:         t.Listen,
		Port:           port,
		Protocol:       t.Protocol,
		Settings:       t.Settings,
		StreamSettings: t.StreamSettings,
		Tag:            tag,
		Sniffing:       t.Sniffing,
	}
}

// ensureWGSecretKey mutates in.Settings in place when the inbound
// is a WG one whose secretKey is empty. Idempotent: a non-empty
// secretKey passes through unchanged. MTU=0 is also stamped to
// the WireGuard-recommended 1420 (avoids overlay fragmentation).
func ensureWGSecretKey(in *runtime.Inbound) error {
	var s runtime.WGSettings
	if in.Settings != "" {
		if err := json.Unmarshal([]byte(in.Settings), &s); err != nil {
			return fmt.Errorf("decode WG settings: %w", err)
		}
	}
	dirty := false
	if s.SecretKey == "" {
		kp, err := wgcrypto.GenerateKeypair()
		if err != nil {
			return fmt.Errorf("generate WG server keypair: %w", err)
		}
		s.SecretKey = kp.Private
		dirty = true
	}
	if s.MTU == 0 {
		s.MTU = 1420
		dirty = true
	}
	if !dirty {
		return nil
	}
	out, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("re-marshal WG settings: %w", err)
	}
	in.Settings = string(out)
	return nil
}

// Update mutates an existing inbound.
func (s *Service) Update(ctx context.Context, nodeID int64, tag string, in *runtime.Inbound) (*runtime.Inbound, error) {
	r, err := s.rt.Get(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	return r.UpdateInbound(ctx, tag, in)
}

// Delete removes an inbound. Idempotent — missing tag is success.
func (s *Service) Delete(ctx context.Context, nodeID int64, tag string) error {
	r, err := s.rt.Get(ctx, nodeID)
	if err != nil {
		return err
	}
	return r.DeleteInbound(ctx, tag)
}

// SetEnable flips just the enable bit.
func (s *Service) SetEnable(ctx context.Context, nodeID int64, tag string, enable bool) error {
	r, err := s.rt.Get(ctx, nodeID)
	if err != nil {
		return err
	}
	return r.SetInboundEnable(ctx, tag, enable)
}

// ---- Fleet-wide aggregation -----------------------------------------------

// FleetInbound annotates each inbound with the node it lives on so
// admins can tell which node a row came from.
type FleetInbound struct {
	NodeID   int64           `json:"node_id"`
	NodeName string          `json:"node_name"`
	Inbound  runtime.Inbound `json:"inbound"`
}

// FleetResult is the typed shape of a fleet-wide list. NodeErrors
// keys an offline-or-misconfigured node id to a short string so the
// admin UI can render a per-node toast without losing healthy rows.
type FleetResult struct {
	Inbounds   []FleetInbound   `json:"inbounds"`
	NodeErrors map[int64]string `json:"node_errors,omitempty"`
}

// ListAll walks every enabled node concurrently (capped) and returns
// every inbound + a per-node error map. A single node failure never
// aborts the walk.
func (s *Service) ListAll(ctx context.Context) (*FleetResult, error) {
	nodes, err := s.nodes.ListEnabledNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("inbound.ListAll: %w", err)
	}
	if len(nodes) == 0 {
		return &FleetResult{}, nil
	}

	conc := s.FleetConcurrency
	if conc <= 0 {
		conc = 8
	}

	var (
		mu       sync.Mutex
		results  = make([]FleetInbound, 0)
		errsByID = map[int64]string{}
	)

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(conc)

	for i := range nodes {
		n := nodes[i]
		g.Go(func() error {
			r, err := s.rt.Get(gctx, n.ID)
			if err != nil {
				mu.Lock()
				errsByID[n.ID] = err.Error()
				mu.Unlock()
				return nil
			}
			inbounds, err := r.ListInbounds(gctx)
			if err != nil {
				mu.Lock()
				errsByID[n.ID] = err.Error()
				mu.Unlock()
				return nil
			}
			collected := make([]FleetInbound, 0, len(inbounds))
			for _, in := range inbounds {
				collected = append(collected, FleetInbound{
					NodeID:   n.ID,
					NodeName: n.Name,
					Inbound:  in,
				})
			}
			mu.Lock()
			results = append(results, collected...)
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()

	out := &FleetResult{Inbounds: results}
	if len(errsByID) > 0 {
		out.NodeErrors = errsByID
	}
	return out, nil
}
