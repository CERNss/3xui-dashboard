package webhook

import (
	"encoding/hex"
	"testing"
)

func TestPatternsMatch(t *testing.T) {
	cases := []struct {
		patterns []string
		event    string
		want     bool
	}{
		{[]string{"*"}, "anything.you.want", true},
		{[]string{"node.online"}, "node.online", true},
		{[]string{"node.online"}, "node.offline", false},
		{[]string{"node.*"}, "node.online", true},
		{[]string{"node.*"}, "node.offline", true},
		{[]string{"node.*"}, "order.created", false},
		{[]string{"order.*", "node.online"}, "node.online", true},
		{[]string{"order.*", "node.online"}, "user.registered", false},
		{nil, "node.online", false},
	}
	for _, c := range cases {
		if got := patternsMatch(c.patterns, c.event); got != c.want {
			t.Errorf("patternsMatch(%v, %q) = %v, want %v", c.patterns, c.event, got, c.want)
		}
	}
}

func TestSign_DeterministicAndVerifiable(t *testing.T) {
	secret := "abc-secret"
	ts := "1700000000"
	body := []byte(`{"hello":"world"}`)
	a := sign(secret, ts, body)
	b := sign(secret, ts, body)
	if a != b {
		t.Errorf("sign should be deterministic: %s vs %s", a, b)
	}
	if len(a) != 64 {
		t.Errorf("sign hex len = %d, want 64 (sha256)", len(a))
	}
	if _, err := hex.DecodeString(a); err != nil {
		t.Errorf("sign output is not hex: %v", err)
	}
	// Different secret → different signature.
	if sign("other-secret", ts, body) == a {
		t.Error("different secret produced same signature")
	}
}

func TestRandomSecret_LenAndHex(t *testing.T) {
	s := randomSecret()
	if len(s) != 32 {
		t.Errorf("randomSecret len = %d, want 32", len(s))
	}
	if _, err := hex.DecodeString(s); err != nil {
		t.Errorf("randomSecret is not hex: %v", err)
	}
}
