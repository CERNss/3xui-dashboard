package traffic

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/cern/3xui-dashboard/internal/model"
)

// TestEnforceSharedQuotas_NoDepsIsNoOp verifies the deployment path
// where SetSharedQuotaDeps was never called (no pool-driven plans
// exist) is a no-op: no error, no changes, no panel calls. This is
// what protects single-target deployments from the new code path.
func TestEnforceSharedQuotas_NoDepsIsNoOp(t *testing.T) {
	s := &Service{
		log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	stats, err := s.EnforceSharedQuotas(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("EnforceSharedQuotas with nil deps returned error: %v", err)
	}
	if stats.GroupsExamined != 0 {
		t.Errorf("groups_examined = %d, want 0 when deps are nil", stats.GroupsExamined)
	}
	if stats.OwnersDisabled != 0 || stats.OwnersRestored != 0 {
		t.Errorf("expected no mutations on nil-deps run, got disabled=%d restored=%d",
			stats.OwnersDisabled, stats.OwnersRestored)
	}
	if len(stats.Errors) != 0 {
		t.Errorf("expected no errors, got %v", stats.Errors)
	}
}

// TestEnforceSharedQuotas_PartialDepsIsNoOp covers the half-wired
// path where one but not both of (plans, clients) was wired. The
// enforcement must skip rather than panic so a misconfigured
// deployment doesn't break the periodic traffic job.
func TestEnforceSharedQuotas_PartialDepsIsNoOp(t *testing.T) {
	s := &Service{
		plans: stubPlanLookup{},
		// clients intentionally left nil.
		log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	stats, err := s.EnforceSharedQuotas(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.GroupsExamined != 0 {
		t.Errorf("expected 0 groups examined when clients dep is nil, got %d", stats.GroupsExamined)
	}
}

type stubPlanLookup struct{}

func (stubPlanLookup) Get(ctx context.Context, id int64) (*model.Plan, error) { return nil, nil }
