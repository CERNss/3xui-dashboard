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

// InboundRef identifies one inbound in the provenance ledger.
type InboundRef struct {
	NodeID int64
	Tag    string
}

// ClientState is the per-client annotation the fleet list carries:
// whether the dashboard created the client (managed) and which portal
// user it is bound to (nil for unbound). Merged server-side from the
// managed_clients ledger + client_ownerships.
type ClientState struct {
	NodeID      int64  `json:"node_id"`
	InboundTag  string `json:"inbound_tag"`
	ClientEmail string `json:"client_email"`
	Managed     bool   `json:"managed"`
	UserID      *int64 `json:"user_id,omitempty"`
}

// Ledger is the provenance store the service records inbound
// lifecycle events into and annotates fleet reads from. Nil-tolerant:
// an unset ledger disables recording and annotation (tests, minimal
// deployments). Mirrors the NodeListSource pattern so tests can
// inject fakes.
type Ledger interface {
	RecordInbound(ctx context.Context, nodeID int64, tag string) error
	ForgetInbound(ctx context.Context, nodeID int64, tag string) error
	RenameInboundTag(ctx context.Context, nodeID int64, oldTag, newTag string) error
	ListInboundRefs(ctx context.Context) ([]InboundRef, error)
	ListClientStates(ctx context.Context) ([]ClientState, error)
}

// Service composes the runtime manager and a node enumerator. Most
// methods are thin wrappers around runtime.Remote; the value-add is
// ListAll which fans out across the fleet.
type Service struct {
	rt     *runtime.Manager
	nodes  NodeListSource
	ledger Ledger
	log    *slog.Logger

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

// SetLedger attaches the provenance ledger. Optional — a nil ledger
// leaves creation unrecorded and fleet reads unannotated.
func (s *Service) SetLedger(l Ledger) { s.ledger = l }

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
	created, err := r.AddInbound(ctx, in)
	if err != nil {
		return nil, err
	}
	// Provenance: stamp the ledger so the admin views classify this
	// inbound as dashboard-created. Prefer the panel-echoed tag (the
	// panel auto-generates one when the request left it empty).
	// Recording failure is non-fatal — the panel write already
	// happened and can't be rolled back; warn and move on.
	if s.ledger != nil {
		tag := created.Tag
		if tag == "" {
			tag = in.Tag
		}
		if tag == "" {
			s.log.Warn("created inbound has no tag; skipping provenance record",
				slog.Int64("node_id", nodeID))
		} else if err := s.ledger.RecordInbound(ctx, nodeID, tag); err != nil {
			s.log.Warn("provenance record failed after inbound create",
				slog.Int64("node_id", nodeID),
				slog.String("tag", tag),
				slog.String("err", err.Error()),
			)
		}
	}
	return created, nil
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
	if err := resolveIntent(ctx, r, in); err != nil {
		return nil, err
	}
	updated, err := r.UpdateInbound(ctx, tag, in)
	if err != nil {
		return nil, err
	}
	// Tag rename: cascade every tag-keyed dashboard table (ledger,
	// ownerships, pool targets) or user subscriptions silently break.
	// This error is deliberately loud — the panel rename landed, and
	// the operator must know local state is out of step. Note the
	// admin UI always sends tag:"" on edit (never a rename); only API
	// callers can rename.
	if s.ledger != nil && in.Tag != "" && in.Tag != tag {
		if err := s.ledger.RenameInboundTag(ctx, nodeID, tag, in.Tag); err != nil {
			return updated, fmt.Errorf("inbound renamed on panel but local tag cascade failed: %w", err)
		}
	}
	return updated, nil
}

// Delete removes an inbound. Idempotent — missing tag is success.
// After the panel delete, every dashboard-side record keyed to the
// inbound (provenance ledger + client ownerships) is cleared: the
// panel destroys the inbound's clients with it, so those rows would
// be dead weight. A ledger failure surfaces as an error — retrying
// is safe (panel delete no-ops, forget re-runs).
func (s *Service) Delete(ctx context.Context, nodeID int64, tag string) error {
	r, err := s.rt.Get(ctx, nodeID)
	if err != nil {
		return err
	}
	if err := r.DeleteInbound(ctx, tag); err != nil {
		return err
	}
	if s.ledger != nil {
		if err := s.ledger.ForgetInbound(ctx, nodeID, tag); err != nil {
			return fmt.Errorf("inbound deleted on panel but local cleanup failed (retry the delete): %w", err)
		}
	}
	return nil
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
// admins can tell which node a row came from, plus whether this
// dashboard created it (provenance ledger hit).
type FleetInbound struct {
	NodeID   int64           `json:"node_id"`
	NodeName string          `json:"node_name"`
	Managed  bool            `json:"managed"`
	Inbound  runtime.Inbound `json:"inbound"`
}

// FleetResult is the typed shape of a fleet-wide list. NodeErrors
// keys an offline-or-misconfigured node id to a short string so the
// admin UI can render a per-node toast without losing healthy rows.
// ClientStates carries the per-client managed/user annotations the
// admin views join against (keyed by node+tag+email).
type FleetResult struct {
	Inbounds     []FleetInbound   `json:"inbounds"`
	ClientStates []ClientState    `json:"client_states,omitempty"`
	NodeErrors   map[int64]string `json:"node_errors,omitempty"`
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
	s.annotateFleet(ctx, out)
	return out, nil
}

// annotateFleet stamps Managed on each inbound and attaches the
// merged per-client states. Ledger read failures degrade to an
// unannotated result (warn only) — provenance display must never
// take down the fleet view.
func (s *Service) annotateFleet(ctx context.Context, out *FleetResult) {
	if s.ledger == nil || out == nil {
		return
	}
	refs, err := s.ledger.ListInboundRefs(ctx)
	if err != nil {
		s.log.Warn("fleet provenance annotation skipped", slog.String("err", err.Error()))
		return
	}
	managed := make(map[InboundRef]struct{}, len(refs))
	for _, ref := range refs {
		managed[ref] = struct{}{}
	}
	for i := range out.Inbounds {
		_, ok := managed[InboundRef{NodeID: out.Inbounds[i].NodeID, Tag: out.Inbounds[i].Inbound.Tag}]
		out.Inbounds[i].Managed = ok
	}
	states, err := s.ledger.ListClientStates(ctx)
	if err != nil {
		s.log.Warn("fleet client-state annotation skipped", slog.String("err", err.Error()))
		return
	}
	out.ClientStates = states
}
