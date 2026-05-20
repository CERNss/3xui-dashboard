// Package e2e is an end-to-end integration test that drives the full
// HTTP API against a real Postgres + a mocked 3x-ui panel. The test
// is gated by the INTEGRATION_DB_URL environment variable so plain
// `go test ./...` stays hermetic.
//
// Run locally:
//
//	docker run -d --rm --name pg-e2e \
//	  -e POSTGRES_PASSWORD=test -e POSTGRES_DB=dashboard_e2e \
//	  -p 5499:5432 postgres:16-alpine
//	INTEGRATION_DB_URL='postgres://postgres:test@127.0.0.1:5499/dashboard_e2e?sslmode=disable' \
//	  go test ./internal/e2e/... -v -count=1
//	docker stop pg-e2e
//
// Each test resets the schema before running, so they can be run in
// any order.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/app"
	"github.com/cern/3xui-dashboard/internal/config"
	"github.com/cern/3xui-dashboard/internal/repository"
)

const (
	adminUser = "admin"
	adminPass = "letmein-e2e"
	jwtSecret = "e2e-test-jwt-secret-must-be-suitably-long-32"
)

// harness packs a fresh DB + a httptest server + a mocked 3x-ui panel.
type harness struct {
	t      *testing.T
	cfg    *config.Config
	db     *gorm.DB
	logger *slog.Logger
	server *httptest.Server
	panel  *mockPanel
	app    *app.App
	client *http.Client
}

// harnessOption mutates the config before app.Build is called.
// Tests use these to opt into payment gateway wiring, custom
// notify routes, etc.
type harnessOption func(*config.Config)

func setupHarness(t *testing.T, opts ...harnessOption) *harness {
	t.Helper()
	dbURL := os.Getenv("INTEGRATION_DB_URL")
	if dbURL == "" {
		t.Skip("INTEGRATION_DB_URL not set — skipping e2e (see file comment for docker one-liner)")
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	cfg := &config.Config{
		Env: "dev",
		Server: config.Server{
			ListenAddr:      "127.0.0.1:0",
			ReadTimeout:     5 * time.Second,
			WriteTimeout:    10 * time.Second,
			ShutdownTimeout: 5 * time.Second,
			LogLevel:        "error",
			LogFormat:       "text",
		},
		DB:                 config.DB{URL: dbURL, MaxOpenConns: 5, MaxIdleConns: 2, MigrateOnBoot: false},
		Auth:               config.Auth{JWTSecret: jwtSecret, AccessTokenTTL: time.Hour},
		Admin:              config.Admin{Username: adminUser, Password: adminPass},
		PublicRegistration: true,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	openCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, err := repository.Open(openCtx, cfg, logger)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	// Drop + recreate the schema so migrations re-run on every test.
	resetSchema(t, db)
	if err := repository.MigrateUp(db, logger); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	panel := newMockPanel()
	t.Cleanup(panel.Close)

	a := app.Build(cfg, db, logger)
	// Swap the manager's transport so node calls reach our httptest
	// panel without tripping the SSRF guard (the mock listens on
	// 127.0.0.1 which the guard would otherwise refuse).
	a.RuntimeManager.SetHTTPClient(panel.server.Client())

	server := httptest.NewServer(a.Engine)
	t.Cleanup(func() {
		server.Close()
		shutdownCtx, c := context.WithTimeout(context.Background(), 3*time.Second)
		defer c()
		a.Shutdown(shutdownCtx)
		_ = repository.Close(db)
	})

	return &harness{
		t:      t,
		cfg:    cfg,
		db:     db,
		logger: logger,
		server: server,
		panel:  panel,
		app:    a,
		client: server.Client(),
	}
}

// resetSchema nukes every table+sequence under the public schema.
// Cheaper + more reliable than DROP DATABASE since the DB itself
// stays around between tests.
func resetSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec("DROP SCHEMA public CASCADE").Error; err != nil {
		t.Fatalf("drop schema: %v", err)
	}
	if err := db.Exec("CREATE SCHEMA public").Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
}

// ---- HTTP helpers ----------------------------------------------------------

func (h *harness) URL(path string) string { return h.server.URL + path }

type req struct {
	method string
	path   string
	body   any
	token  string
}

// do performs the request, decodes JSON into out (if non-nil), and
// returns the HTTP status. Body is JSON-encoded if non-nil.
func (h *harness) do(t *testing.T, r req, out any) int {
	t.Helper()
	var body io.Reader
	if r.body != nil {
		b, err := json.Marshal(r.body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		body = bytes.NewReader(b)
	}
	httpReq, err := http.NewRequest(r.method, h.URL(r.path), body)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if r.body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	if r.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.token)
	}
	resp, err := h.client.Do(httpReq)
	if err != nil {
		t.Fatalf("%s %s: %v", r.method, r.path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if out != nil && len(raw) > 0 {
		_ = json.Unmarshal(raw, out)
	}
	return resp.StatusCode
}

// raw is like do but returns the body bytes too — for assertions on
// non-JSON responses like /sub/<id>.
func (h *harness) raw(t *testing.T, r req) (int, http.Header, []byte) {
	t.Helper()
	var body io.Reader
	if r.body != nil {
		b, _ := json.Marshal(r.body)
		body = bytes.NewReader(b)
	}
	httpReq, _ := http.NewRequest(r.method, h.URL(r.path), body)
	if r.body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	if r.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+r.token)
	}
	resp, err := h.client.Do(httpReq)
	if err != nil {
		t.Fatalf("%s %s: %v", r.method, r.path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, raw
}

func (h *harness) adminLogin(t *testing.T) string {
	t.Helper()
	var out struct {
		Token string `json:"token"`
	}
	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/admin/auth/login",
		body:   map[string]string{"username": adminUser, "password": adminPass},
	}, &out); got != http.StatusOK {
		t.Fatalf("admin login: status=%d", got)
	}
	if out.Token == "" {
		t.Fatal("admin login returned empty token")
	}
	return out.Token
}

func (h *harness) registerUser(t *testing.T, email, password string) (id int64, token string) {
	t.Helper()
	var out struct {
		Token  string `json:"token"`
		UserID int64  `json:"user_id"`
	}
	if got := h.do(t, req{
		method: http.MethodPost,
		path:   "/api/user/auth/register",
		body:   map[string]string{"email": email, "password": password},
	}, &out); got != http.StatusCreated {
		t.Fatalf("register: status=%d", got)
	}
	return out.UserID, out.Token
}

// jsonString is a tiny helper so test assertions can do path lookups
// without dragging in a JSON-path library.
func jsonString(b []byte, key string) string {
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

// itoa keeps the test prose tight.
func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// pathEsc keeps email path-escaping tight.
func pathEsc(s string) string { return url.PathEscape(s) }

// formatHeader is unused but kept to suppress "fmt imported and not
// used" if other helpers are removed.
var _ = fmt.Sprintf

// ensure strings package stays imported even if a future change
// removes the only consumer (handy for the next test to add).
var _ = strings.HasPrefix
