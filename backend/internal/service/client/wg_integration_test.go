package client

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/config"
	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/runtime"
	"github.com/cern/3xui-dashboard/internal/service/wgcrypto"
)

// fakeNodeLoader is the runtime.Manager dependency: returns a single
// node whose endpoint points at our httptest server.
type fakeNodeLoader struct{ node *model.Node }

func (f *fakeNodeLoader) GetNode(_ context.Context, id int64) (*model.Node, error) {
	if id != f.node.ID {
		return nil, nil
	}
	return f.node, nil
}
func (f *fakeNodeLoader) ListEnabledNodes(_ context.Context) ([]model.Node, error) {
	return []model.Node{*f.node}, nil
}

// stubPanel models the slice of the 3x-ui fork surface
// WGProvisioner actually touches: GET /inbounds/list, POST
// /inbounds/update/:id. Mutations land in m.inbounds and the next
// list call reflects them, matching the real fork's behavior.
type stubPanel struct {
	mu       sync.Mutex
	inbounds map[int64]*runtime.Inbound // by id
	idCount  int64
}

func newStubPanel() *stubPanel {
	return &stubPanel{inbounds: map[int64]*runtime.Inbound{}}
}

// seed inserts an inbound, returns the assigned id.
func (s *stubPanel) seed(in runtime.Inbound) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.idCount++
	in.ID = s.idCount
	s.inbounds[in.ID] = &in
	return in.ID
}

func (s *stubPanel) httpHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !strings.HasPrefix(req.Header.Get("Authorization"), "Bearer ") {
			http.Error(w, "missing bearer", http.StatusUnauthorized)
			return
		}
		switch {
		case req.URL.Path == "/panel/api/inbounds/list":
			s.mu.Lock()
			out := make([]runtime.Inbound, 0, len(s.inbounds))
			for _, in := range s.inbounds {
				out = append(out, *in)
			}
			s.mu.Unlock()
			writeStubEnv(w, out)
		case strings.HasPrefix(req.URL.Path, "/panel/api/inbounds/update/"):
			s.handleUpdate(w, req)
		default:
			writeStubEnv(w, nil)
		}
	}
}

func (s *stubPanel) handleUpdate(w http.ResponseWriter, req *http.Request) {
	parts := strings.Split(req.URL.Path, "/")
	if len(parts) < 6 {
		writeStubEnv(w, nil)
		return
	}
	id, _ := strconv.ParseInt(parts[5], 10, 64)
	if err := req.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	in, ok := s.inbounds[id]
	if !ok {
		writeStubEnv(w, nil)
		return
	}
	if v := req.FormValue("settings"); v != "" {
		in.Settings = v
	}
	if v := req.FormValue("streamSettings"); v != "" {
		in.StreamSettings = v
	}
	writeStubEnv(w, *in)
}

func writeStubEnv(w http.ResponseWriter, obj any) {
	type env struct {
		Success bool        `json:"success"`
		Msg     string      `json:"msg"`
		Obj     interface{} `json:"obj"`
	}
	w.Header().Set("Content-Type", "application/json")
	body, _ := json.Marshal(env{Success: true, Obj: obj})
	_, _ = w.Write(body)
}

// ---- setup ----------------------------------------------------------------

func setupWGIntegration(t *testing.T) (*WGProvisioner, *stubPanel, int64, *gorm.DB) {
	t.Helper()
	dbURL := os.Getenv("INTEGRATION_DB_URL")
	if dbURL == "" {
		t.Skip("INTEGRATION_DB_URL not set — skipping WG integration test")
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &config.Config{DB: config.DB{URL: dbURL, MaxOpenConns: 5, MaxIdleConns: 2}}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := repository.Open(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Exec("DROP SCHEMA public CASCADE").Error; err != nil {
		t.Fatalf("drop schema: %v", err)
	}
	if err := db.Exec("CREATE SCHEMA public").Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if err := repository.MigrateUp(db, logger); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = repository.Close(db) })

	panel := newStubPanel()
	srv := httptest.NewServer(panel.httpHandler())
	t.Cleanup(srv.Close)

	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())
	node := &model.Node{
		ID: 1, Name: "wg-test", Scheme: "http",
		Host: u.Hostname(), Port: port, BasePath: "",
		APIToken: "test-token", Enabled: true,
	}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("seed node: %v", err)
	}

	rtMgr := runtime.NewManager(&fakeNodeLoader{node: node}, logger)
	// Manager builds its own http.Client with SSRF guard; the
	// Remote sets WithAllowPrivate(ctx) on every request so a
	// 127.0.0.1 httptest is reachable.
	ownership := repository.NewClientOwnershipRepo(db)
	peers := repository.NewWGPeerRepo(db)
	cipher, err := wgcrypto.NewCipherFromHexKey("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	prov, err := NewWGProvisioner(rtMgr, ownership, peers, cipher)
	if err != nil {
		t.Fatalf("provisioner: %v", err)
	}

	// Seed user (FK on client_ownerships.user_id).
	if err := db.Create(&model.User{ID: 1, SubID: "test-sub", Status: model.UserStatusActive}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	wgInboundID := panel.seed(runtime.Inbound{
		Tag:      "wg-test-inbound",
		Protocol: "wireguard",
		Port:     51820,
		Enable:   true,
		Settings: `{"mtu":1420,"secretKey":"already-set-by-admin-create-flow-fAAA=","peers":[],"noKernelTun":false}`,
	})

	_ = wgInboundID
	return prov, panel, node.ID, db
}

// ---- the actual integration test ------------------------------------------

func TestProvisionPeer_LandsRowsAndPanelPeer(t *testing.T) {
	prov, panel, nodeID, db := setupWGIntegration(t)

	ownership, peer, err := prov.ProvisionPeer(
		context.Background(),
		1, // userID
		nodeID,
		"wg-test-inbound",
		"alice@example.com",
		PlanParams{DurationDays: 30, TrafficLimitBytes: 0},
	)
	_ = peer
	if err != nil {
		t.Fatalf("ProvisionPeer: %v", err)
	}

	// ---- DB-side assertions ----
	if ownership.ID == 0 {
		t.Fatal("ownership row has no id")
	}
	if ownership.Protocol != "wireguard" {
		t.Errorf("ownership.Protocol = %q, want wireguard", ownership.Protocol)
	}
	if ownership.ExpiresAt == nil {
		t.Errorf("ownership has no expires_at (plan was 30 days)")
	}

	var dbPeer model.WGPeer
	if err := db.First(&dbPeer, "client_ownership_id = ?", ownership.ID).Error; err != nil {
		t.Fatalf("wg_peers row not found: %v", err)
	}
	if dbPeer.PublicKey == "" || dbPeer.AllocatedIP == "" {
		t.Errorf("peer row missing key/ip: %+v", dbPeer)
	}
	if len(dbPeer.PrivateKeyEncrypted) == 0 {
		t.Error("private key not encrypted/stored")
	}
	// Allocator hands out .2 first (skips .0 network + .1 gateway).
	if dbPeer.AllocatedIP != "10.0.0.2" {
		t.Errorf("AllocatedIP = %q, want 10.0.0.2", dbPeer.AllocatedIP)
	}

	// ---- Decryption roundtrip ----
	priv, err := prov.DecryptPrivateKey(&dbPeer)
	if err != nil {
		t.Fatalf("DecryptPrivateKey: %v", err)
	}
	if priv == "" || len(priv) != 44 {
		t.Errorf("decrypted private key length wrong: %d", len(priv))
	}
	derived, err := wgcrypto.DerivePublic(priv)
	if err != nil {
		t.Fatalf("DerivePublic: %v", err)
	}
	if derived != dbPeer.PublicKey {
		t.Errorf("derived public %q != stored public %q (encryption is leaking?)", derived, dbPeer.PublicKey)
	}

	// ---- Panel-side assertion ----
	panel.mu.Lock()
	in := panel.inbounds[1]
	panel.mu.Unlock()
	var s runtime.WGSettings
	_ = json.Unmarshal([]byte(in.Settings), &s)
	if len(s.Peers) != 1 {
		t.Fatalf("panel peer count = %d, want 1", len(s.Peers))
	}
	if s.Peers[0].PublicKey != dbPeer.PublicKey {
		t.Errorf("panel public key %q != db public key %q", s.Peers[0].PublicKey, dbPeer.PublicKey)
	}
	if len(s.Peers[0].AllowedIPs) != 1 || s.Peers[0].AllowedIPs[0] != "10.0.0.2/32" {
		t.Errorf("AllowedIPs = %v, want [10.0.0.2/32]", s.Peers[0].AllowedIPs)
	}

	// Re-provision: should return the same row, not duplicate.
	o2, p2, err := prov.ProvisionPeer(
		context.Background(),
		1, nodeID, "wg-test-inbound", "alice@example.com",
		PlanParams{DurationDays: 30},
	)
	if err != nil {
		t.Fatalf("re-ProvisionPeer: %v", err)
	}
	if o2.ID != ownership.ID || p2.ID != dbPeer.ID {
		t.Errorf("re-provision created new rows (ownership %d→%d, peer %d→%d)",
			ownership.ID, o2.ID, dbPeer.ID, p2.ID)
	}
	panel.mu.Lock()
	_ = json.Unmarshal([]byte(panel.inbounds[1].Settings), &s)
	panel.mu.Unlock()
	if len(s.Peers) != 1 {
		t.Errorf("after re-provision panel has %d peers, want 1", len(s.Peers))
	}
}

func TestRemovePeer_ClearsBothSides(t *testing.T) {
	prov, panel, nodeID, db := setupWGIntegration(t)

	ownership, dbPeer, err := prov.ProvisionPeer(
		context.Background(),
		1, nodeID, "wg-test-inbound", "alice@example.com",
		PlanParams{DurationDays: 30},
	)
	if err != nil {
		t.Fatalf("ProvisionPeer setup: %v", err)
	}
	_ = ownership
	_ = dbPeer

	if err := prov.RemovePeer(context.Background(), nodeID, "wg-test-inbound", "alice@example.com"); err != nil {
		t.Fatalf("RemovePeer: %v", err)
	}

	// Ownership row gone.
	var ownCount int64
	if err := db.Model(&model.ClientOwnership{}).Count(&ownCount).Error; err != nil {
		t.Fatalf("count ownership: %v", err)
	}
	if ownCount != 0 {
		t.Errorf("ownership not cleared: count=%d", ownCount)
	}
	// Mirror row gone (ON DELETE CASCADE).
	var peerCount int64
	if err := db.Model(&model.WGPeer{}).Count(&peerCount).Error; err != nil {
		t.Fatalf("count peers: %v", err)
	}
	if peerCount != 0 {
		t.Errorf("wg_peers not cleared: count=%d", peerCount)
	}
	// Panel-side peers[] empty.
	panel.mu.Lock()
	in := panel.inbounds[1]
	panel.mu.Unlock()
	var s runtime.WGSettings
	_ = json.Unmarshal([]byte(in.Settings), &s)
	if len(s.Peers) != 0 {
		t.Errorf("panel still has %d peers", len(s.Peers))
	}

	// RemovePeer is idempotent — second call on already-gone row is a no-op.
	if err := prov.RemovePeer(context.Background(), nodeID, "wg-test-inbound", "alice@example.com"); err != nil {
		t.Errorf("idempotent RemovePeer should succeed, got %v", err)
	}
}

func TestProvisionPeer_AllocatesSecondIP(t *testing.T) {
	prov, _, nodeID, _ := setupWGIntegration(t)

	_, p1, err := prov.ProvisionPeer(
		context.Background(),
		1, nodeID, "wg-test-inbound", "alice@example.com",
		PlanParams{DurationDays: 30},
	)
	if err != nil {
		t.Fatalf("ProvisionPeer alice: %v", err)
	}
	// Seed a second user so the FK passes.
	// (setupWGIntegration only seeds user id=1)
	prov.peers.DB().Exec(`INSERT INTO users (id, sub_id, status) VALUES (2, 'sub-bob', $1)`, model.UserStatusActive)

	_, p2, err := prov.ProvisionPeer(
		context.Background(),
		2, nodeID, "wg-test-inbound", "bob@example.com",
		PlanParams{DurationDays: 30},
	)
	if err != nil {
		t.Fatalf("ProvisionPeer bob: %v", err)
	}
	if p1.AllocatedIP == p2.AllocatedIP {
		t.Errorf("both peers got %s — allocator collision", p1.AllocatedIP)
	}
	if p2.AllocatedIP != "10.0.0.3" {
		t.Errorf("bob got %s, want 10.0.0.3 (second slot after .2)", p2.AllocatedIP)
	}
}
