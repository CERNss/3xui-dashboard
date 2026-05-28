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

func TestValidateIdempotentPurchaseRejectsDifferentRequest(t *testing.T) {
	nodeID := int64(7)
	existing := &model.Order{
		UserID:                 1,
		PlanID:                 2,
		PaymentMethod:          model.PaymentMethodBalance,
		ProvisioningNodeID:     &nodeID,
		ProvisioningInboundTag: "vless-1",
	}
	same := PurchaseInput{
		UserID:     1,
		PlanID:     2,
		NodeID:     7,
		InboundTag: "vless-1",
	}
	if err := validateIdempotentPurchase(existing, same, model.PaymentMethodBalance); err != nil {
		t.Fatalf("same purchase rejected: %v", err)
	}

	cases := []struct {
		name string
		in   PurchaseInput
	}{
		{"other user", PurchaseInput{UserID: 9, PlanID: 2, NodeID: 7, InboundTag: "vless-1"}},
		{"other plan", PurchaseInput{UserID: 1, PlanID: 3, NodeID: 7, InboundTag: "vless-1"}},
		{"other node", PurchaseInput{UserID: 1, PlanID: 2, NodeID: 8, InboundTag: "vless-1"}},
		{"other inbound", PurchaseInput{UserID: 1, PlanID: 2, NodeID: 7, InboundTag: "trojan-1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateIdempotentPurchase(existing, tc.in, model.PaymentMethodBalance); !errors.Is(err, ErrIdempotencyConflict) {
				t.Fatalf("err = %v, want ErrIdempotencyConflict", err)
			}
		})
	}
}

func TestValidateIdempotentPaymentPurchaseRejectsDifferentProvider(t *testing.T) {
	nodeID := int64(7)
	existing := &model.Order{
		UserID:                 1,
		PlanID:                 2,
		PaymentMethod:          "stripe",
		ProvisioningNodeID:     &nodeID,
		ProvisioningInboundTag: "vless-1",
	}
	in := PurchaseViaPaymentInput{
		UserID:     1,
		PlanID:     2,
		NodeID:     7,
		InboundTag: "vless-1",
		Provider:   "alipay",
	}
	if err := validateIdempotentPaymentPurchase(existing, in); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("err = %v, want ErrIdempotencyConflict", err)
	}
}

func TestNormalizeProvisioningPoolFieldsConvertsAllowedProtocols(t *testing.T) {
	got := normalizeProvisioningPoolFields(map[string]any{
		"allowed_protocols": []any{" VLESS ", "trojan", "vless", ""},
		"enabled":           true,
	})
	protocols, ok := got["allowed_protocols"].(model.StringSlice)
	if !ok {
		t.Fatalf("allowed_protocols type = %T, want model.StringSlice", got["allowed_protocols"])
	}
	want := model.StringSlice{"vless", "trojan"}
	if len(protocols) != len(want) {
		t.Fatalf("allowed_protocols = %#v, want %#v", protocols, want)
	}
	for i := range want {
		if protocols[i] != want[i] {
			t.Fatalf("allowed_protocols = %#v, want %#v", protocols, want)
		}
	}
	if got["enabled"] != true {
		t.Fatalf("enabled was not preserved: %#v", got)
	}
}

func TestAdvisoryLockKeyStable(t *testing.T) {
	if advisoryLockKey("vless-1") != advisoryLockKey("vless-1") {
		t.Fatal("same inbound tag should produce same lock key")
	}
	if advisoryLockKey("vless-1") == advisoryLockKey("trojan-1") {
		t.Fatal("different inbound tags unexpectedly collided")
	}
}

func TestParsePlanIDSet(t *testing.T) {
	got := parsePlanIDSet(" 3, 1, bad, 3, 0, -2 ")
	if len(got) != 2 || !got[1] || !got[3] {
		t.Fatalf("parsePlanIDSet returned %#v, want ids 1 and 3", got)
	}
	if empty := parsePlanIDSet("bad, 0, "); empty != nil {
		t.Fatalf("empty parse = %#v, want nil", empty)
	}
}
