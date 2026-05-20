// Package sub assembles user-facing subscriptions: resolve a public
// sub_id to a user, walk that user's ClientOwnership rows, fetch the
// matching inbound configs from each node, and render the result as
// a base64 / JSON / Clash payload.
package sub

import (
	"time"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/runtime"
)

// Link is one renderable proxy link assembled by the link builders.
// It carries everything the format layer (base64, JSON, Clash, sing-box,
// SIP008) needs.
//
// WireGuard variant: Client is nil, WGPeer is set, and the panel-side
// server keypair is reachable via Inbound.Settings → WGSettings. The
// renderers branch on Protocol == "wireguard".
type Link struct {
	URL      string // canonical link URL — e.g. "vless://uuid@host:port?..."
	Protocol string // matches runtime/3x-ui protocol enum
	Remark   string // visible label as configured by the remark model
	Host     string // outward-facing hostname / IP of the node
	Port     int    // inbound's listen port
	Inbound  *runtime.Inbound
	Client   *runtime.Client
	WGPeer   *WGPeerView // populated only when Protocol == "wireguard"
	NodeID   int64
}

// WGPeerView is the read-only WG peer info the renderers consume.
// PrivateKey is already decrypted (the assembler does that once,
// behind closed doors). ServerPublicKey is read from the inbound's
// WGSettings.SecretKey via DerivePublic.
type WGPeerView struct {
	PrivateKey      string
	PublicKey       string
	ServerPublicKey string
	AllocatedIP     string // "10.0.0.2"
	MTU             int    // inbound-level
}

// UserInfo aggregates the numbers we report in the
// Subscription-Userinfo HTTP header. The portal app uses these for
// its "remaining bytes / days" display.
type UserInfo struct {
	UploadBytes   int64
	DownloadBytes int64
	TotalBytes    int64
	ExpiresAt     time.Time // zero = no global expiry
}

// HasExpiry reports whether ExpiresAt was set.
func (u UserInfo) HasExpiry() bool { return !u.ExpiresAt.IsZero() }

// ownershipRow is a slim alias so the sub package doesn't drag in
// all of model in its public types.
type ownershipRow = model.ClientOwnership
