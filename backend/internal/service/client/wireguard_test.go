package client

import (
	"testing"
)

func TestAllocateIP_PicksLowestFree(t *testing.T) {
	got, err := allocateIP(map[string]struct{}{})
	if err != nil {
		t.Fatalf("allocateIP empty: %v", err)
	}
	// .0 is network, .1 is gateway → first free is .2
	if got != "10.0.0.2" {
		t.Errorf("got %s, want 10.0.0.2 (first usable host)", got)
	}
}

func TestAllocateIP_SkipsTaken(t *testing.T) {
	taken := map[string]struct{}{
		"10.0.0.2": {},
		"10.0.0.3": {},
		"10.0.0.5": {},
	}
	got, err := allocateIP(taken)
	if err != nil {
		t.Fatalf("allocateIP: %v", err)
	}
	if got != "10.0.0.4" {
		t.Errorf("got %s, want 10.0.0.4 (first gap)", got)
	}
}

func TestAllocateIP_RespectsGateway(t *testing.T) {
	// Even if .1 is not in taken, it MUST NOT be handed out.
	got, err := allocateIP(map[string]struct{}{})
	if err != nil {
		t.Fatalf("allocateIP: %v", err)
	}
	if got == "10.0.0.1" {
		t.Fatal("allocator handed out gateway address 10.0.0.1")
	}
}

func TestAllocateIP_Exhausted(t *testing.T) {
	taken := map[string]struct{}{}
	// Fill every usable host in /24 (254 addresses).
	for i := 2; i <= 255; i++ {
		taken[ipv4(i)] = struct{}{}
	}
	if _, err := allocateIP(taken); err == nil {
		t.Fatal("expected exhausted error, got nil")
	}
}

func TestFirstIPOfCIDR(t *testing.T) {
	cases := []struct {
		in        string
		want      string
		shouldErr bool
	}{
		{"10.0.0.42/32", "10.0.0.42", false},
		{"10.0.0.42", "10.0.0.42", false},
		{"10.0.0.0/24", "10.0.0.0", false},
		{"not-an-ip", "", true},
	}
	for _, tc := range cases {
		got, ok := firstIPOfCIDR(tc.in)
		if tc.shouldErr {
			if ok {
				t.Errorf("firstIPOfCIDR(%q) = %q,%v, want error", tc.in, got, ok)
			}
			continue
		}
		if !ok || got != tc.want {
			t.Errorf("firstIPOfCIDR(%q) = %q,%v, want %q,true", tc.in, got, ok, tc.want)
		}
	}
}

func ipv4(last int) string {
	return "10.0.0." + itoa(last)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [4]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
