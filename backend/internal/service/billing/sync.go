package billing

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/service/client"
)

// SyncSummary reports what a plan→subscriber mirror pass did.
type SyncSummary struct {
	PlanID    int64    `json:"plan_id"`
	Users     int      `json:"users"`
	Refreshed int      `json:"refreshed"`
	Added     int      `json:"added"`
	Removed   int      `json:"removed"`
	Errors    []string `json:"errors,omitempty"`
}

// SyncPlanSubscribers mirrors the CURRENT plan content onto every
// active subscriber, so editing a plan updates users who already
// bought it ("保存即全量镜像"):
//
//   - traffic / IP limits are refreshed on the panel client + the
//     ownership row (expiry, credentials, and traffic counters are
//     untouched);
//   - for pool-backed plans, targets added to the pool are
//     provisioned for each subscriber (inheriting their current
//     expiry), and ownerships whose target row was DELETED from the
//     pool are torn down. A target that is merely disabled or at
//     capacity pauses new placements but never removes existing
//     clients;
//   - duration changes are deliberately not retroactive — they apply
//     on the next renewal.
//
// Active subscriber rows = (enabled OR disabled_by_quota) and not
// expired. Per-row failures are collected into Summary.Errors, never
// fatal — re-running the sync is idempotent and heals stragglers.
func (s *Service) SyncPlanSubscribers(ctx context.Context, planID int64) (*SyncSummary, error) {
	plan, err := s.plans.Get(ctx, planID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, ErrPlanNotFound
	}
	if s.ownership == nil {
		return nil, fmt.Errorf("billing.SyncPlanSubscribers: ownership repo not wired")
	}

	rows, err := s.ownership.ListByPlan(ctx, planID)
	if err != nil {
		return nil, err
	}

	summary := &SyncSummary{PlanID: planID, Errors: []string{}}
	now := time.Now().UTC()

	byUser := map[int64][]model.ClientOwnership{}
	for _, row := range rows {
		active := (row.Enabled || row.DisabledByQuota) &&
			(row.ExpiresAt == nil || row.ExpiresAt.After(now))
		if !active {
			continue
		}
		byUser[row.UserID] = append(byUser[row.UserID], row)
	}

	// Removal keys on the pool's target ROWS (deleted target = tear
	// down), independent of the capacity/preflight-filtered candidate
	// list used for additions.
	var poolSet map[provisioningTarget]struct{}
	if plan.ProvisioningPoolID != nil && s.pools != nil {
		pool, err := s.pools.Get(ctx, *plan.ProvisioningPoolID)
		if err != nil {
			return nil, err
		}
		if pool != nil {
			poolSet = make(map[provisioningTarget]struct{}, len(pool.Targets))
			for _, t := range pool.Targets {
				poolSet[provisioningTarget{NodeID: t.NodeID, InboundTag: t.InboundTag}] = struct{}{}
			}
		}
	}

	userIDs := make([]int64, 0, len(byUser))
	for uid := range byUser {
		userIDs = append(userIDs, uid)
	}
	sort.Slice(userIDs, func(i, j int) bool { return userIDs[i] < userIDs[j] })

	for _, uid := range userIDs {
		urows := byUser[uid]
		summary.Users++

		// 1) Refresh limits on every active ownership.
		for _, row := range urows {
			if err := s.client.RefreshClientLimits(ctx, row.NodeID, row.InboundTag, row.ClientEmail, plan.TrafficLimitBytes, plan.IPLimit); err != nil {
				summary.Errors = append(summary.Errors,
					fmt.Sprintf("user=%d node=%d tag=%q refresh: %v", uid, row.NodeID, row.InboundTag, err))
				continue
			}
			summary.Refreshed++
		}

		if poolSet == nil {
			continue // non-pool plan: nothing to mirror beyond limits
		}

		existing := make(map[provisioningTarget]struct{}, len(urows))
		// The user's inherited expiry for newly added targets: nil
		// (non-expiring) if any row is non-expiring, otherwise the
		// latest expiry across their rows.
		var maxExpiry *time.Time
		nonExpiring := false
		for _, row := range urows {
			existing[provisioningTarget{NodeID: row.NodeID, InboundTag: row.InboundTag}] = struct{}{}
			if row.ExpiresAt == nil {
				nonExpiring = true
			} else if maxExpiry == nil || row.ExpiresAt.After(*maxExpiry) {
				v := *row.ExpiresAt
				maxExpiry = &v
			}
		}
		if nonExpiring {
			maxExpiry = nil
		}

		// 2) Provision targets the pool has that the user lacks.
		desired, err := s.resolveProvisioningTargets(ctx, plan, uid)
		if err != nil {
			summary.Errors = append(summary.Errors, fmt.Sprintf("user=%d resolve targets: %v", uid, err))
		} else {
			pid := plan.ID
			for _, tgt := range desired {
				if _, ok := existing[tgt]; ok {
					continue
				}
				params := client.PlanParams{
					PlanID:            &pid,
					DurationDays:      0,
					TrafficLimitBytes: plan.TrafficLimitBytes,
					IPLimit:           plan.IPLimit,
					ExpiresAtOverride: maxExpiry,
				}
				if _, err := s.client.ProvisionClient(ctx, uid, tgt.NodeID, tgt.InboundTag, params); err != nil {
					summary.Errors = append(summary.Errors,
						fmt.Sprintf("user=%d node=%d tag=%q add: %v", uid, tgt.NodeID, tgt.InboundTag, err))
					continue
				}
				summary.Added++
			}
		}

		// 3) Tear down ownerships whose target row left the pool.
		for _, row := range urows {
			if _, ok := poolSet[provisioningTarget{NodeID: row.NodeID, InboundTag: row.InboundTag}]; ok {
				continue
			}
			if err := s.client.DeleteClient(ctx, row.NodeID, row.InboundTag, row.ClientEmail); err != nil {
				summary.Errors = append(summary.Errors,
					fmt.Sprintf("user=%d node=%d tag=%q remove: %v", uid, row.NodeID, row.InboundTag, err))
				continue
			}
			summary.Removed++
		}
	}

	s.log.Info("plan subscribers synced",
		slog.Int64("plan_id", planID),
		slog.Int("users", summary.Users),
		slog.Int("refreshed", summary.Refreshed),
		slog.Int("added", summary.Added),
		slog.Int("removed", summary.Removed),
		slog.Int("errors", len(summary.Errors)),
	)
	return summary, nil
}
