package repository

import (
	"context"
	"testing"

	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/model"
)

// seedProvenanceFixture creates a node + user and returns both. The
// provenance tests hang ownership / ledger rows off these.
func seedProvenanceFixture(t *testing.T, db *gorm.DB) (nodeID, userID int64) {
	t.Helper()
	node := model.Node{Name: "prov-node", Scheme: "https", Host: "prov.test", Port: 443, APIToken: "t", Enabled: true}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	email := "prov@x"
	user := model.User{Email: &email, SubID: "prov-sub", Status: "active"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return node.ID, user.ID
}

func TestProvenanceRepo_RecordIsIdempotent(t *testing.T) {
	db := setupStatsDB(t)
	ctx := context.Background()
	nodeID, _ := seedProvenanceFixture(t, db)
	repo := NewProvenanceRepo(db)

	for i := 0; i < 2; i++ {
		if err := repo.RecordInbound(ctx, nodeID, "tag-a"); err != nil {
			t.Fatalf("RecordInbound #%d: %v", i, err)
		}
		if err := repo.RecordClient(ctx, nodeID, "tag-a", "mail-a"); err != nil {
			t.Fatalf("RecordClient #%d: %v", i, err)
		}
	}
	var inbounds, clients int64
	db.Model(&model.ManagedInbound{}).Count(&inbounds)
	db.Model(&model.ManagedClient{}).Count(&clients)
	if inbounds != 1 || clients != 1 {
		t.Errorf("counts after double record = %d/%d, want 1/1", inbounds, clients)
	}

	// ForgetClient on a present row, then on a missing one — both nil.
	if err := repo.ForgetClient(ctx, nodeID, "tag-a", "mail-a"); err != nil {
		t.Fatalf("ForgetClient: %v", err)
	}
	if err := repo.ForgetClient(ctx, nodeID, "tag-a", "mail-a"); err != nil {
		t.Fatalf("ForgetClient (missing): %v", err)
	}
}

func TestProvenanceRepo_ForgetInboundCascades(t *testing.T) {
	db := setupStatsDB(t)
	ctx := context.Background()
	nodeID, userID := seedProvenanceFixture(t, db)
	repo := NewProvenanceRepo(db)

	// Ledger rows + an ownership + a wg_peer hanging off it.
	if err := repo.RecordInbound(ctx, nodeID, "wg-1"); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordClient(ctx, nodeID, "wg-1", "peer-mail"); err != nil {
		t.Fatal(err)
	}
	own := model.ClientOwnership{UserID: userID, NodeID: nodeID, InboundTag: "wg-1", ClientEmail: "peer-mail", Enabled: true}
	if err := db.Create(&own).Error; err != nil {
		t.Fatalf("create ownership: %v", err)
	}
	peer := model.WGPeer{ClientOwnershipID: own.ID, InboundID: 7, PublicKey: "pk", PrivateKeyEncrypted: []byte("sealed"), AllocatedIP: "10.0.0.2"}
	if err := db.Create(&peer).Error; err != nil {
		t.Fatalf("create wg peer: %v", err)
	}
	// A second inbound that must survive untouched.
	if err := repo.RecordInbound(ctx, nodeID, "keep-1"); err != nil {
		t.Fatal(err)
	}

	if err := repo.ForgetInbound(ctx, nodeID, "wg-1"); err != nil {
		t.Fatalf("ForgetInbound: %v", err)
	}

	counts := map[string]int64{}
	for name, m := range map[string]any{
		"managed_inbounds": &model.ManagedInbound{},
		"managed_clients":  &model.ManagedClient{},
		"ownerships":       &model.ClientOwnership{},
		"wg_peers":         &model.WGPeer{},
	} {
		var c int64
		if err := db.Model(m).Count(&c).Error; err != nil {
			t.Fatalf("count %s: %v", name, err)
		}
		counts[name] = c
	}
	if counts["managed_inbounds"] != 1 { // keep-1 survives
		t.Errorf("managed_inbounds = %d, want 1", counts["managed_inbounds"])
	}
	if counts["managed_clients"] != 0 || counts["ownerships"] != 0 {
		t.Errorf("client rows survived forget: clients=%d ownerships=%d", counts["managed_clients"], counts["ownerships"])
	}
	if counts["wg_peers"] != 0 {
		t.Errorf("wg_peers = %d, want 0 (FK cascade off client_ownerships)", counts["wg_peers"])
	}
}

func TestProvenanceRepo_RenameInboundTagCascadesAllTables(t *testing.T) {
	db := setupStatsDB(t)
	ctx := context.Background()
	nodeID, userID := seedProvenanceFixture(t, db)
	repo := NewProvenanceRepo(db)

	if err := repo.RecordInbound(ctx, nodeID, "old"); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordClient(ctx, nodeID, "old", "m1"); err != nil {
		t.Fatal(err)
	}
	own := model.ClientOwnership{UserID: userID, NodeID: nodeID, InboundTag: "old", ClientEmail: "m1", Enabled: true}
	if err := db.Create(&own).Error; err != nil {
		t.Fatal(err)
	}
	pool := model.ProvisioningPool{Name: "p", Enabled: true}
	if err := db.Create(&pool).Error; err != nil {
		t.Fatal(err)
	}
	target := model.ProvisioningPoolTarget{PoolID: pool.ID, NodeID: nodeID, InboundTag: "old", Enabled: true}
	if err := db.Create(&target).Error; err != nil {
		t.Fatal(err)
	}

	// No-ops first.
	if err := repo.RenameInboundTag(ctx, nodeID, "old", ""); err != nil {
		t.Fatalf("rename to empty must no-op: %v", err)
	}
	if err := repo.RenameInboundTag(ctx, nodeID, "old", "old"); err != nil {
		t.Fatalf("rename to same must no-op: %v", err)
	}

	if err := repo.RenameInboundTag(ctx, nodeID, "old", "new"); err != nil {
		t.Fatalf("RenameInboundTag: %v", err)
	}
	for name, q := range map[string]string{
		"managed_inbounds":          `SELECT COUNT(*) FROM managed_inbounds WHERE node_id = ? AND inbound_tag = 'new'`,
		"managed_clients":           `SELECT COUNT(*) FROM managed_clients WHERE node_id = ? AND inbound_tag = 'new'`,
		"client_ownerships":         `SELECT COUNT(*) FROM client_ownerships WHERE node_id = ? AND inbound_tag = 'new'`,
		"provisioning_pool_targets": `SELECT COUNT(*) FROM provisioning_pool_targets WHERE node_id = ? AND inbound_tag = 'new'`,
	} {
		var c int64
		if err := db.Raw(q, nodeID).Scan(&c).Error; err != nil {
			t.Fatalf("count %s: %v", name, err)
		}
		if c != 1 {
			t.Errorf("%s renamed rows = %d, want 1", name, c)
		}
	}
}

func TestProvenanceRepo_ListClientStatesMergesLedgerAndOwnership(t *testing.T) {
	db := setupStatsDB(t)
	ctx := context.Background()
	nodeID, userID := seedProvenanceFixture(t, db)
	repo := NewProvenanceRepo(db)

	// Three shapes: managed+owned, managed-only (unbound direct add),
	// owned-only (pre-ledger legacy row — post-backfill this is rare
	// but the merge must still surface it as unmanaged).
	if err := repo.RecordClient(ctx, nodeID, "t1", "both"); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientOwnership{UserID: userID, NodeID: nodeID, InboundTag: "t1", ClientEmail: "both", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordClient(ctx, nodeID, "t1", "ledger-only"); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ClientOwnership{UserID: userID, NodeID: nodeID, InboundTag: "t1", ClientEmail: "own-only", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}

	rows, err := repo.ListClientStates(ctx)
	if err != nil {
		t.Fatalf("ListClientStates: %v", err)
	}
	byEmail := map[string]ClientStateRow{}
	for _, r := range rows {
		byEmail[r.ClientEmail] = r
	}
	if len(byEmail) != 3 {
		t.Fatalf("states = %d, want 3 (%+v)", len(byEmail), rows)
	}
	if r := byEmail["both"]; !r.Managed || r.UserID == nil || *r.UserID != userID {
		t.Errorf("both: %+v, want managed + user", r)
	}
	if r := byEmail["ledger-only"]; !r.Managed || r.UserID != nil {
		t.Errorf("ledger-only: %+v, want managed + nil user", r)
	}
	if r := byEmail["own-only"]; r.Managed || r.UserID == nil {
		t.Errorf("own-only: %+v, want unmanaged + user", r)
	}
}
