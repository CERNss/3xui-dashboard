package payment

import (
	"context"
	"errors"
	"testing"

	"github.com/cern/3xui-dashboard/internal/model"
)

func TestFormatYuan(t *testing.T) {
	cases := []struct {
		cents int64
		want  string
	}{
		{0, "0.00"},
		{1, "0.01"},
		{10, "0.10"},
		{99, "0.99"},
		{100, "1.00"},
		{1234, "12.34"},
		{99999, "999.99"},
		{100000, "1000.00"},
	}
	for _, c := range cases {
		if got := FormatYuan(c.cents); got != c.want {
			t.Errorf("FormatYuan(%d) = %q, want %q", c.cents, got, c.want)
		}
	}
}

// stubGateway implements Gateway for registry tests.
type stubGateway struct{ name string }

func (s *stubGateway) Provider() string { return s.name }
func (s *stubGateway) CreatePayment(_ context.Context, _ *model.Order, _ string) (CreateResult, error) {
	return CreateResult{}, nil
}
func (s *stubGateway) Query(_ context.Context, _ string) (Status, error)         { return StatusPending, nil }
func (s *stubGateway) VerifyNotify(_ map[string]string, _ string) error          { return nil }

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	if _, err := r.Get("alipay"); !errors.Is(err, ErrUnknownProvider) {
		t.Errorf("Get on empty registry should be ErrUnknownProvider, got %v", err)
	}
	r.Register(&stubGateway{name: "alipay"})
	if _, err := r.Get("alipay"); err != nil {
		t.Errorf("Get after Register: %v", err)
	}
}

func TestRegistry_NilRegisterIsNoOp(t *testing.T) {
	r := NewRegistry()
	r.Register(nil) // must not panic
	if got := r.EnabledProviders(); len(got) != 1 || got[0] != "balance" {
		t.Errorf("EnabledProviders after nil register = %v, want [balance]", got)
	}
}

func TestRegistry_EnabledProvidersIncludesBalance(t *testing.T) {
	r := NewRegistry()
	r.Register(&stubGateway{name: "alipay"})
	r.Register(&stubGateway{name: "stripe"})
	got := r.EnabledProviders()
	if len(got) != 3 {
		t.Errorf("EnabledProviders = %v, want 3 entries", got)
	}
	if got[0] != "balance" {
		t.Errorf("first entry = %q, want balance (always first)", got[0])
	}
}
