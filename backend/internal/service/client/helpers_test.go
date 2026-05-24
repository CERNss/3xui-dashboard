package client

import (
	"strings"
	"testing"
	"time"

	"github.com/cern/3xui-dashboard/internal/model"
)

func TestComputeExpiry(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour

	cases := []struct {
		desc         string
		existing     *model.ClientOwnership
		durationDays int
		wantOffset   time.Duration
		wantZero     bool
	}{
		{
			desc:         "first provision 30d",
			durationDays: 30,
			wantOffset:   30 * day,
		},
		{
			desc:         "zero duration → non-expiring",
			durationDays: 0,
			wantZero:     true,
		},
		{
			desc: "extend from existing future expiry (15 days remaining + 30 = 45)",
			existing: &model.ClientOwnership{
				ExpiresAt: tptr(now.Add(15 * day)),
			},
			durationDays: 30,
			wantOffset:   45 * day,
		},
		{
			desc: "expired existing → extend from now",
			existing: &model.ClientOwnership{
				ExpiresAt: tptr(now.Add(-5 * day)),
			},
			durationDays: 30,
			wantOffset:   30 * day,
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			got := computeExpiry(now, c.existing, c.durationDays)
			if c.wantZero {
				if !got.IsZero() {
					t.Errorf("want zero time, got %v", got)
				}
				return
			}
			want := now.Add(c.wantOffset)
			if !got.Equal(want) {
				t.Errorf("got %v, want %v", got, want)
			}
		})
	}
}

func TestBuildWireClient_VLESSGetsUUID(t *testing.T) {
	c := buildWireClient("vless", "alice", "sub-1", time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), 1<<30, 0)
	if c.ID == "" {
		t.Error("vless client should have ID (UUID)")
	}
	if c.Password != "" {
		t.Error("vless client should not have Password")
	}
	if c.Email != "alice" || c.SubID != "sub-1" {
		t.Errorf("identity mismatch: %+v", c)
	}
	if c.TotalGB != 1<<30 {
		t.Errorf("TotalGB = %d, want %d (bytes)", c.TotalGB, 1<<30)
	}
	if c.ExpiryTime == 0 {
		t.Error("ExpiryTime should be ms-since-epoch, got 0")
	}
}

func TestBuildWireClient_AppliesIPLimit(t *testing.T) {
	c := buildWireClient("vless", "alice", "sub-1", time.Time{}, 0, 3)
	if c.LimitIP != 3 {
		t.Errorf("LimitIP = %d, want 3", c.LimitIP)
	}
}

func TestBuildWireClient_TrojanGetsPassword(t *testing.T) {
	c := buildWireClient("trojan", "bob", "sub-2", time.Time{}, 0, 0)
	if c.Password == "" {
		t.Error("trojan client should have Password")
	}
	if c.ID != "" {
		t.Error("trojan client should not have ID")
	}
	if c.ExpiryTime != 0 {
		t.Errorf("non-expiring → ExpiryTime should be 0, got %d", c.ExpiryTime)
	}
	if !strings.HasPrefix(c.Password, "") || len(c.Password) < 16 {
		t.Errorf("Password looks too short: %q", c.Password)
	}
}

func TestBuildWireClient_ShadowsocksGetsPassword(t *testing.T) {
	c := buildWireClient("shadowsocks", "carol", "sub-3", time.Time{}, 0, 0)
	if c.Password == "" {
		t.Error("shadowsocks client should have Password")
	}
}

func TestBuildWireClient_HysteriaGetsAuth(t *testing.T) {
	c := buildWireClient("hysteria", "dan", "sub-4", time.Time{}, 0, 0)
	if c.Auth == "" {
		t.Error("hysteria client should have Auth populated")
	}
	if c.ID != "" || c.Password != "" {
		t.Errorf("hysteria client should not carry ID/Password (got id=%q password=%q)", c.ID, c.Password)
	}
	c2 := buildWireClient("hysteria2", "erin", "sub-5", time.Time{}, 0, 0)
	if c2.Auth != "" || c2.ID == "" {
		t.Error("unsupported hysteria2 input should use the generic UUID shape")
	}
}

func TestBuildWireClient_UnknownGetsUUIDIdAsSafeDefault(t *testing.T) {
	c := buildWireClient("totally-made-up", "fred", "sub-6", time.Time{}, 0, 0)
	if c.ID == "" {
		t.Error("unknown protocol should default to UUID id")
	}
}

func TestRandomAuthString_FormatAndUnique(t *testing.T) {
	seen := map[string]struct{}{}
	for i := 0; i < 50; i++ {
		got := randomAuthString(16)
		if len(got) != 16 {
			t.Fatalf("len = %d, want 16", len(got))
		}
		for _, r := range got {
			// Verify URL-safe alphabet membership; reject ambiguous chars.
			if r == '0' || r == 'O' || r == '1' || r == 'l' || r == 'I' {
				t.Fatalf("ambiguous char %q in %q", r, got)
			}
		}
		if _, dup := seen[got]; dup {
			t.Fatalf("duplicate auth string at iter %d — crypto/rand is suspect", i)
		}
		seen[got] = struct{}{}
	}
}

func tptr(t time.Time) *time.Time { return &t }
