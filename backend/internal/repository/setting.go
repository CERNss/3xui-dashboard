package repository

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/cern/3xui-dashboard/internal/model"
)

// Cipher seals/opens secret setting values at rest. Satisfied by
// wgcrypto.Cipher; injected via SetCipher so the repository layer
// doesn't depend on a service package. Nil = encryption unavailable.
type Cipher interface {
	Seal(plaintext []byte) ([]byte, error)
	Open(sealed []byte) ([]byte, error)
}

// secretPrefix marks a stored value as encrypted so GetSecret knows to
// decrypt it (and a plaintext value is never mistaken for ciphertext).
const secretPrefix = "enc:v1:"

// SettingRepo persists admin-controlled runtime toggles in the
// `settings` key/value table. Reads are point lookups by key; writes
// are upserts on the primary key. All values are stored as TEXT —
// typed coercion happens via the helper methods below.
type SettingRepo struct {
	db     *gorm.DB
	cipher Cipher // optional; set via SetCipher to enable GetSecret/SetSecret
}

// NewSettingRepo returns a repository bound to db.
func NewSettingRepo(db *gorm.DB) *SettingRepo {
	return &SettingRepo{db: db}
}

// SetCipher wires the at-rest cipher used by GetSecret/SetSecret. When
// unset, SetSecret errors and GetSecret can only return plaintext rows.
func (r *SettingRepo) SetCipher(c Cipher) { r.cipher = c }

// HasCipher reports whether an at-rest cipher is configured — i.e.
// whether secret settings can be stored. The admin UI uses this to warn
// when SECRET_ENCRYPTION_KEY is unset before a save fails.
func (r *SettingRepo) HasCipher() bool { return r.cipher != nil }

// Get returns the value for key. Returns ("", false, nil) if the key
// is not present. A storage error is returned as a non-nil error.
func (r *SettingRepo) Get(ctx context.Context, key string) (string, bool, error) {
	var row model.Setting
	if err := r.db.WithContext(ctx).First(&row, "key = ?", key).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("setting.Get %q: %w", key, err)
	}
	return row.Value, true, nil
}

// Set upserts (key, value). The updated_at column is set to now() by
// the database default on insert, and stamped here on update.
func (r *SettingRepo) Set(ctx context.Context, key, value string) error {
	row := model.Setting{Key: key, Value: value}
	res := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&row)
	if res.Error != nil {
		return fmt.Errorf("setting.Set %q: %w", key, res.Error)
	}
	return nil
}

// SetSecret encrypts plaintext with the configured cipher and upserts it.
// Errors if no cipher is configured (set SECRET_ENCRYPTION_KEY).
func (r *SettingRepo) SetSecret(ctx context.Context, key, plaintext string) error {
	if r.cipher == nil {
		return errors.New("setting.SetSecret: no cipher configured (set SECRET_ENCRYPTION_KEY)")
	}
	stored, err := sealSecret(r.cipher, plaintext)
	if err != nil {
		return fmt.Errorf("setting.SetSecret %q: %w", key, err)
	}
	return r.Set(ctx, key, stored)
}

// GetSecret reads + decrypts a secret value, returning fallback when the
// key is absent. A value stored without the encrypted prefix (e.g. set
// via plain Set) is returned as-is. Returns an error if an encrypted
// value is found but no cipher is configured.
func (r *SettingRepo) GetSecret(ctx context.Context, key, fallback string) (string, error) {
	stored, ok, err := r.Get(ctx, key)
	if err != nil {
		return fallback, err
	}
	if !ok || stored == "" {
		return fallback, nil
	}
	if !strings.HasPrefix(stored, secretPrefix) {
		return stored, nil // plaintext row
	}
	if r.cipher == nil {
		return fallback, errors.New("setting.GetSecret: encrypted value but no cipher configured")
	}
	pt, err := openSecret(r.cipher, stored)
	if err != nil {
		return fallback, fmt.Errorf("setting.GetSecret %q: %w", key, err)
	}
	return pt, nil
}

// sealSecret encrypts plaintext and returns the prefixed, base64-encoded
// storage form.
func sealSecret(c Cipher, plaintext string) (string, error) {
	sealed, err := c.Seal([]byte(plaintext))
	if err != nil {
		return "", err
	}
	return secretPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// openSecret reverses sealSecret. A value without secretPrefix is
// returned unchanged (treated as plaintext).
func openSecret(c Cipher, stored string) (string, error) {
	if !strings.HasPrefix(stored, secretPrefix) {
		return stored, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, secretPrefix))
	if err != nil {
		return "", fmt.Errorf("decode secret: %w", err)
	}
	pt, err := c.Open(raw)
	if err != nil {
		return "", fmt.Errorf("open secret: %w", err)
	}
	return string(pt), nil
}

// Delete removes a row. Missing keys are not an error.
func (r *SettingRepo) Delete(ctx context.Context, key string) error {
	if err := r.db.WithContext(ctx).Delete(&model.Setting{}, "key = ?", key).Error; err != nil {
		return fmt.Errorf("setting.Delete %q: %w", key, err)
	}
	return nil
}

// GetAll returns every key/value pair, keyed by string. Useful for the
// admin Settings page (task 14.9).
func (r *SettingRepo) GetAll(ctx context.Context) (map[string]string, error) {
	var rows []model.Setting
	if err := r.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("setting.GetAll: %w", err)
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.Key] = r.Value
	}
	return out, nil
}

// ---- Typed accessors -------------------------------------------------------

// GetBool returns the value coerced to bool, or fallback if the key is
// not present. Coercion accepts: "true"/"false", "1"/"0", "yes"/"no",
// case-insensitive. Anything else returns an error.
func (r *SettingRepo) GetBool(ctx context.Context, key string, fallback bool) (bool, error) {
	v, ok, err := r.Get(ctx, key)
	if err != nil || !ok {
		return fallback, err
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes", "on":
		return true, nil
	case "false", "0", "no", "off":
		return false, nil
	default:
		return fallback, fmt.Errorf("setting %q: invalid bool %q", key, v)
	}
}

// GetInt returns the value coerced to int64, or fallback if absent.
func (r *SettingRepo) GetInt(ctx context.Context, key string, fallback int64) (int64, error) {
	v, ok, err := r.Get(ctx, key)
	if err != nil || !ok {
		return fallback, err
	}
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil {
		return fallback, fmt.Errorf("setting %q: invalid int %q", key, v)
	}
	return n, nil
}

// GetString returns the value or fallback if absent.
func (r *SettingRepo) GetString(ctx context.Context, key, fallback string) (string, error) {
	v, ok, err := r.Get(ctx, key)
	if err != nil || !ok {
		return fallback, err
	}
	return v, nil
}

// SetBool / SetInt are convenience wrappers around Set.
func (r *SettingRepo) SetBool(ctx context.Context, key string, value bool) error {
	return r.Set(ctx, key, strconv.FormatBool(value))
}

func (r *SettingRepo) SetInt(ctx context.Context, key string, value int64) error {
	return r.Set(ctx, key, strconv.FormatInt(value, 10))
}
