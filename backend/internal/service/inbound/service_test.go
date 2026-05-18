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
