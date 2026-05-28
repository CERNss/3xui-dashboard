package runtime

import "context"

// NodeRuntime is the dashboard-side surface for talking to one
// upstream 3x-ui panel. Every method takes a Context so callers can
// thread per-request deadlines and cancellation through.
//
// The interface is the *only* thing handlers / services should depend
// on. The concrete *Remote lives behind it so tests can supply a
// fake.
type NodeRuntime interface {
	// NodeID is the dashboard's internal id of the node this runtime
	// is bound to (i.e. nodes.id from Postgres). Useful for logging
	// + cache eviction.
	NodeID() int64

	// ---- Server ----------------------------------------------------------
	Probe(ctx context.Context) (*Status, error)
	RestartXray(ctx context.Context) error

	// ---- Inbounds --------------------------------------------------------
	ListInbounds(ctx context.Context) ([]Inbound, error)
	GetInbound(ctx context.Context, tag string) (*Inbound, error)
	AddInbound(ctx context.Context, in *Inbound) (*Inbound, error)
	UpdateInbound(ctx context.Context, tag string, in *Inbound) (*Inbound, error)
	UpdateInboundByID(ctx context.Context, id int64, in *Inbound) (*Inbound, error)
	DeleteInbound(ctx context.Context, tag string) error
	SetInboundEnable(ctx context.Context, tag string, enable bool) error

	// ---- Clients ---------------------------------------------------------
	// AddClient appends client to the named inbound via /clients/add.
	AddClient(ctx context.Context, inboundTag string, client Client) error
	// UpdateClient mutates the client identified by Email on the
	// named inbound via /clients/update/:email.
	UpdateClient(ctx context.Context, inboundTag string, client Client) error
	// DeleteClientByEmail removes the client by its email handle.
	// Idempotent: a missing client is treated as success.
	DeleteClientByEmail(ctx context.Context, inboundTag, email string) error

	// ---- Traffic ---------------------------------------------------------
	GetClientTraffic(ctx context.Context, email string) (*ClientTraffic, error)
	FetchTrafficSnapshot(ctx context.Context) (*TrafficSnapshot, error)

	ResetClientTraffic(ctx context.Context, inboundTag, email string) error
	ResetInboundTraffic(ctx context.Context, inboundTag string) error
	ResetAllClientTraffics(ctx context.Context, inboundTag string) error
	ResetAllTraffics(ctx context.Context) error
}
