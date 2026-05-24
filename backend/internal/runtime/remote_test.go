package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/cern/3xui-dashboard/internal/model"
)

// ---- test helpers ----------------------------------------------------------

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

// newTestRemote spins up an httptest.Server, builds a Remote pointed
// at it, and returns both so individual tests can assert on the
// captured request log.
func newTestRemote(t *testing.T, handler http.Handler) (*Remote, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse srv.URL: %v", err)
	}
	port, _ := strconv.Atoi(u.Port())

	node := &model.Node{
		ID:       42,
		Name:     "test-node",
		Scheme:   u.Scheme,
		Host:     u.Hostname(),
		Port:     port,
		BasePath: "",
		APIToken: "secret-token",
		Enabled:  true,
	}
	// Plain http.Client (no SSRF guard) for unit tests so httptest's
	// 127.0.0.1 listener is reachable.
	r := NewRemote(node, srv.Client(), testLogger())
	return r, srv
}

// okEnvelope wraps obj in the 3x-ui success envelope and writes it.
func okEnvelope(w http.ResponseWriter, obj any) {
	w.Header().Set("Content-Type", "application/json")
	body, _ := json.Marshal(Envelope{Success: true, Msg: "ok", Obj: rawJSON(obj)})
	_, _ = w.Write(body)
}

func errEnvelope(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	body, _ := json.Marshal(Envelope{Success: false, Msg: msg})
	_, _ = w.Write(body)
}

func rawJSON(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// ---- envelope decode -------------------------------------------------------

func TestEnvelopeDecode_Success(t *testing.T) {
	r, _ := newTestRemote(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/panel/api/server/status" || req.Method != http.MethodGet {
			t.Errorf("unexpected request %s %s", req.Method, req.URL.Path)
		}
		if got := req.Header.Get("Authorization"); got != "Bearer secret-token" {
			t.Errorf("Authorization header = %q, want Bearer secret-token", got)
		}
		okEnvelope(w, Status{
			CPU:    12.5,
			Mem:    MemStat{Current: 800, Total: 4000},
			Xray:   XrayStat{State: "running", Version: "25.0.0"},
			Uptime: 999,
		})
	}))
	status, err := r.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if status.CPU != 12.5 || status.Mem.Total != 4000 || status.Xray.Version != "25.0.0" {
		t.Errorf("decoded status mismatch: %+v", status)
	}
	if got := status.MemPercent(); got < 19 || got > 21 {
		t.Errorf("MemPercent = %v, want ~20", got)
	}
}

func TestEnvelopeDecode_PanelError(t *testing.T) {
	r, _ := newTestRemote(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		errEnvelope(w, "bad request")
	}))
	_, err := r.Probe(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var ev *EnvelopeError
	if !errors.As(err, &ev) {
		t.Fatalf("error is not *EnvelopeError: %v", err)
	}
	if ev.Msg != "bad request" {
		t.Errorf("envelope msg = %q, want bad request", ev.Msg)
	}
}

func TestUnauthorizedSurfacedClearly(t *testing.T) {
	r, _ := newTestRemote(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	_, err := r.Probe(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("want unauthorized error, got %v", err)
	}
}

// ---- tag → id cache --------------------------------------------------------

func TestTagCache_PopulatesFromList(t *testing.T) {
	var listCalls atomic.Int32
	r, _ := newTestRemote(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/panel/api/inbounds/list":
			listCalls.Add(1)
			okEnvelope(w, []Inbound{
				{ID: 1, Tag: "vless-1", Port: 443},
				{ID: 2, Tag: "trojan-1", Port: 444},
			})
		case "/panel/api/inbounds/setEnable/1":
			okEnvelope(w, nil)
		default:
			t.Errorf("unexpected path %s", req.URL.Path)
		}
	}))

	// First call: cache empty → triggers /list.
	if err := r.SetInboundEnable(context.Background(), "vless-1", true); err != nil {
		t.Fatalf("SetInboundEnable: %v", err)
	}
	if got := listCalls.Load(); got != 1 {
		t.Errorf("list calls = %d, want 1 after first miss", got)
	}

	// Second call: cache populated → no /list.
	if err := r.SetInboundEnable(context.Background(), "vless-1", false); err != nil {
		t.Fatalf("SetInboundEnable #2: %v", err)
	}
	if got := listCalls.Load(); got != 1 {
		t.Errorf("list calls = %d, want still 1 after cache hit", got)
	}
}

func TestTagCache_RefreshOnMiss(t *testing.T) {
	var listCalls atomic.Int32
	r, _ := newTestRemote(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/panel/api/inbounds/list":
			n := listCalls.Add(1)
			if n == 1 {
				okEnvelope(w, []Inbound{{ID: 1, Tag: "old"}})
			} else {
				okEnvelope(w, []Inbound{{ID: 1, Tag: "old"}, {ID: 9, Tag: "fresh"}})
			}
		case "/panel/api/inbounds/setEnable/9":
			okEnvelope(w, nil)
		default:
			t.Errorf("unexpected path %s", req.URL.Path)
		}
	}))

	// Seed the cache.
	if _, err := r.ListInbounds(context.Background()); err != nil {
		t.Fatalf("ListInbounds seed: %v", err)
	}
	// Ask for a tag not present → forces refresh.
	if err := r.SetInboundEnable(context.Background(), "fresh", true); err != nil {
		t.Fatalf("SetInboundEnable fresh: %v", err)
	}
	if got := listCalls.Load(); got != 2 {
		t.Errorf("list calls = %d, want 2 (seed + miss-refresh)", got)
	}
}

// ---- idempotent delete -----------------------------------------------------

func TestDeleteInbound_MissingTagIsNoop(t *testing.T) {
	var calls atomic.Int32
	r, _ := newTestRemote(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls.Add(1)
		switch req.URL.Path {
		case "/panel/api/inbounds/list":
			okEnvelope(w, []Inbound{{ID: 1, Tag: "exists"}})
		default:
			t.Errorf("unexpected path %s (delete should not hit network for missing tag)", req.URL.Path)
		}
	}))

	if err := r.DeleteInbound(context.Background(), "does-not-exist"); err != nil {
		t.Fatalf("DeleteInbound missing tag returned error: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("calls = %d, want 1 (only the /list refresh)", got)
	}
}

func TestDeleteClientByEmail_MissingTagIsNoop(t *testing.T) {
	r, _ := newTestRemote(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/panel/api/inbounds/list" {
			okEnvelope(w, []Inbound{})
			return
		}
		t.Errorf("unexpected path %s", req.URL.Path)
	}))
	if err := r.DeleteClientByEmail(context.Background(), "missing-tag", "user@example.com"); err != nil {
		t.Fatalf("DeleteClientByEmail returned error for missing tag: %v", err)
	}
}

// TestAddClient_UsesInboundsAddClient asserts AddClient hits the
// real fork route /panel/api/inbounds/addClient with body shape
// {id, settings: stringified-json}. This was wrongly migrated to
// /panel/api/clients/add in commit d2598ec (#11) based on outdated
// MHSanaei/3x-ui source reading; the production fork (verified
// against node-1 on 2026-05-21) has the /clients/* group absent
// and the /inbounds/* routes active. Don't migrate again
// without running the T1 probe on the actual target fork first.
func TestAddClient_UsesInboundsAddClient(t *testing.T) {
	var seenPath string
	var seenBody []byte
	r, _ := newTestRemote(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/panel/api/inbounds/list":
			okEnvelope(w, []Inbound{{ID: 7, Tag: "vless-in"}})
		case "/panel/api/inbounds/addClient":
			seenPath = req.URL.Path
			seenBody, _ = io.ReadAll(req.Body)
			okEnvelope(w, nil)
		default:
			t.Errorf("unexpected path %s", req.URL.Path)
		}
	}))

	if err := r.AddClient(context.Background(), "vless-in", Client{Email: "alice@example.com", ID: "uuid-1"}); err != nil {
		t.Fatalf("AddClient: %v", err)
	}
	if seenPath != "/panel/api/inbounds/addClient" {
		t.Errorf("AddClient hit %q, want /panel/api/inbounds/addClient", seenPath)
	}

	var got struct {
		ID       int64  `json:"id"`
		Settings string `json:"settings"`
	}
	if err := json.Unmarshal(seenBody, &got); err != nil {
		t.Fatalf("decode body: %v (body=%s)", err, seenBody)
	}
	if got.ID != 7 {
		t.Errorf("id = %d, want 7 (resolved from tag)", got.ID)
	}
	if !strings.Contains(got.Settings, `"alice@example.com"`) || !strings.Contains(got.Settings, `"uuid-1"`) {
		t.Errorf("settings JSON doesn't carry client fields: %q", got.Settings)
	}
}

// TestUpdateInboundByID_PostsFormToUpdatePath asserts the new
// id-keyed variant skips the tag→id lookup and posts a form-encoded
// inbound to /inbounds/update/:id. This is the RMW happy path for
// WireGuard peer mutation.
func TestUpdateInboundByID_PostsFormToUpdatePath(t *testing.T) {
	var capturedPath, capturedSettings, capturedProtocol string
	r, _ := newTestRemote(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/panel/api/inbounds/update/7" {
			t.Errorf("unexpected path %s", req.URL.Path)
			return
		}
		capturedPath = req.URL.Path
		if got := req.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/x-www-form-urlencoded") {
			t.Errorf("Content-Type = %q, want form-urlencoded", got)
		}
		body, _ := io.ReadAll(req.Body)
		vals, _ := url.ParseQuery(string(body))
		capturedSettings = vals.Get("settings")
		capturedProtocol = vals.Get("protocol")
		okEnvelope(w, Inbound{ID: 7, Tag: "wg-1", Protocol: "wireguard"})
	}))

	in := &Inbound{
		ID:       7,
		Tag:      "wg-1",
		Protocol: "wireguard",
		Port:     51820,
		Settings: `{"mtu":1420,"peers":[{"publicKey":"AAA"}]}`,
	}
	updated, err := r.UpdateInboundByID(context.Background(), 7, in)
	if err != nil {
		t.Fatalf("UpdateInboundByID: %v", err)
	}
	if capturedPath != "/panel/api/inbounds/update/7" {
		t.Errorf("hit %q, want /panel/api/inbounds/update/7", capturedPath)
	}
	if capturedProtocol != "wireguard" {
		t.Errorf("protocol form field = %q, want wireguard", capturedProtocol)
	}
	if !strings.Contains(capturedSettings, `"peers"`) {
		t.Errorf("settings form field missing peers array: %q", capturedSettings)
	}
	if updated.Tag != "wg-1" {
		t.Errorf("response decode = %+v, want tag wg-1", updated)
	}
}

// TestInbound_IsWireguard guards against accidental case-changes
// to the fork's protocol string.
func TestInbound_IsWireguard(t *testing.T) {
	if (&Inbound{Protocol: "wireguard"}).IsWireguard() != true {
		t.Error("wireguard inbound not detected")
	}
	if (&Inbound{Protocol: "vless"}).IsWireguard() != false {
		t.Error("vless inbound flagged as wireguard")
	}
	// The fork emits lowercase only — uppercase MUST NOT match,
	// otherwise mixed-case drift goes unnoticed.
	if (&Inbound{Protocol: "WireGuard"}).IsWireguard() != false {
		t.Error("case-folded match would mask fork-protocol drift")
	}
}

// TestAddClient_404SurfacesPath asserts that when the panel route
// is missing, the returned error names the actual path that 404'd
// rather than silently falling back to a write that pretends
// success — operators need fork-version drift visible.
func TestAddClient_404SurfacesPath(t *testing.T) {
	r, _ := newTestRemote(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/panel/api/inbounds/list":
			okEnvelope(w, []Inbound{{ID: 3, Tag: "vless-in"}})
		case "/panel/api/inbounds/addClient":
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected path %s", req.URL.Path)
		}
	}))

	err := r.AddClient(context.Background(), "vless-in", Client{Email: "bob@example.com"})
	if err == nil {
		t.Fatal("expected 404 to surface as error, got nil")
	}
	if !strings.Contains(err.Error(), "/panel/api/inbounds/addClient") {
		t.Errorf("error %q does not name the 404'd path /panel/api/inbounds/addClient", err.Error())
	}
}

// ---- base path normalization -----------------------------------------------

func TestNormalizeBasePath(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "/"},
		{"/", "/"},
		{"admin", "/admin/"},
		{"/admin", "/admin/"},
		{"admin/", "/admin/"},
		{"/admin/", "/admin/"},
		{" /a/ ", "/a/"},
	}
	for _, c := range cases {
		if got := normalizeBasePath(c.in); got != c.want {
			t.Errorf("normalizeBasePath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBuildBaseURL(t *testing.T) {
	n := &model.Node{Scheme: "https", Host: "node.example.com", Port: 2053, BasePath: "panel"}
	got := buildBaseURL(n)
	want := "https://node.example.com:2053/panel/"
	if got != want {
		t.Errorf("buildBaseURL = %q, want %q", got, want)
	}
}

// ---- sanitize stream settings ----------------------------------------------

func TestSanitizeStreamSettings_StripsCertPathsWhenInlinePresent(t *testing.T) {
	stream := `{
        "network": "tcp",
        "security": "tls",
        "tlsSettings": {
            "certificates": [
                {
                    "certificate": ["-----BEGIN CERT-----"],
                    "key": ["-----BEGIN KEY-----"],
                    "certificateFile": "/etc/ssl/cert.pem",
                    "keyFile": "/etc/ssl/key.pem"
                }
            ]
        }
    }`
	out := sanitizeStreamSettingsForRemote(stream)

	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("sanitized output is not JSON: %v", err)
	}
	var tls map[string]any
	_ = json.Unmarshal(parsed["tlsSettings"], &tls)
	certs := tls["certificates"].([]any)
	first := certs[0].(map[string]any)
	if _, ok := first["certificateFile"]; ok {
		t.Errorf("certificateFile should have been stripped, still present: %+v", first)
	}
	if _, ok := first["keyFile"]; ok {
		t.Errorf("keyFile should have been stripped, still present: %+v", first)
	}
	if _, ok := first["certificate"]; !ok {
		t.Errorf("inline certificate should be preserved")
	}
}

func TestSanitizeStreamSettings_KeepsPathsWhenNoInline(t *testing.T) {
	stream := `{"tlsSettings": {"certificates": [{"certificateFile":"/etc/ssl/cert.pem"}]}}`
	out := sanitizeStreamSettingsForRemote(stream)
	if !strings.Contains(out, "/etc/ssl/cert.pem") {
		t.Errorf("path stripped when no inline cert was present: %s", out)
	}
}

func TestSanitizeStreamSettings_MalformedIsPassThrough(t *testing.T) {
	in := `{not valid json`
	if got := sanitizeStreamSettingsForRemote(in); got != in {
		t.Errorf("malformed input was modified: got %q want %q", got, in)
	}
}
