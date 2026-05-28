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
	"strings"
	"testing"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/runtime"
)

// mockPanelServer returns an httptest.Server that answers the three
// panel keygen endpoints we care about with fixed canned responses.
// callCount records how many times each endpoint was hit so tests can
// assert "no panel round-trip happened" cases too.
type mockPanelServer struct {
	*httptest.Server
	callCount map[string]int
}

func newMockPanelServer(t *testing.T) *mockPanelServer {
	t.Helper()
	m := &mockPanelServer{callCount: map[string]int{}}
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		m.callCount[req.URL.Path]++
		w.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/panel/api/server/getNewX25519Cert":
			writeEnvelope(w, map[string]string{
				"privateKey": "FAKE_PRIV_X25519",
				"publicKey":  "FAKE_PUB_X25519",
			})
		case "/panel/api/server/getNewmldsa65":
			writeEnvelope(w, map[string]string{
				"seed":   "FAKE_MLDSA65_SEED",
				"verify": "FAKE_MLDSA65_VERIFY",
			})
		case "/panel/api/server/getNewVlessEnc":
			writeEnvelope(w, map[string]any{
				"auths": []map[string]string{
					{"id": "x25519", "label": "X25519", "decryption": "DEC_X25519", "encryption": "ENC_X25519"},
					{"id": "mlkem768", "label": "ML-KEM-768", "decryption": "DEC_MLKEM768", "encryption": "ENC_MLKEM768"},
				},
			})
		default:
			http.NotFound(w, req)
		}
	})
	m.Server = httptest.NewServer(handler)
	t.Cleanup(m.Server.Close)
	return m
}

func writeEnvelope(w http.ResponseWriter, obj any) {
	raw, _ := json.Marshal(obj)
	env := map[string]any{
		"success": true,
		"msg":     "ok",
		"obj":     json.RawMessage(raw),
	}
	body, _ := json.Marshal(env)
	_, _ = io.WriteString(w, string(body))
}

func remoteFromMock(t *testing.T, m *mockPanelServer) *runtime.Remote {
	t.Helper()
	u, err := url.Parse(m.URL)
	if err != nil {
		t.Fatalf("parse mock URL: %v", err)
	}
	port, _ := strconv.Atoi(u.Port())
	node := &model.Node{
		ID:       1,
		Name:     "mock",
		Scheme:   u.Scheme,
		Host:     u.Hostname(),
		Port:     port,
		APIToken: "secret",
		Enabled:  true,
	}
	return runtime.NewRemote(node, m.Client(), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestResolveIntent_VlessEncryptionX25519(t *testing.T) {
	m := newMockPanelServer(t)
	r := remoteFromMock(t, m)

	in := &runtime.Inbound{
		Protocol: "vless",
		Settings: `{"clients":[],"decryption":"auto:x25519","encryption":"auto:x25519","_intent":{"vlessAuth":"x25519"}}`,
	}
	if err := resolveIntent(context.Background(), r, in); err != nil {
		t.Fatalf("resolveIntent: %v", err)
	}
	if strings.Contains(in.Settings, "_intent") {
		t.Errorf("settings still contains _intent: %s", in.Settings)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(in.Settings), &parsed); err != nil {
		t.Fatalf("settings JSON invalid: %v", err)
	}
	if parsed["decryption"] != "DEC_X25519" {
		t.Errorf("decryption = %v, want DEC_X25519", parsed["decryption"])
	}
	if parsed["encryption"] != "ENC_X25519" {
		t.Errorf("encryption = %v, want ENC_X25519", parsed["encryption"])
	}
	if m.callCount["/panel/api/server/getNewVlessEnc"] != 1 {
		t.Errorf("getNewVlessEnc call count = %d, want 1", m.callCount["/panel/api/server/getNewVlessEnc"])
	}
}

func TestResolveIntent_VlessEncryptionMlkem768(t *testing.T) {
	m := newMockPanelServer(t)
	r := remoteFromMock(t, m)

	in := &runtime.Inbound{
		Protocol: "vless",
		Settings: `{"clients":[],"_intent":{"vlessAuth":"mlkem768"}}`,
	}
	if err := resolveIntent(context.Background(), r, in); err != nil {
		t.Fatalf("resolveIntent: %v", err)
	}
	var parsed map[string]any
	_ = json.Unmarshal([]byte(in.Settings), &parsed)
	if parsed["decryption"] != "DEC_MLKEM768" {
		t.Errorf("decryption = %v, want DEC_MLKEM768", parsed["decryption"])
	}
	if parsed["encryption"] != "ENC_MLKEM768" {
		t.Errorf("encryption = %v, want ENC_MLKEM768", parsed["encryption"])
	}
}

func TestResolveIntent_RealityKeypair(t *testing.T) {
	m := newMockPanelServer(t)
	r := remoteFromMock(t, m)

	in := &runtime.Inbound{
		Protocol:       "vless",
		Settings:       `{"clients":[]}`,
		StreamSettings: `{"network":"tcp","security":"reality","realitySettings":{"dest":"www.amazon.com:443"},"_intent":{"realityKeypair":true}}`,
	}
	if err := resolveIntent(context.Background(), r, in); err != nil {
		t.Fatalf("resolveIntent: %v", err)
	}
	if strings.Contains(in.StreamSettings, "_intent") {
		t.Errorf("streamSettings still contains _intent: %s", in.StreamSettings)
	}
	var stream map[string]any
	_ = json.Unmarshal([]byte(in.StreamSettings), &stream)
	reality, _ := stream["realitySettings"].(map[string]any)
	if reality["privateKey"] != "FAKE_PRIV_X25519" || reality["publicKey"] != "FAKE_PUB_X25519" {
		t.Errorf("reality keypair not filled: %+v", reality)
	}
	if m.callCount["/panel/api/server/getNewX25519Cert"] != 1 {
		t.Errorf("getNewX25519Cert call count = %d, want 1", m.callCount["/panel/api/server/getNewX25519Cert"])
	}
}

func TestResolveIntent_Mldsa65(t *testing.T) {
	m := newMockPanelServer(t)
	r := remoteFromMock(t, m)

	in := &runtime.Inbound{
		StreamSettings: `{"security":"reality","realitySettings":{},"_intent":{"realityMldsa65":true}}`,
	}
	if err := resolveIntent(context.Background(), r, in); err != nil {
		t.Fatalf("resolveIntent: %v", err)
	}
	var stream map[string]any
	_ = json.Unmarshal([]byte(in.StreamSettings), &stream)
	reality, _ := stream["realitySettings"].(map[string]any)
	if reality["mldsa65Seed"] != "FAKE_MLDSA65_SEED" || reality["mldsa65Verify"] != "FAKE_MLDSA65_VERIFY" {
		t.Errorf("mldsa65 not filled: %+v", reality)
	}
}

func TestResolveIntent_ShortIdsRandom(t *testing.T) {
	m := newMockPanelServer(t)
	r := remoteFromMock(t, m)

	in := &runtime.Inbound{
		StreamSettings: `{"realitySettings":{},"_intent":{"realityRandomShortIds":true}}`,
	}
	if err := resolveIntent(context.Background(), r, in); err != nil {
		t.Fatalf("resolveIntent: %v", err)
	}
	var stream map[string]any
	_ = json.Unmarshal([]byte(in.StreamSettings), &stream)
	reality, _ := stream["realitySettings"].(map[string]any)
	ids, _ := reality["shortIds"].([]any)
	if len(ids) != 8 {
		t.Errorf("expected 8 short IDs, got %d", len(ids))
	}
	for _, raw := range ids {
		s, _ := raw.(string)
		if len(s) < 2 || len(s) > 16 {
			t.Errorf("short ID %q out of expected hex length range", s)
		}
	}
	// Short IDs are generated locally — no panel round-trip.
	if m.callCount["/panel/api/server/getNewX25519Cert"] != 0 {
		t.Errorf("unexpected X25519 call when only short IDs requested")
	}
}

func TestResolveIntent_WireguardKeypair(t *testing.T) {
	m := newMockPanelServer(t)
	r := remoteFromMock(t, m)

	in := &runtime.Inbound{
		Protocol: "wireguard",
		Settings: `{"mtu":1420,"secretKey":"","pubKey":"","peers":[],"_intent":{"wireguardKeypair":true}}`,
	}
	if err := resolveIntent(context.Background(), r, in); err != nil {
		t.Fatalf("resolveIntent: %v", err)
	}
	if strings.Contains(in.Settings, "_intent") {
		t.Errorf("settings still contains _intent: %s", in.Settings)
	}
	var parsed map[string]any
	_ = json.Unmarshal([]byte(in.Settings), &parsed)
	secret, _ := parsed["secretKey"].(string)
	pub, _ := parsed["pubKey"].(string)
	if len(secret) != 44 || len(pub) != 44 {
		t.Errorf("WG keypair not filled: secret=%q pub=%q", secret, pub)
	}
	if secret == pub {
		t.Errorf("secret and pub keys are identical")
	}
	if total := totalCalls(m.callCount); total != 0 {
		t.Errorf("WG keypair generation should not hit panel, got %d calls", total)
	}
}

func TestResolveIntent_NoIntentNoCall(t *testing.T) {
	m := newMockPanelServer(t)
	r := remoteFromMock(t, m)

	in := &runtime.Inbound{
		Protocol:       "vless",
		Settings:       `{"clients":[],"decryption":"none","encryption":"none"}`,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	originalSettings := in.Settings
	originalStream := in.StreamSettings
	if err := resolveIntent(context.Background(), r, in); err != nil {
		t.Fatalf("resolveIntent: %v", err)
	}
	if in.Settings != originalSettings {
		t.Errorf("settings mutated despite no intent: was=%s now=%s", originalSettings, in.Settings)
	}
	if in.StreamSettings != originalStream {
		t.Errorf("streamSettings mutated despite no intent")
	}
	if total := totalCalls(m.callCount); total != 0 {
		t.Errorf("expected zero panel calls when no intent present, got %d", total)
	}
}

func totalCalls(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}
