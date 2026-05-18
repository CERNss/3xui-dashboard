// Package auth issues and verifies JWT access tokens used by both
// the admin console (audience "admin") and the user portal (audience
// "user"). The same signing secret is used for both — they are
// separated purely by the aud claim so a single secret rotation
// invalidates every active session.
package auth

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Audience values. AudAdmin is for the admin console; AudUser is for
// the portal user.
const (
	AudAdmin = "admin"
	AudUser  = "user"
)

// Issuer is set as the `iss` claim on every token so a token
// originating from a different deployment is rejected.
const Issuer = "3xui-dashboard"

// Errors callers can branch on.
var (
	ErrInvalidCredentials = errors.New("auth: invalid credentials")
	ErrInvalidToken       = errors.New("auth: invalid token")
	ErrTokenExpired       = errors.New("auth: token expired")
	ErrWrongAudience      = errors.New("auth: wrong audience")
)

// Claims is what the dashboard puts on every issued JWT. RegisteredClaims
// covers iss/aud/sub/iat/exp; we add Username for admins (the same
// field is unused for portal tokens — those identify users by sub).
type Claims struct {
	jwt.RegisteredClaims
	Username string `json:"username,omitempty"`
}

// Service is the stateless token issuer/verifier. Construct once at
// startup with the JWT secret + access-token TTL.
type Service struct {
	secret    []byte
	ttl       time.Duration
	adminUser string
	adminPass string
}

// New returns an auth service. The admin username/password come from
// the env config; the secret is shared by both audiences.
func New(secret string, ttl time.Duration, adminUser, adminPass string) *Service {
	return &Service{
		secret:    []byte(secret),
		ttl:       ttl,
		adminUser: adminUser,
		adminPass: adminPass,
	}
}

// AdminTTL exposes the configured access-token TTL — handy for
// returning expires-in to clients.
func (s *Service) AdminTTL() time.Duration { return s.ttl }

// CheckAdminCredentials performs a constant-time comparison of the
// supplied username / password against the env-configured admin
// credentials. Returns ErrInvalidCredentials on mismatch.
func (s *Service) CheckAdminCredentials(username, password string) error {
	uOK := subtle.ConstantTimeCompare([]byte(username), []byte(s.adminUser)) == 1
	pOK := subtle.ConstantTimeCompare([]byte(password), []byte(s.adminPass)) == 1
	if !uOK || !pOK {
		return ErrInvalidCredentials
	}
	return nil
}

// IssueAdminToken returns a signed admin JWT. sub = username,
// aud = "admin".
func (s *Service) IssueAdminToken(username string, now time.Time) (string, time.Time, error) {
	exp := now.Add(s.ttl)
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			Subject:   username,
			Audience:  jwt.ClaimStrings{AudAdmin},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
		Username: username,
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("auth: sign admin token: %w", err)
	}
	return signed, exp, nil
}

// IssueUserToken returns a signed portal JWT. sub = stringified
// userID, aud = "user".
func (s *Service) IssueUserToken(userID int64, now time.Time) (string, time.Time, error) {
	exp := now.Add(s.ttl)
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			Subject:   strconv.FormatInt(userID, 10),
			Audience:  jwt.ClaimStrings{AudUser},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("auth: sign user token: %w", err)
	}
	return signed, exp, nil
}

// VerifyToken parses and validates a token, ensuring the audience
// matches one of the wantAud entries. Returns the parsed claims on
// success.
//
// Error mapping:
//   - bad signature / malformed token        → ErrInvalidToken
//   - exp in the past                        → ErrTokenExpired
//   - aud not in wantAud                     → ErrWrongAudience
func (s *Service) VerifyToken(tokenStr string, wantAud ...string) (*Claims, error) {
	if tokenStr == "" {
		return nil, ErrInvalidToken
	}
	claims := &Claims{}
	tok, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return s.secret, nil
	},
		jwt.WithIssuer(Issuer),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if !tok.Valid {
		return nil, ErrInvalidToken
	}
	if len(wantAud) > 0 {
		if !audienceMatches(claims.Audience, wantAud) {
			return nil, ErrWrongAudience
		}
	}
	return claims, nil
}

func audienceMatches(got jwt.ClaimStrings, want []string) bool {
	for _, g := range got {
		for _, w := range want {
			if g == w {
				return true
			}
		}
	}
	return false
}
