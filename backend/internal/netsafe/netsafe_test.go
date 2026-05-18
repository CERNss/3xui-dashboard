package netsafe

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
)

func TestIsPublic(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"8.8.8.8", true},
		{"1.1.1.1", true},
		{"2606:4700:4700::1111", true}, // Cloudflare v6
		{"127.0.0.1", false},
		{"10.0.0.1", false},
		{"172.16.5.5", false},
		{"172.31.0.1", false},
		{"192.168.1.1", false},
		{"169.254.169.254", false}, // AWS metadata
		{"100.64.0.1", false},      // CGNAT
		{"100.127.255.255", false}, // CGNAT upper
		{"0.0.0.0", false},
		{"255.255.255.255", false}, // broadcast
		{"240.0.0.1", false},       // reserved
		{"::1", false},
		{"fc00::1", false},  // ULA
		{"fe80::1", false},  // link-local
		{"ff02::1", false},  // multicast
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if ip == nil {
			t.Fatalf("bad test IP %q", c.ip)
		}
		if got := IsPublic(ip); got != c.want {
			t.Errorf("IsPublic(%s) = %v, want %v", c.ip, got, c.want)
		}
	}
}

func TestDialContext_RefusesLoopbackLiteral(t *testing.T) {
	dial := NewDialContext(DialerOptions{})
	_, err := dial(context.Background(), "tcp", "127.0.0.1:1")
	if err == nil {
		t.Fatal("expected refusal, got nil")
	}
	if !errors.Is(err, ErrPrivateAddress) {
		t.Errorf("expected ErrPrivateAddress, got %v", err)
	}
}

func TestDialContext_RefusesPrivateLiteral(t *testing.T) {
	dial := NewDialContext(DialerOptions{})
	_, err := dial(context.Background(), "tcp", "10.5.5.5:80")
	if err == nil {
		t.Fatal("expected refusal, got nil")
	}
	if !errors.Is(err, ErrPrivateAddress) {
		t.Errorf("expected ErrPrivateAddress, got %v", err)
	}
}

func TestDialContext_RefusesAWSMetadata(t *testing.T) {
	dial := NewDialContext(DialerOptions{})
	_, err := dial(context.Background(), "tcp", "169.254.169.254:80")
	if err == nil || !errors.Is(err, ErrPrivateAddress) {
		t.Errorf("AWS metadata not blocked: %v", err)
	}
}

func TestDialContext_AllowPrivateContextSkipsCheck(t *testing.T) {
	// With allow-private the dialer should not error on the policy
	// check. We still don't actually connect — point at a closed
	// port so the real connect fails with a non-policy error.
	dial := NewDialContext(DialerOptions{})
	ctx := WithAllowPrivate(context.Background())
	_, err := dial(ctx, "tcp", "127.0.0.1:1")
	if err == nil {
		t.Fatal("expected connect failure, got nil")
	}
	if errors.Is(err, ErrPrivateAddress) {
		t.Errorf("policy fired despite allow-private: %v", err)
	}
	// Should be a connection refused / network error, not an
	// ErrPrivateAddress.
	if !strings.Contains(err.Error(), "connect") &&
		!strings.Contains(err.Error(), "refused") &&
		!strings.Contains(err.Error(), "timeout") {
		t.Logf("note: error was %q — acceptable as long as it is not the policy error", err)
	}
}
