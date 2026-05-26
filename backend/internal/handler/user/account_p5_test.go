package user

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/config"
	"github.com/cern/3xui-dashboard/internal/mailer"
	"github.com/cern/3xui-dashboard/internal/middleware"
	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	authsvc "github.com/cern/3xui-dashboard/internal/service/auth"
	"github.com/cern/3xui-dashboard/internal/service/event"
	"github.com/cern/3xui-dashboard/internal/service/messages"
	usersvc "github.com/cern/3xui-dashboard/internal/service/user"
	"github.com/cern/3xui-dashboard/internal/service/verification"
)

type p5Harness struct {
	engine *gin.Engine
	db     *gorm.DB
	users  *usersvc.Service
	verify *verification.Service
	auth   *authsvc.Service
}

func setupP5Harness(t *testing.T) *p5Harness {
	t.Helper()
	dbURL := os.Getenv("INTEGRATION_DB_URL")
	if dbURL == "" {
		t.Skip("INTEGRATION_DB_URL not set — skipping user handler P5 tests")
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := &config.Config{
		DB:                 config.DB{URL: dbURL, MaxOpenConns: 5, MaxIdleConns: 2},
		PublicRegistration: true,
		OIDC: config.OIDC{
			Issuer:       "https://idp.example.com",
			ClientID:     "client",
			ClientSecret: "secret",
			RedirectURL:  "https://dashboard.example.com/oidc/callback",
			Scopes:       []string{"openid", "email", "profile"},
			DisplayName:  "Example SSO",
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	userRepo := repository.NewUserRepo(db)
	settingRepo := repository.NewSettingRepo(db)
	bus := event.New()
	userService := usersvc.New(userRepo, settingRepo, bus, cfg, logger)
	mailerSvc := mailer.New(config.SMTP{}, logger)
	msgs := messages.New(mailerSvc, repository.NewNotificationLogRepo(db), bus, userRepo, nil, logger)
	verifyService := verification.New(db, msgs, logger)
	authService := authsvc.New("a-very-long-test-secret-value", time.Hour, "admin", "password")

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	apiUser := engine.Group("/api/user")
	NewAuthHandler(userService, authService, verifyService, true).RegisterRoutes(apiUser)
	apiUserAuthed := engine.Group("/api/user", middleware.RequireActiveUser(authService, userRepo))
	NewAccountHandler(userService, userRepo, verifyService).RegisterRoutes(apiUserAuthed)

	return &p5Harness{engine: engine, db: db, users: userService, verify: verifyService, auth: authService}
}

func (h *p5Harness) seedUser(t *testing.T, email, password string) *model.User {
	t.Helper()
	if _, err := h.users.AdminCreate(context.Background(), usersvc.AdminCreateInput{
		Email:    email,
		Password: password,
	}); err != nil {
		t.Fatalf("admin create: %v", err)
	}
	got, err := h.users.Login(context.Background(), usersvc.LoginInput{Email: email, Password: password})
	if err != nil {
		t.Fatalf("login seeded: %v", err)
	}
	return got
}

func (h *p5Harness) token(t *testing.T, userID int64) string {
	t.Helper()
	token, _, err := h.auth.IssueUserToken(userID, time.Now())
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return token
}

func (h *p5Harness) doJSON(t *testing.T, method, path, token string, body any, out any) int {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.engine.ServeHTTP(w, req)
	if out != nil {
		if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
			t.Fatalf("decode response (%d %s): %v", w.Code, w.Body.String(), err)
		}
	}
	return w.Code
}

func TestP5ProfilePatchUpdatesDisplayName(t *testing.T) {
	h := setupP5Harness(t)
	u := h.seedUser(t, "alice@example.com", "password123")

	var out model.User
	status := h.doJSON(t, http.MethodPatch, "/api/user/profile", h.token(t, u.ID), gin.H{
		"display_name": "Alice Cooper",
	}, &out)
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if out.DisplayName != "Alice Cooper" {
		t.Fatalf("display_name = %q", out.DisplayName)
	}
}

func TestP5ChangeEmailRequiresVerificationToken(t *testing.T) {
	h := setupP5Harness(t)
	u := h.seedUser(t, "alice@example.com", "password123")

	status := h.doJSON(t, http.MethodPost, "/api/user/change-email", h.token(t, u.ID), gin.H{
		"email": "new@example.com",
	}, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("missing token status = %d", status)
	}

	confirmed, err := h.verify.Confirm(context.Background(), "new@example.com", seedCode(t, h.db, "new@example.com", verification.PurposeChangeEmail), verification.PurposeChangeEmail)
	if err != nil {
		t.Fatalf("confirm code: %v", err)
	}
	var out model.User
	status = h.doJSON(t, http.MethodPost, "/api/user/change-email", h.token(t, u.ID), gin.H{
		"email":              "new@example.com",
		"verification_token": confirmed.Token,
	}, &out)
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if out.Email == nil || *out.Email != "new@example.com" || !out.EmailVerified {
		t.Fatalf("email not verified: %+v", out)
	}
}

func TestP5OIDCCreateAccountDoesNotConsumeTokenWhenPendingInvalid(t *testing.T) {
	h := setupP5Harness(t)
	code := seedCode(t, h.db, "new@example.com", verification.PurposeOIDCCreateAccount)
	confirmed, err := h.verify.Confirm(context.Background(), "new@example.com", code, verification.PurposeOIDCCreateAccount)
	if err != nil {
		t.Fatalf("confirm code: %v", err)
	}

	status := h.doJSON(t, http.MethodPost, "/api/user/auth/oidc/create-account", "", gin.H{
		"pending_token":      "missing",
		"display_name":       "New User",
		"email":              "new@example.com",
		"password":           "password123",
		"verification_token": confirmed.Token,
	}, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("invalid pending status = %d", status)
	}
	if err := h.verify.CheckToken(context.Background(), "new@example.com", verification.PurposeOIDCCreateAccount, confirmed.Token); err != nil {
		t.Fatalf("verification token should remain reusable after preflight failure: %v", err)
	}
}

func seedCode(t *testing.T, db *gorm.DB, email string, purpose verification.Purpose) string {
	t.Helper()
	const code = "123456"
	row := struct {
		Email      string
		Purpose    string
		CodeHash   string `gorm:"column:code_hash"`
		ExpiresAt  time.Time
		SentAt     time.Time
		Attempts   int
		ConsumedAt *time.Time
	}{
		Email:     email,
		Purpose:   string(purpose),
		CodeHash:  hashVerificationCodeForTest(code),
		ExpiresAt: time.Now().Add(10 * time.Minute),
		SentAt:    time.Now().Add(-time.Minute),
	}
	if err := db.Table("email_verification_codes").Create(&row).Error; err != nil {
		t.Fatalf("seed code: %v", err)
	}
	return code
}

func hashVerificationCodeForTest(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}
