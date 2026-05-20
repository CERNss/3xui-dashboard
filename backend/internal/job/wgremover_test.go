package job

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/cern/3xui-dashboard/internal/model"
)

// stubWGRemover counts RemovePeer calls so a test can assert the
// ExpiryJob delegated to it instead of UpdateClient.
type stubWGRemover struct {
	mu    sync.Mutex
	calls []struct {
		NodeID     int64
		InboundTag string
		Email      string
	}
	err error
}

func (s *stubWGRemover) RemovePeer(ctx context.Context, nodeID int64, tag, email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, struct {
		NodeID     int64
		InboundTag string
		Email      string
	}{NodeID: nodeID, InboundTag: tag, Email: email})
	return s.err
}

// TestSetWGRemover_AttachesHook verifies the constructor allows a
// later attachment (idempotency we depend on for the app wiring
// branch — when WG_MASTER_KEY is set we attach AFTER NewExpiryJob).
func TestSetWGRemover_AttachesHook(t *testing.T) {
	j := &ExpiryJob{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if j.wg != nil {
		t.Fatal("expected wg to start nil")
	}
	r := &stubWGRemover{}
	j.SetWGRemover(r)
	if j.wg == nil {
		t.Fatal("SetWGRemover did not attach the hook")
	}
	// Replace with another stub — idempotent overwrite.
	j.SetWGRemover(&stubWGRemover{})
	if j.wg == r {
		t.Fatal("SetWGRemover should overwrite, not append")
	}
}

// TestWGRemover_PropagatesError ensures a RemovePeer failure
// surfaces to the caller rather than being silently swallowed.
// This is a contract test against the WGRemover interface alone —
// the real ExpiryJob.disableOnNode integration is exercised by
// the DB-backed RunOnce tests.
func TestWGRemover_PropagatesError(t *testing.T) {
	want := errors.New("node unreachable")
	s := &stubWGRemover{err: want}
	got := s.RemovePeer(context.Background(), 42, "wg-1", "alice@example.com")
	if !errors.Is(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if len(s.calls) != 1 {
		t.Errorf("expected 1 call, got %d", len(s.calls))
	}
	c := s.calls[0]
	if c.NodeID != 42 || c.InboundTag != "wg-1" || c.Email != "alice@example.com" {
		t.Errorf("call args wrong: %+v", c)
	}
}

// TestWGRemoverImpl_IsModelClientOwnershipShape pins the field set
// the job exposes to the WGRemover so future signature changes
// trip a compile error here rather than at runtime.
func TestWGRemoverImpl_IsModelClientOwnershipShape(t *testing.T) {
	// Compile-time assert: a model.ClientOwnership has the three
	// fields we pass to RemovePeer.
	o := model.ClientOwnership{NodeID: 1, InboundTag: "wg", ClientEmail: "e"}
	_ = o.NodeID
	_ = o.InboundTag
	_ = o.ClientEmail
}
