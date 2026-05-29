package billing

import (
	"testing"
)

func TestSortTargetsForLock_Deterministic(t *testing.T) {
	in := []provisioningTarget{
		{NodeID: 3, InboundTag: "z"},
		{NodeID: 1, InboundTag: "b"},
		{NodeID: 1, InboundTag: "a"},
		{NodeID: 2, InboundTag: "m"},
		{NodeID: 3, InboundTag: "a"},
	}
	got := sortTargetsForLock(in)
	want := []provisioningTarget{
		{NodeID: 1, InboundTag: "a"},
		{NodeID: 1, InboundTag: "b"},
		{NodeID: 2, InboundTag: "m"},
		{NodeID: 3, InboundTag: "a"},
		{NodeID: 3, InboundTag: "z"},
	}
	if len(got) != len(want) {
		t.Fatalf("len mismatch: got %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("idx %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestSortTargetsForLock_StableSecondaryKey(t *testing.T) {
	// Same nodeID, different tag — secondary key (tag asc) decides.
	in := []provisioningTarget{
		{NodeID: 7, InboundTag: "z"},
		{NodeID: 7, InboundTag: "a"},
		{NodeID: 7, InboundTag: "m"},
	}
	got := sortTargetsForLock(in)
	want := []provisioningTarget{
		{NodeID: 7, InboundTag: "a"},
		{NodeID: 7, InboundTag: "m"},
		{NodeID: 7, InboundTag: "z"},
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("idx %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestSortTargetsForLock_DoesNotMutateInput(t *testing.T) {
	in := []provisioningTarget{
		{NodeID: 2, InboundTag: "b"},
		{NodeID: 1, InboundTag: "a"},
	}
	original := make([]provisioningTarget, len(in))
	copy(original, in)
	_ = sortTargetsForLock(in)
	for i := range in {
		if in[i] != original[i] {
			t.Errorf("input mutated at idx %d: %+v -> %+v", i, original[i], in[i])
		}
	}
}

func TestDerefInt64(t *testing.T) {
	if derefInt64(nil) != 0 {
		t.Errorf("nil should return 0")
	}
	v := int64(42)
	if derefInt64(&v) != 42 {
		t.Errorf("got %d, want 42", derefInt64(&v))
	}
}
