package billing

import (
	"errors"
	"testing"

	"github.com/cern/3xui-dashboard/internal/model"
)

func TestNormalizePlan_OK(t *testing.T) {
	p := &model.Plan{Name: "  Pro 30d  ", PriceCents: 5000, DurationDays: 30, TrafficLimitBytes: 1 << 40}
	if err := normalizePlan(p); err != nil {
		t.Fatalf("normalizePlan: %v", err)
	}
	if p.Name != "Pro 30d" {
		t.Errorf("name not trimmed: %q", p.Name)
	}
}

func TestNormalizePlan_Rejects(t *testing.T) {
	cases := []struct {
		desc string
		p    model.Plan
	}{
		{"empty name", model.Plan{Name: " ", PriceCents: 10, DurationDays: 1, TrafficLimitBytes: 1}},
		{"negative price", model.Plan{Name: "x", PriceCents: -1, DurationDays: 1, TrafficLimitBytes: 1}},
		{"negative duration", model.Plan{Name: "x", PriceCents: 1, DurationDays: -1, TrafficLimitBytes: 1}},
		{"negative traffic", model.Plan{Name: "x", PriceCents: 1, DurationDays: 1, TrafficLimitBytes: -1}},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			err := normalizePlan(&c.p)
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("want ErrInvalidInput, got %v", err)
			}
		})
	}
}

func TestProvisioningTargetRoundtrip(t *testing.T) {
	cases := []struct {
		nodeID int64
		tag    string
	}{
		{1, "vless-tcp-tls"},
		{42, "shadowsocks"},
		{99999, "vless-reality-vision"},
		{1, ""}, // empty tag allowed — caller validates separately
	}
	for _, c := range cases {
		encoded := encodeProvisioningTarget(c.nodeID, c.tag)
		gotID, gotTag, err := decodeProvisioningTarget(encoded)
		if err != nil {
			t.Errorf("decode(%q): %v", encoded, err)
			continue
		}
		if gotID != c.nodeID || gotTag != c.tag {
			t.Errorf("roundtrip(%d, %q) = (%d, %q)", c.nodeID, c.tag, gotID, gotTag)
		}
	}
}

func TestProvisioningTarget_DecodeEmpty(t *testing.T) {
	id, tag, err := decodeProvisioningTarget("")
	if err != nil {
		t.Errorf("empty string should decode to zero values, got err=%v", err)
	}
	if id != 0 || tag != "" {
		t.Errorf("empty decoded to (%d, %q), want (0, \"\")", id, tag)
	}
}

func TestProvisioningTarget_DecodeBadInput(t *testing.T) {
	cases := []string{
		"not-a-target",
		"target:notanumber:foo",
		"target:42-missing-second-colon",
	}
	for _, s := range cases {
		if _, _, err := decodeProvisioningTarget(s); err == nil {
			t.Errorf("expected error on %q", s)
		}
	}
}

func TestIsUniqueViolation(t *testing.T) {
	if !isUniqueViolation(errors.New("pq: duplicate key value violates unique constraint")) {
		t.Error("missed lib/pq style message")
	}
	if !isUniqueViolation(errors.New("ERROR (SQLSTATE 23505): conflict")) {
		t.Error("missed SQLSTATE 23505")
	}
	if isUniqueViolation(errors.New("something else")) {
		t.Error("false positive on unrelated error")
	}
	if isUniqueViolation(nil) {
		t.Error("nil should be false")
	}
}
