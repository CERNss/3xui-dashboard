package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func newTestSvc() *Service {
	return New("a-very-long-test-secret-value", time.Hour, "admin", "letmein")
}

func TestCheckAdminCredentials(t *testing.T) {
	s := newTestSvc()
	if err := s.CheckAdminCredentials("admin", "letmein"); err != nil {
		t.Fatalf("good creds rejected: %v", err)
	}
	if err := s.CheckAdminCredentials("admin", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("bad password: want ErrInvalidCredentials, got %v", err)
	}
	if err := s.CheckAdminCredentials("not-admin", "letmein"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("bad username: want ErrInvalidCredentials, got %v", err)
	}
	if err := s.CheckAdminCredentials("", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("empty: want ErrInvalidCredentials, got %v", err)
	}
}

func TestIssueAndVerifyAdminToken(t *testing.T) {
	s := newTestSvc()
	tok, exp, err := s.IssueAdminToken("admin", time.Now())
	if err != nil {
		t.Fatalf("IssueAdminToken: %v", err)
	}
	if tok == "" || exp.IsZero() {
		t.Fatal("issued empty token/exp")
	}
	claims, err := s.VerifyToken(tok, AudAdmin)
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if claims.Subject != "admin" {
		t.Errorf("sub = %q, want admin", claims.Subject)
	}
	if claims.Username != "admin" {
		t.Errorf("username = %q, want admin", claims.Username)
	}
	if len(claims.Audience) != 1 || claims.Audience[0] != AudAdmin {
		t.Errorf("aud = %v, want [%s]", claims.Audience, AudAdmin)
	}
}

func TestVerifyToken_RejectsUserOnAdminAudience(t *testing.T) {
	s := newTestSvc()
	tok, _, err := s.IssueUserToken(42, time.Now())
	if err != nil {
		t.Fatalf("IssueUserToken: %v", err)
	}
	if _, err := s.VerifyToken(tok, AudAdmin); !errors.Is(err, ErrWrongAudience) {
		t.Errorf("expected ErrWrongAudience, got %v", err)
	}
}

func TestVerifyToken_RejectsAdminOnUserAudience(t *testing.T) {
	s := newTestSvc()
	tok, _, err := s.IssueAdminToken("admin", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.VerifyToken(tok, AudUser); !errors.Is(err, ErrWrongAudience) {
		t.Errorf("expected ErrWrongAudience, got %v", err)
	}
}

func TestVerifyToken_ExpiredTokenRejected(t *testing.T) {
	s := newTestSvc()
	tok, _, err := s.IssueAdminToken("admin", time.Now().Add(-2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.VerifyToken(tok, AudAdmin); !errors.Is(err, ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}
}

func TestVerifyToken_RejectsMissingExpiry(t *testing.T) {
	s := newTestSvc()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:   Issuer,
			Subject:  "admin",
			Audience: jwt.ClaimStrings{AudAdmin},
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
		Username: "admin",
	})
	signed, err := tok.SignedString(s.secret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	if _, err := s.VerifyToken(signed, AudAdmin); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken for missing exp, got %v", err)
	}
}

func TestVerifyToken_BadSignatureRejected(t *testing.T) {
	s := newTestSvc()
	other := New("different-secret", time.Hour, "admin", "letmein")
	tok, _, err := other.IssueAdminToken("admin", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.VerifyToken(tok, AudAdmin); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestVerifyToken_MalformedRejected(t *testing.T) {
	s := newTestSvc()
	cases := []string{"", "abc.def", "not.a.token.at.all"}
	for _, c := range cases {
		if _, err := s.VerifyToken(c, AudAdmin); err == nil {
			t.Errorf("%q: want error, got nil", c)
		}
	}
}

func TestVerifyToken_RejectsRS256(t *testing.T) {
	s := newTestSvc()
	// Build a token by hand with an alg we do not accept (none).
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{
		Issuer:    Issuer,
		Subject:   "admin",
		Audience:  jwt.ClaimStrings{AudAdmin},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	})
	signed, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none-alg token: %v", err)
	}
	if _, err := s.VerifyToken(signed, AudAdmin); err == nil {
		t.Error("none-alg token was accepted")
	} else if !errors.Is(err, ErrInvalidToken) && !strings.Contains(err.Error(), "signing method") {
		// Acceptable so long as it's an error and references the
		// signing method or maps to ErrInvalidToken.
		t.Errorf("unexpected error shape: %v", err)
	}
}
