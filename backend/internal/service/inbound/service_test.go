package inbound

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/runtime"
)

// fakeLoader implements runtime.NodeLoader from a static list.
type fakeLoader struct{ nodes []model.Node }

func (f *fakeLoader) GetNode(_ context.Context, id int64) (*model.Node, error) {
	for i := range f.nodes {
		if f.nodes[i].ID == id {
			return &f.nodes[i], nil
		}
	}
	return nil, nil
}
func (f *fakeLoader) ListEnabledNodes(_ context.Context) ([]model.Node, error) {
	out := []model.Node{}
	for _, n := range f.nodes {
		if n.Enabled {
			out = append(out, n)
		}
	}
	return out, nil
}

// fakeNodeRefs adapts a fakeLoader to inbound.NodeListSource.
type fakeNodeRefs struct{ loader *fakeLoader }

func (s *fakeNodeRefs) ListEnabledNodes(ctx context.Context) ([]NodeRef, error) {
	rows, err := s.loader.ListEnabledNodes(ctx)
	if err != nil {
		return nil, err
	}
	refs := make([]NodeRef, len(rows))
	for i, n := range rows {
		refs[i] = NodeRef{ID: n.ID, Name: n.Name}
	}
	return refs, nil
}

func nullLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

// fakeLedger records provenance calls and serves canned annotation
// data — the in-memory stand-in for repository.ProvenanceRepo.
type fakeLedger struct {
	recorded  []InboundRef
	forgotten []InboundRef
	renames   []struct {
		NodeID   int64
		Old, New string
	}
	refs   []InboundRef
	states []ClientState
}

func (f *fakeLedger) RecordInbound(_ context.Context, nodeID int64, tag string) error {
	f.recorded = append(f.recorded, InboundRef{NodeID: nodeID, Tag: tag})
	return nil
}

func (f *fakeLedger) ForgetInbound(_ context.Context, nodeID int64, tag string) error {
	f.forgotten = append(f.forgotten, InboundRef{NodeID: nodeID, Tag: tag})
	return nil
}

func (f *fakeLedger) RenameInboundTag(_ context.Context, nodeID int64, oldTag, newTag string) error {
	f.renames = append(f.renames, struct {
		NodeID   int64
		Old, New string
	}{nodeID, oldTag, newTag})
	return nil
}

func (f *fakeLedger) ListInboundRefs(_ context.Context) ([]InboundRef, error) {
	return f.refs, nil
}

func (f *fakeLedger) ListClientStates(_ context.Context) ([]ClientState, error) {
	return f.states, nil
}

// crudPanelServer serves list/add/update/del in the 3x-ui envelope.
// add echoes a panel-assigned tag when the request left it empty
// (mirrors the fork's auto-generated "inbound-<port>" behaviour).
func crudPanelServer(t *testing.T, inbounds []runtime.Inbound) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case req.URL.Path == "/panel/api/inbounds/list":
			body, _ := json.Marshal(map[string]any{"success": true, "obj": inbounds})
			_, _ = w.Write(body)
		case req.URL.Path == "/panel/api/inbounds/add":
			if err := req.ParseForm(); err != nil {
				t.Fatalf("parse add form: %v", err)
			}
			tag := req.FormValue("tag")
			if tag == "" {
				tag = "inbound-" + req.FormValue("port")
			}
			body, _ := json.Marshal(map[string]any{"success": true, "obj": runtime.Inbound{ID: 99, Tag: tag}})
			_, _ = w.Write(body)
		case len(req.URL.Path) > len("/panel/api/inbounds/update/") && req.URL.Path[:len("/panel/api/inbounds/update/")] == "/panel/api/inbounds/update/":
			if err := req.ParseForm(); err != nil {
				t.Fatalf("parse update form: %v", err)
			}
			body, _ := json.Marshal(map[string]any{"success": true, "obj": runtime.Inbound{ID: 7, Tag: req.FormValue("tag")}})
			_, _ = w.Write(body)
		case len(req.URL.Path) > len("/panel/api/inbounds/del/") && req.URL.Path[:len("/panel/api/inbounds/del/")] == "/panel/api/inbounds/del/":
			body, _ := json.Marshal(map[string]any{"success": true})
			_, _ = w.Write(body)
		default:
			t.Fatalf("unexpected path %s", req.URL.Path)
		}
	}))
}

// 3x-ui style server returning a fixed inbounds list.
func panelServer(t *testing.T, inbounds []runtime.Inbound) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/panel/api/inbounds/list" {
			t.Errorf("unexpected path %s", req.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		body, _ := json.Marshal(map[string]any{"success": true, "obj": inbounds})
		_, _ = w.Write(body)
	}))
}

// failingServer always 500s.
func failingServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
}

func updatingPanelServer(t *testing.T, current runtime.Inbound) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/panel/api/inbounds/list":
			body, _ := json.Marshal(map[string]any{"success": true, "obj": []runtime.Inbound{current}})
			_, _ = w.Write(body)
		case "/panel/api/server/getNewmldsa65":
			body, _ := json.Marshal(map[string]any{
				"success": true,
				"obj": map[string]string{
					"seed":   "UPDATED_MLDSA65_SEED",
					"verify": "UPDATED_MLDSA65_VERIFY",
				},
			})
			_, _ = w.Write(body)
		case "/panel/api/inbounds/update/7":
			if err := req.ParseForm(); err != nil {
				t.Fatalf("parse update form: %v", err)
			}
			current.StreamSettings = req.FormValue("streamSettings")
			body, _ := json.Marshal(map[string]any{"success": true, "obj": current})
			_, _ = w.Write(body)
		default:
			t.Fatalf("unexpected path %s", req.URL.Path)
		}
	}))
}

func nodeForURL(t *testing.T, id int64, name string, base string) model.Node {
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(u.Port())
	return model.Node{
		ID:       id,
		Name:     name,
		Scheme:   u.Scheme,
		Host:     u.Hostname(),
		Port:     port,
		APIToken: "tok",
		Enabled:  true,
	}
}

func TestListAll_FleetHappyPath(t *testing.T) {
	srvA := panelServer(t, []runtime.Inbound{{ID: 1, Tag: "a-vless", Port: 443}})
	defer srvA.Close()
	srvB := panelServer(t, []runtime.Inbound{{ID: 1, Tag: "b-trojan", Port: 444}})
	defer srvB.Close()

	loader := &fakeLoader{nodes: []model.Node{
		nodeForURL(t, 1, "alpha", srvA.URL),
		nodeForURL(t, 2, "beta", srvB.URL),
	}}
	mgr := runtime.NewManager(loader, nullLogger())
	// Replace the manager's SSRF-guarded transport with a plain
	// http.Client so 127.0.0.1 listeners are reachable in tests.
	mgr.SetHTTPClient(srvA.Client())
	svc := New(mgr, &fakeNodeRefs{loader: loader}, nullLogger())

	res, err := svc.ListAll(context.Background())
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(res.Inbounds) != 2 {
		t.Fatalf("inbounds = %d, want 2", len(res.Inbounds))
	}
	if len(res.NodeErrors) != 0 {
		t.Errorf("unexpected NodeErrors: %v", res.NodeErrors)
	}
}

func TestListAll_EmptyRemoteListSerializesAsArray(t *testing.T) {
	srv := panelServer(t, nil)
	defer srv.Close()

	loader := &fakeLoader{nodes: []model.Node{
		nodeForURL(t, 1, "empty", srv.URL),
	}}
	mgr := runtime.NewManager(loader, nullLogger())
	mgr.SetHTTPClient(srv.Client())
	svc := New(mgr, &fakeNodeRefs{loader: loader}, nullLogger())

	res, err := svc.ListAll(context.Background())
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if res.Inbounds == nil {
		t.Fatal("inbounds slice is nil, want empty non-nil slice for JSON []")
	}
	if len(res.Inbounds) != 0 {
		t.Fatalf("inbounds = %d, want 0", len(res.Inbounds))
	}
}

func TestListAll_PartialFailureSurfacesHealthyAndErrors(t *testing.T) {
	srvOK := panelServer(t, []runtime.Inbound{{ID: 1, Tag: "live", Port: 443}})
	defer srvOK.Close()
	srvBad := failingServer()
	defer srvBad.Close()

	loader := &fakeLoader{nodes: []model.Node{
		nodeForURL(t, 7, "good", srvOK.URL),
		nodeForURL(t, 9, "broken", srvBad.URL),
	}}
	mgr := runtime.NewManager(loader, nullLogger())
	mgr.SetHTTPClient(srvOK.Client())
	svc := New(mgr, &fakeNodeRefs{loader: loader}, nullLogger())

	res, err := svc.ListAll(context.Background())
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(res.Inbounds) != 1 || res.Inbounds[0].NodeName != "good" {
		t.Errorf("healthy result missing: %+v", res.Inbounds)
	}
	if msg, ok := res.NodeErrors[9]; !ok || msg == "" {
		t.Errorf("expected error for node 9, got %v", res.NodeErrors)
	}
}

func TestAdd_RecordsManagedInbound(t *testing.T) {
	srv := crudPanelServer(t, []runtime.Inbound{{ID: 1, Tag: "existing", Port: 443}})
	defer srv.Close()

	loader := &fakeLoader{nodes: []model.Node{nodeForURL(t, 3, "alpha", srv.URL)}}
	mgr := runtime.NewManager(loader, nullLogger())
	mgr.SetHTTPClient(srv.Client())
	svc := New(mgr, &fakeNodeRefs{loader: loader}, nullLogger())
	ledger := &fakeLedger{}
	svc.SetLedger(ledger)

	// Empty request tag: the panel-assigned tag must be recorded,
	// not the empty one.
	created, err := svc.Add(context.Background(), 3, &runtime.Inbound{Protocol: "vless", Port: 9443, Settings: `{"clients":[]}`})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if created.Tag != "inbound-9443" {
		t.Fatalf("created tag = %q, want panel-assigned inbound-9443", created.Tag)
	}
	if len(ledger.recorded) != 1 || ledger.recorded[0] != (InboundRef{NodeID: 3, Tag: "inbound-9443"}) {
		t.Errorf("recorded = %+v, want [{3 inbound-9443}]", ledger.recorded)
	}
}

func TestDelete_ForgetsManagedInbound(t *testing.T) {
	srv := crudPanelServer(t, []runtime.Inbound{{ID: 5, Tag: "doomed", Port: 443}})
	defer srv.Close()

	loader := &fakeLoader{nodes: []model.Node{nodeForURL(t, 3, "alpha", srv.URL)}}
	mgr := runtime.NewManager(loader, nullLogger())
	mgr.SetHTTPClient(srv.Client())
	svc := New(mgr, &fakeNodeRefs{loader: loader}, nullLogger())
	ledger := &fakeLedger{}
	svc.SetLedger(ledger)

	if err := svc.Delete(context.Background(), 3, "doomed"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(ledger.forgotten) != 1 || ledger.forgotten[0] != (InboundRef{NodeID: 3, Tag: "doomed"}) {
		t.Errorf("forgotten = %+v, want [{3 doomed}]", ledger.forgotten)
	}
}

func TestUpdate_TagRenameCascades(t *testing.T) {
	srv := crudPanelServer(t, []runtime.Inbound{{ID: 7, Tag: "old-tag", Port: 443}})
	defer srv.Close()

	loader := &fakeLoader{nodes: []model.Node{nodeForURL(t, 3, "alpha", srv.URL)}}
	mgr := runtime.NewManager(loader, nullLogger())
	mgr.SetHTTPClient(srv.Client())
	svc := New(mgr, &fakeNodeRefs{loader: loader}, nullLogger())
	ledger := &fakeLedger{}
	svc.SetLedger(ledger)

	if _, err := svc.Update(context.Background(), 3, "old-tag", &runtime.Inbound{Tag: "new-tag", Protocol: "vless", Port: 443}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(ledger.renames) != 1 || ledger.renames[0].Old != "old-tag" || ledger.renames[0].New != "new-tag" || ledger.renames[0].NodeID != 3 {
		t.Fatalf("renames = %+v, want [{3 old-tag new-tag}]", ledger.renames)
	}

	// The admin UI always submits tag:"" on edit — that must never be
	// treated as a rename to the empty string.
	if _, err := svc.Update(context.Background(), 3, "old-tag", &runtime.Inbound{Tag: "", Protocol: "vless", Port: 443}); err != nil {
		t.Fatalf("Update with empty tag: %v", err)
	}
	if len(ledger.renames) != 1 {
		t.Errorf("empty body tag triggered a rename: %+v", ledger.renames)
	}
}

func TestListAll_AnnotatesManagedAndClientStates(t *testing.T) {
	srv := panelServer(t, []runtime.Inbound{
		{ID: 1, Tag: "ours", Port: 443},
		{ID: 2, Tag: "theirs", Port: 444},
	})
	defer srv.Close()

	loader := &fakeLoader{nodes: []model.Node{nodeForURL(t, 4, "alpha", srv.URL)}}
	mgr := runtime.NewManager(loader, nullLogger())
	mgr.SetHTTPClient(srv.Client())
	svc := New(mgr, &fakeNodeRefs{loader: loader}, nullLogger())
	uid := int64(11)
	svc.SetLedger(&fakeLedger{
		refs: []InboundRef{{NodeID: 4, Tag: "ours"}},
		states: []ClientState{
			{NodeID: 4, InboundTag: "ours", ClientEmail: "abc123", Managed: true, UserID: &uid},
		},
	})

	res, err := svc.ListAll(context.Background())
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	managedByTag := map[string]bool{}
	for _, row := range res.Inbounds {
		managedByTag[row.Inbound.Tag] = row.Managed
	}
	if !managedByTag["ours"] || managedByTag["theirs"] {
		t.Errorf("managed flags wrong: %+v", managedByTag)
	}
	if len(res.ClientStates) != 1 || !res.ClientStates[0].Managed || res.ClientStates[0].UserID == nil || *res.ClientStates[0].UserID != 11 {
		t.Errorf("client states = %+v", res.ClientStates)
	}
}

func TestListAll_NilLedgerLeavesUnannotated(t *testing.T) {
	srv := panelServer(t, []runtime.Inbound{{ID: 1, Tag: "raw", Port: 443}})
	defer srv.Close()

	loader := &fakeLoader{nodes: []model.Node{nodeForURL(t, 1, "alpha", srv.URL)}}
	mgr := runtime.NewManager(loader, nullLogger())
	mgr.SetHTTPClient(srv.Client())
	svc := New(mgr, &fakeNodeRefs{loader: loader}, nullLogger())

	res, err := svc.ListAll(context.Background())
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if res.Inbounds[0].Managed {
		t.Error("nil ledger must leave Managed=false")
	}
	if res.ClientStates != nil {
		t.Errorf("nil ledger must leave ClientStates nil, got %+v", res.ClientStates)
	}
}

func TestUpdate_ResolvesIntentBeforePanelUpdate(t *testing.T) {
	current := runtime.Inbound{
		ID:             7,
		Tag:            "reality-443",
		Protocol:       "vless",
		Port:           443,
		Settings:       `{"clients":[]}`,
		StreamSettings: `{"security":"reality","realitySettings":{}}`,
	}
	srv := updatingPanelServer(t, current)
	defer srv.Close()

	loader := &fakeLoader{nodes: []model.Node{nodeForURL(t, 1, "reality", srv.URL)}}
	mgr := runtime.NewManager(loader, nullLogger())
	mgr.SetHTTPClient(srv.Client())
	svc := New(mgr, &fakeNodeRefs{loader: loader}, nullLogger())

	updated, err := svc.Update(context.Background(), 1, "reality-443", &runtime.Inbound{
		Tag:            "reality-443",
		Protocol:       "vless",
		Port:           443,
		Settings:       `{"clients":[]}`,
		StreamSettings: `{"security":"reality","realitySettings":{},"_intent":{"realityMldsa65":true}}`,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated == nil {
		t.Fatal("updated inbound is nil")
	}
	if contains := json.Valid([]byte(updated.StreamSettings)); !contains {
		t.Fatalf("updated streamSettings is invalid JSON: %s", updated.StreamSettings)
	}
	var stream map[string]any
	_ = json.Unmarshal([]byte(updated.StreamSettings), &stream)
	reality, _ := stream["realitySettings"].(map[string]any)
	settings, _ := reality["settings"].(map[string]any)
	if reality["mldsa65Seed"] != "UPDATED_MLDSA65_SEED" || settings["mldsa65Verify"] != "UPDATED_MLDSA65_VERIFY" {
		t.Fatalf("ML-DSA-65 not resolved before update: %+v", reality)
	}
	if _, ok := stream["_intent"]; ok {
		t.Fatalf("_intent leaked to panel update: %s", updated.StreamSettings)
	}
}
