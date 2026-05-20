package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"

	"github.com/cern/3xui-dashboard/internal/runtime"
)

// mockPanel stands in for the 3x-ui /panel/api surface (MHSanaei/3x-ui
// fork route shape). It tracks /clients/{add,update,del} so the rest
// of the dashboard sees a consistent view across calls.
type mockPanel struct {
	server *httptest.Server

	mu       sync.Mutex
	inbounds map[string]*runtime.Inbound // keyed by tag
	idCount  int64
	calls    map[string]int
}

func newMockPanel() *mockPanel {
	mp := &mockPanel{
		inbounds: map[string]*runtime.Inbound{},
		calls:    map[string]int{},
	}
	mp.server = httptest.NewServer(http.HandlerFunc(mp.handle))
	return mp
}

func (m *mockPanel) Close() { m.server.Close() }
func (m *mockPanel) URL() string { return m.server.URL }

// SeedInbound registers an inbound the dashboard can read+mutate.
func (m *mockPanel) SeedInbound(in runtime.Inbound) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.idCount++
	in.ID = m.idCount
	m.inbounds[in.Tag] = &in
}

// Calls returns how many times path was hit.
func (m *mockPanel) Calls(path string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls[path]
}

// ClientsOn returns the parsed client list on the named inbound at
// the time of call. Used by the test to verify provisioning landed.
func (m *mockPanel) ClientsOn(tag string) []runtime.Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	in, ok := m.inbounds[tag]
	if !ok {
		return nil
	}
	var s runtime.InboundSettings
	_ = json.Unmarshal([]byte(in.Settings), &s)
	return append([]runtime.Client(nil), s.Clients...)
}

func (m *mockPanel) handle(w http.ResponseWriter, req *http.Request) {
	m.mu.Lock()
	m.calls[req.URL.Path]++
	m.mu.Unlock()

	if got := req.Header.Get("Authorization"); !strings.HasPrefix(got, "Bearer ") {
		http.Error(w, "missing bearer", http.StatusUnauthorized)
		return
	}

	switch {
	case req.URL.Path == "/panel/api/server/status":
		writeEnv(w, runtime.Status{
			CPU: 10.5, CPUCores: 4,
			Mem:    runtime.MemStat{Current: 1 << 30, Total: 4 << 30},
			Xray:   runtime.XrayStat{State: "running", Version: "25.1.0"},
			Uptime: 12345,
		})

	case req.URL.Path == "/panel/api/inbounds/list":
		m.mu.Lock()
		out := make([]runtime.Inbound, 0, len(m.inbounds))
		for _, in := range m.inbounds {
			out = append(out, *in)
		}
		m.mu.Unlock()
		writeEnv(w, out)

	case req.URL.Path == "/panel/api/clients/onlines":
		writeEnv(w, []string{})

	case req.URL.Path == "/panel/api/clients/lastOnline":
		writeEnv(w, map[string]int64{})

	case req.URL.Path == "/panel/api/clients/add":
		m.handleAddClient(w, req)

	case strings.HasPrefix(req.URL.Path, "/panel/api/clients/update/"):
		m.handleUpdateClient(w, req)

	case strings.HasPrefix(req.URL.Path, "/panel/api/clients/del/"):
		m.handleDelClientByEmail(w, req)

	case strings.HasPrefix(req.URL.Path, "/panel/api/clients/resetTraffic/"):
		writeEnv(w, nil)

	case strings.HasPrefix(req.URL.Path, "/panel/api/clients/traffic/"):
		writeEnv(w, nil)

	default:
		writeEnv(w, nil)
	}
}

// handleAddClient mirrors the fork's POST /panel/api/clients/add.
// Body shape: {client: model.Client, inboundIds: [int]}.
func (m *mockPanel) handleAddClient(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	var r struct {
		Client     runtime.Client `json:"client"`
		InboundIDs []int          `json:"inboundIds"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	if len(r.InboundIDs) == 0 {
		http.Error(w, "missing inboundIds", http.StatusBadRequest)
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, in := range m.inbounds {
		if in.ID != int64(r.InboundIDs[0]) {
			continue
		}
		var existing runtime.InboundSettings
		_ = json.Unmarshal([]byte(in.Settings), &existing)
		existing.Clients = append(existing.Clients, r.Client)
		out, _ := json.Marshal(existing)
		in.Settings = string(out)
		break
	}
	writeEnv(w, nil)
}

// handleUpdateClient mirrors the fork's POST /panel/api/clients/update/:email.
// Body is a raw runtime.Client; path captures the email key.
func (m *mockPanel) handleUpdateClient(w http.ResponseWriter, req *http.Request) {
	parts := strings.Split(req.URL.Path, "/")
	// "" "panel" "api" "clients" "update" "<email>"
	if len(parts) < 6 {
		writeEnv(w, nil)
		return
	}
	email := parts[5]
	body, _ := io.ReadAll(req.Body)
	var newClient runtime.Client
	if err := json.Unmarshal(body, &newClient); err != nil {
		writeEnv(w, nil)
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, in := range m.inbounds {
		var existing runtime.InboundSettings
		_ = json.Unmarshal([]byte(in.Settings), &existing)
		for i := range existing.Clients {
			if existing.Clients[i].Email == email {
				existing.Clients[i] = newClient
				out, _ := json.Marshal(existing)
				in.Settings = string(out)
				writeEnv(w, nil)
				return
			}
		}
	}
	writeEnv(w, nil)
}

// handleDelClientByEmail mirrors the fork's POST /panel/api/clients/del/:email.
func (m *mockPanel) handleDelClientByEmail(w http.ResponseWriter, req *http.Request) {
	parts := strings.Split(req.URL.Path, "/")
	// "" "panel" "api" "clients" "del" "<email>"
	if len(parts) < 6 {
		writeEnv(w, nil)
		return
	}
	email := parts[5]
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, in := range m.inbounds {
		var existing runtime.InboundSettings
		_ = json.Unmarshal([]byte(in.Settings), &existing)
		filtered := existing.Clients[:0]
		for _, c := range existing.Clients {
			if c.Email == email {
				continue
			}
			filtered = append(filtered, c)
		}
		existing.Clients = filtered
		out, _ := json.Marshal(existing)
		in.Settings = string(out)
	}
	writeEnv(w, nil)
}

func writeEnv(w http.ResponseWriter, obj any) {
	type env struct {
		Success bool        `json:"success"`
		Msg     string      `json:"msg"`
		Obj     interface{} `json:"obj"`
	}
	w.Header().Set("Content-Type", "application/json")
	body, _ := json.Marshal(env{Success: true, Msg: "ok", Obj: obj})
	_, _ = w.Write(body)
}
