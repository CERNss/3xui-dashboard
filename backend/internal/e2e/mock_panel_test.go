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

	case req.URL.Path == "/panel/api/inbounds/onlines":
		writeEnv(w, []string{})

	case req.URL.Path == "/panel/api/inbounds/lastOnline":
		writeEnv(w, map[string]int64{})

	case req.URL.Path == "/panel/api/inbounds/addClient":
		m.handleAddClient(w, req)

	case strings.HasPrefix(req.URL.Path, "/panel/api/inbounds/updateClient/"):
		m.handleUpdateClient(w, req)

	case strings.Contains(req.URL.Path, "/delClientByEmail/"):
		m.handleDelClientByEmail(w, req)

	default:
		writeEnv(w, nil)
	}
}

// handleAddClient mirrors the fork's POST /panel/api/inbounds/addClient.
// Body shape (legacy, verified production fork 2026-05-21):
//
//	{"id": <inbound_id>, "settings": "<stringified-json {\"clients\":[{...}]}>"}
func (m *mockPanel) handleAddClient(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	var r struct {
		ID       int64  `json:"id"`
		Settings string `json:"settings"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	var s runtime.InboundSettings
	if err := json.Unmarshal([]byte(r.Settings), &s); err != nil {
		http.Error(w, "bad inner json", http.StatusBadRequest)
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, in := range m.inbounds {
		if in.ID != r.ID {
			continue
		}
		var existing runtime.InboundSettings
		_ = json.Unmarshal([]byte(in.Settings), &existing)
		existing.Clients = append(existing.Clients, s.Clients...)
		out, _ := json.Marshal(existing)
		in.Settings = string(out)
		break
	}
	writeEnv(w, nil)
}

// handleUpdateClient mirrors POST /panel/api/inbounds/updateClient/:clientId.
// Path param is the existing client's UUID/password/auth; body is
// the same {id, settings} envelope as addClient.
func (m *mockPanel) handleUpdateClient(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	var r struct {
		ID       int64  `json:"id"`
		Settings string `json:"settings"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		writeEnv(w, nil)
		return
	}
	var s runtime.InboundSettings
	if err := json.Unmarshal([]byte(r.Settings), &s); err != nil || len(s.Clients) == 0 {
		writeEnv(w, nil)
		return
	}
	newClient := s.Clients[0]
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, in := range m.inbounds {
		if in.ID != r.ID {
			continue
		}
		var existing runtime.InboundSettings
		_ = json.Unmarshal([]byte(in.Settings), &existing)
		for i := range existing.Clients {
			if existing.Clients[i].Email == newClient.Email {
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

// handleDelClientByEmail mirrors POST /panel/api/inbounds/:id/delClientByEmail/:email.
func (m *mockPanel) handleDelClientByEmail(w http.ResponseWriter, req *http.Request) {
	parts := strings.Split(req.URL.Path, "/")
	// "" "panel" "api" "inbounds" "<id>" "delClientByEmail" "<email>"
	if len(parts) < 7 {
		writeEnv(w, nil)
		return
	}
	email := parts[6]
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
