package sub

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/runtime"
)

// ErrNotFound is returned by Build when sub_id doesn't resolve to a
// user. Handlers convert to 404.
var ErrNotFound = errors.New("sub: not found")

// NodeLookup is a tiny interface so the sub package doesn't depend
// on the full node service. main wires an adapter over node.Service.
type NodeLookup interface {
	GetNode(ctx context.Context, id int64) (*model.Node, error)
}

// Assembler composes a user's subscription. It caches inbound
// payloads per (nodeID, tag) for inboundTTL to avoid hammering each
// node when many users share the same inbound.
type Assembler struct {
	users     *repository.UserRepo
	ownership *repository.ClientOwnershipRepo
	nodes     NodeLookup
	rt        *runtime.Manager
	log       *slog.Logger

	inboundTTL time.Duration

	mu       sync.Mutex
	inbCache map[string]inboundCacheEntry // key = nodeID|tag
}

type inboundCacheEntry struct {
	exp     time.Time
	inbound *runtime.Inbound
}

// New constructs an Assembler. inboundTTL <= 0 picks the default of
// 15s — enough to absorb a thundering-herd refresh of the same
// subscription without the cache itself becoming a staleness source.
func New(users *repository.UserRepo, ownership *repository.ClientOwnershipRepo, nodes NodeLookup, rt *runtime.Manager, lg *slog.Logger, inboundTTL time.Duration) *Assembler {
	if inboundTTL <= 0 {
		inboundTTL = 15 * time.Second
	}
	return &Assembler{
		users:      users,
		ownership:  ownership,
		nodes:      nodes,
		rt:         rt,
		log:        lg.With(slog.String("component", "sub.assembler")),
		inboundTTL: inboundTTL,
		inbCache:   make(map[string]inboundCacheEntry),
	}
}

// SubscriptionData is what handlers receive: every assembled link
// plus the aggregated UserInfo header values.
type SubscriptionData struct {
	User      *model.User
	Links     []Link
	UserInfo  UserInfo
	RemarkFmt string
}

// Build resolves subID and assembles every link the matching user
// owns. Inbound-fetch failures for individual nodes do NOT abort the
// build — affected ownerships are skipped and logged.
func (a *Assembler) Build(ctx context.Context, subID string, remarkFmt string) (*SubscriptionData, error) {
	if remarkFmt == "" {
		remarkFmt = "-ieo"
	}
	user, err := a.users.GetBySubID(ctx, subID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}
	ownerships, err := a.ownership.ListByUser(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	data := &SubscriptionData{User: user, RemarkFmt: remarkFmt}

	for i := range ownerships {
		o := &ownerships[i]
		if !o.Enabled {
			continue
		}
		node, err := a.nodes.GetNode(ctx, o.NodeID)
		if err != nil || node == nil {
			a.log.Warn("node lookup failed", slog.Int64("node_id", o.NodeID))
			continue
		}
		in, err := a.fetchInbound(ctx, o.NodeID, o.InboundTag)
		if err != nil {
			a.log.Warn("inbound fetch failed",
				slog.Int64("node_id", o.NodeID),
				slog.String("tag", o.InboundTag),
				slog.String("error", err.Error()),
			)
			continue
		}
		client, ok := findClientByEmail(in, o.ClientEmail)
		if !ok {
			a.log.Warn("client not on inbound", slog.String("email", o.ClientEmail))
			continue
		}
		remark := formatRemark(remarkFmt, node.Name, in.Remark, in.Tag, client.Email)
		url := BuildLink(node.Host, in.Port, in, client, remark)
		if url == "" {
			continue
		}
		data.Links = append(data.Links, Link{
			URL: url, Protocol: in.Protocol, Remark: remark,
			Inbound: in, Client: client, NodeID: node.ID,
		})

		// UserInfo aggregates each client's lifetime counters; for v1
		// we surface the panel-reported up/down sum.
		data.UserInfo.UploadBytes += client.TotalGB // bytes (3x-ui quirk)
		// We rely on the most recent ClientTraffic if we have it:
		for _, ct := range in.ClientStats {
			if ct.Email == client.Email {
				data.UserInfo.UploadBytes = ct.Up
				data.UserInfo.DownloadBytes = ct.Down
				data.UserInfo.TotalBytes = ct.Total
				break
			}
		}
		if o.ExpiresAt != nil && (data.UserInfo.ExpiresAt.IsZero() || o.ExpiresAt.Before(data.UserInfo.ExpiresAt)) {
			data.UserInfo.ExpiresAt = *o.ExpiresAt
		}
	}
	return data, nil
}

// FormatBase64 returns newline-joined links, base64-encoded — the
// classic subscription payload accepted by every Xray client.
func (a *Assembler) FormatBase64(d *SubscriptionData) string {
	if d == nil || len(d.Links) == 0 {
		return ""
	}
	urls := make([]string, len(d.Links))
	for i, l := range d.Links {
		urls[i] = l.URL
	}
	return base64.StdEncoding.EncodeToString([]byte(strings.Join(urls, "\n")))
}

// FormatJSON returns the structured form clients consume as a
// "subscription URL config". Minimal v1 — just the URLs and per-
// link metadata. Group 9.5 can grow this into the full Xray client
// schema later.
func (a *Assembler) FormatJSON(d *SubscriptionData) ([]byte, error) {
	if d == nil {
		return []byte("[]"), nil
	}
	type linkOut struct {
		Protocol string `json:"protocol"`
		URL      string `json:"url"`
		Remark   string `json:"remark"`
	}
	out := make([]linkOut, len(d.Links))
	for i, l := range d.Links {
		out[i] = linkOut{Protocol: l.Protocol, URL: l.URL, Remark: l.Remark}
	}
	return json.Marshal(out)
}

// UserInfoHeader returns the value of the Subscription-Userinfo
// header per the de-facto convention used by V2RayN / Clash. Zero
// fields are emitted as 0 / omitted as appropriate.
func (a *Assembler) UserInfoHeader(d *SubscriptionData) string {
	if d == nil {
		return ""
	}
	u := d.UserInfo
	s := fmt.Sprintf("upload=%d; download=%d; total=%d", u.UploadBytes, u.DownloadBytes, u.TotalBytes)
	if u.HasExpiry() {
		s += fmt.Sprintf("; expire=%d", u.ExpiresAt.Unix())
	}
	return s
}

// fetchInbound returns the inbound for (nodeID, tag), serving from a
// short-TTL cache when possible.
func (a *Assembler) fetchInbound(ctx context.Context, nodeID int64, tag string) (*runtime.Inbound, error) {
	key := fmt.Sprintf("%d|%s", nodeID, tag)
	a.mu.Lock()
	if e, ok := a.inbCache[key]; ok && time.Now().Before(e.exp) {
		a.mu.Unlock()
		return e.inbound, nil
	}
	a.mu.Unlock()

	r, err := a.rt.Get(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	in, err := r.GetInbound(ctx, tag)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	a.inbCache[key] = inboundCacheEntry{exp: time.Now().Add(a.inboundTTL), inbound: in}
	a.mu.Unlock()
	return in, nil
}

// findClientByEmail parses inbound.Settings and pulls the matching
// client.
func findClientByEmail(in *runtime.Inbound, email string) (*runtime.Client, bool) {
	if in == nil || in.Settings == "" {
		return nil, false
	}
	var s runtime.InboundSettings
	if err := json.Unmarshal([]byte(in.Settings), &s); err != nil {
		return nil, false
	}
	for i := range s.Clients {
		if s.Clients[i].Email == email {
			return &s.Clients[i], true
		}
	}
	return nil, false
}

// formatRemark substitutes the spec into a human-readable label.
// Spec convention (matches 3x-ui's remarkModel setting):
// the first rune is the separator (typically '-'); each subsequent
// rune is a single-letter token expanded from this set:
//
//	i — inbound remark
//	e — client email
//	o — node name
//	t — inbound tag
//
// Example: "-ieo" → "<inboundRemark> - <email> - <node>" (with
// missing pieces dropped). Empty spec defaults to "-ieo".
func formatRemark(spec, nodeName, inboundRemark, inboundTag, clientEmail string) string {
	if spec == "" {
		spec = "-ieo"
	}
	runes := []rune(spec)
	sep := " - "
	tokens := runes
	if len(runes) > 1 {
		sep = " " + string(runes[0]) + " "
		tokens = runes[1:]
	}
	parts := make([]string, 0, len(tokens))
	for _, r := range tokens {
		switch r {
		case 'i':
			if inboundRemark != "" {
				parts = append(parts, inboundRemark)
			}
		case 'e':
			if clientEmail != "" {
				parts = append(parts, clientEmail)
			}
		case 'o':
			if nodeName != "" {
				parts = append(parts, nodeName)
			}
		case 't':
			if inboundTag != "" {
				parts = append(parts, inboundTag)
			}
		}
	}
	return strings.Join(parts, sep)
}
