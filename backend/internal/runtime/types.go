// Package runtime is the dashboard-side client for the upstream 3x-ui
// node API. It speaks the panel wire format directly — Bearer auth,
// {success,msg,obj} envelopes, stringified-JSON `settings` columns —
// behind a NodeRuntime interface so the rest of the codebase doesn't
// have to know about any of that.
package runtime

import "encoding/json"

// ---------------------------------------------------------------------------
// Wire types — these mirror the JSON the 3x-ui panel emits / accepts.
// They are intentionally distinct from internal/model: this is the API
// contract, not the dashboard's persistence schema.
// ---------------------------------------------------------------------------

// Inbound is one inbound config as the 3x-ui panel reports / accepts.
// Settings / StreamSettings / Sniffing carry stringified JSON; the
// client decodes them as needed.
//
// Tag is the stable identifier. The numeric ID is unstable across
// recreations — never key on it; resolve via tag→id cache.
type Inbound struct {
	ID                   int64           `json:"id"`
	Up                   int64           `json:"up"`
	Down                 int64           `json:"down"`
	Total                int64           `json:"total"`
	AllTime              int64           `json:"allTime"`
	Remark               string          `json:"remark"`
	Enable               bool            `json:"enable"`
	ExpiryTime           int64           `json:"expiryTime"`
	TrafficReset         string          `json:"trafficReset"`
	LastTrafficResetTime int64           `json:"lastTrafficResetTime"`
	ClientStats          []ClientTraffic `json:"clientStats"`
	Listen               string          `json:"listen"`
	Port                 int             `json:"port"`
	Protocol             string          `json:"protocol"`
	Settings             string          `json:"settings"`        // stringified JSON
	StreamSettings       string          `json:"streamSettings"`  // stringified JSON
	Tag                  string          `json:"tag"`
	Sniffing             string          `json:"sniffing"`        // stringified JSON
}

// IsWireguard reports whether this inbound carries a WG peer
// list rather than the standard clients[] envelope. The
// MHSanaei/3x-ui fork registers the protocol verbatim — case
// matters; the runtime uses lowercase as the panel emits it.
func (i *Inbound) IsWireguard() bool { return i.Protocol == "wireguard" }

// InboundSettings is the parsed structure inside Inbound.Settings.
// Clients live here; the dashboard reads/writes them to perform
// client mutations.
type InboundSettings struct {
	Clients    []Client          `json:"clients,omitempty"`
	Decryption string            `json:"decryption,omitempty"`
	Fallbacks  []json.RawMessage `json:"fallbacks,omitempty"`
	// Everything else is preserved opaque so we round-trip cleanly.
	Extras map[string]json.RawMessage `json:"-"`
}

// Client mirrors the per-client object stored in InboundSettings.Clients.
// TotalGB is the traffic cap in BYTES despite the name (3x-ui quirk).
// ExpiryTime is unix milliseconds; 0 = never; negative = relative-
// from-first-use (the absolute value is the duration in ms).
//
// Per-protocol credential field placement:
//   - VLESS / VMess        → ID (UUID)
//   - Trojan / Shadowsocks → Password (random hex)
//   - Hysteria             → Auth (random URL-safe string)
type Client struct {
	ID         string `json:"id,omitempty"`         // VLESS/VMess UUID
	Password   string `json:"password,omitempty"`   // Trojan / Shadowsocks
	Auth       string `json:"auth,omitempty"`       // Hysteria
	Security   string `json:"security,omitempty"`
	Flow       string `json:"flow,omitempty"`
	Email      string `json:"email"`
	LimitIP    int    `json:"limitIp,omitempty"`
	TotalGB    int64  `json:"totalGB,omitempty"`    // BYTES, not GB
	ExpiryTime int64  `json:"expiryTime,omitempty"` // ms
	Enable     bool   `json:"enable"`
	TgID       int64  `json:"tgId,omitempty"`
	SubID      string `json:"subId,omitempty"`
	Comment    string `json:"comment,omitempty"`
	Reset      int    `json:"reset,omitempty"`
	CreatedAt  int64  `json:"created_at,omitempty"`
	UpdatedAt  int64  `json:"updated_at,omitempty"`
}

// WGSettings is the parsed shape of an Inbound.Settings JSON
// string when Protocol == "wireguard". Captured 2026-05-20 from
// a live MHSanaei/3x-ui node via T0 probing (see
// openspec/changes/add-protocol-wireguard/notes/3xui-wg-api.md).
type WGSettings struct {
	MTU         int      `json:"mtu"`
	SecretKey   string   `json:"secretKey"`
	Peers       []WGPeer `json:"peers"`
	NoKernelTun bool     `json:"noKernelTun"`
}

// WGPeer is one element of WGSettings.Peers. AllowedIPs is a
// list of CIDR strings ("10.0.0.2/32") rather than IPs alone.
// KeepAlive is the persistent-keepalive interval in seconds; 0
// disables it.
type WGPeer struct {
	PrivateKey string   `json:"privateKey"`
	PublicKey  string   `json:"publicKey"`
	PSK        string   `json:"psk,omitempty"`
	AllowedIPs []string `json:"allowedIPs"`
	KeepAlive  int      `json:"keepAlive"`
}

// HysteriaStreamConfig is the parsed shape of
// Inbound.StreamSettings → hysteriaSettings. Captured 2026-05-20.
// `Version` is 2 for hysteria2; the older v1 protocol stores
// transport bits differently and is treated as out-of-scope
// (warning logged on encounter).
type HysteriaStreamConfig struct {
	Version        int    `json:"version"`
	UDPIdleTimeout int    `json:"udpIdleTimeout"` // seconds; default 60
	Auth           string `json:"auth,omitempty"` // inbound-wide pre-shared password (rarely used)
}

// ClientTraffic is what the panel returns inside Inbound.ClientStats
// and from /clients/traffic/:email. Up/Down are cumulative bytes
// since the last reset.
type ClientTraffic struct {
	ID         int64  `json:"id"`
	InboundID  int64  `json:"inboundId"`
	Enable     bool   `json:"enable"`
	Email      string `json:"email"`
	UUID       string `json:"uuid,omitempty"`
	SubID      string `json:"subId,omitempty"`
	Up         int64  `json:"up"`
	Down       int64  `json:"down"`
	AllTime    int64  `json:"allTime"`
	ExpiryTime int64  `json:"expiryTime"`
	Total      int64  `json:"total"`
	Reset      int    `json:"reset"`
	LastOnline int64  `json:"lastOnline,omitempty"` // unix seconds
}

// ---------------------------------------------------------------------------
// Server status (GET /panel/api/server/status)
// ---------------------------------------------------------------------------

// Status is the subset of /server/status the dashboard cares about.
// Fields the panel emits but we don't read are intentionally absent
// so unknown additions land in DecoderUnused without surfacing.
type Status struct {
	CPU      float64       `json:"cpu"`
	CPUCores int           `json:"cpuCores"`
	Mem      MemStat       `json:"mem"`
	Xray     XrayStat      `json:"xray"`
	Uptime   int64         `json:"uptime"` // seconds
	Loads    []float64     `json:"loads"`
	NetIO    NetCounters   `json:"netIO"`
	PublicIP PublicIPStats `json:"publicIP"`
}

type MemStat struct {
	Current int64 `json:"current"`
	Total   int64 `json:"total"`
}

type XrayStat struct {
	State    string `json:"state"`
	ErrorMsg string `json:"errorMsg"`
	Version  string `json:"version"`
}

type NetCounters struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

type PublicIPStats struct {
	IPv4 string `json:"ipv4"`
	IPv6 string `json:"ipv6"`
}

// MemPercent returns Mem.Current / Mem.Total as a percentage (0-100).
// Returns 0 when Total is zero to avoid divide-by-zero.
func (s Status) MemPercent() float64 {
	if s.Mem.Total == 0 {
		return 0
	}
	return float64(s.Mem.Current) * 100 / float64(s.Mem.Total)
}

// ---------------------------------------------------------------------------
// Traffic snapshot (composed by the runtime layer)
// ---------------------------------------------------------------------------

// TrafficSnapshot is the per-call result of FetchTrafficSnapshot.
// It aggregates the data needed by the traffic-collection job into a
// single struct so the caller doesn't have to make three separate
// calls.
type TrafficSnapshot struct {
	// Inbounds with their ClientStats populated.
	Inbounds []Inbound
	// OnlineEmails is best-effort: empty if the panel rejected the
	// call.
	OnlineEmails []string
	// LastOnlineByEmail is best-effort: nil if rejected.
	LastOnlineByEmail map[string]int64
}
