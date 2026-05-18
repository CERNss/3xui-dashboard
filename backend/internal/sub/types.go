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
// It carries everything the format layer (base64, JSON, Clash) needs.
type Link struct {
	URL      string // canonical link URL — e.g. "vless://uuid@host:port?..."
	Protocol string // matches runtime/3x-ui protocol enum
	Remark   string // visible label as configured by the remark model
	Inbound  *runtime.Inbound
	Client   *runtime.Client
	NodeID   int64
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
