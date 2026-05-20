package sub

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cern/3xui-dashboard/internal/runtime"
	"github.com/cern/3xui-dashboard/internal/service/wgcrypto"
)

// decodeWGSettings parses an Inbound.Settings JSON string into
// the typed WG settings struct. Empty input is treated as
// "no peers / no key" rather than an error so a half-configured
// inbound surfaces as "no WG link" instead of a hard failure.
func decodeWGSettings(s string) (*runtime.WGSettings, error) {
	if s == "" {
		return &runtime.WGSettings{}, nil
	}
	var out runtime.WGSettings
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, fmt.Errorf("WG settings unmarshal: %w", err)
	}
	return &out, nil
}

// deriveServerPublic derives the inbound's public key from
// the panel-stored secretKey (which is the WG server private key).
// Returns an error if secretKey is empty — that's a misconfigured
// inbound and the renderer should drop the link rather than
// publish a no-key WG config.
func deriveServerPublic(secretKey string) (string, error) {
	if secretKey == "" {
		return "", fmt.Errorf("WG inbound has empty secretKey")
	}
	return wgcrypto.DerivePublic(secretKey)
}

// BuildWGConf returns the .conf body for a WG peer — the
// wg-quick / wireguard-android compatible ini text. Newlines are
// LF only; the format is whitespace-insensitive but consistent
// output is friendlier to grep + diff.
//
// The Endpoint line uses the node's outward-facing host + the
// inbound's listen port — this matches what the node's
// reverse-proxied WG socket is reachable at, NOT
// 0.0.0.0 from the inbound JSON.
func BuildWGConf(l Link) string {
	if l.WGPeer == nil {
		return ""
	}
	p := l.WGPeer
	var b strings.Builder
	b.WriteString("[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", p.PrivateKey)
	fmt.Fprintf(&b, "Address = %s/32\n", p.AllocatedIP)
	if p.MTU > 0 {
		fmt.Fprintf(&b, "MTU = %d\n", p.MTU)
	}
	b.WriteString("DNS = 1.1.1.1, 8.8.8.8\n")
	b.WriteString("\n[Peer]\n")
	fmt.Fprintf(&b, "PublicKey = %s\n", p.ServerPublicKey)
	fmt.Fprintf(&b, "Endpoint = %s:%d\n", l.Host, l.Port)
	b.WriteString("AllowedIPs = 0.0.0.0/0, ::/0\n")
	b.WriteString("PersistentKeepalive = 25\n")
	return b.String()
}

// BuildWGConfZip packages multiple WG configs into a single ZIP
// archive — one .conf per link, named after the peer's remark
// (sanitized to ASCII printable, fallback to public-key prefix).
// The result is a self-contained bundle for portal "download all
// WG configs" actions.
func BuildWGConfZip(links []Link) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	seen := map[string]int{}
	for _, l := range links {
		if l.Protocol != "wireguard" || l.WGPeer == nil {
			continue
		}
		name := safeWGFilename(l.Remark, l.WGPeer.PublicKey)
		// De-dup repeated names within the same archive.
		if n := seen[name]; n > 0 {
			seen[name] = n + 1
			name = fmt.Sprintf("%s-%d", name, n)
		} else {
			seen[name] = 1
		}
		w, err := zw.Create(name + ".conf")
		if err != nil {
			return nil, fmt.Errorf("zip create %s.conf: %w", name, err)
		}
		if _, err := w.Write([]byte(BuildWGConf(l))); err != nil {
			return nil, fmt.Errorf("zip write %s.conf: %w", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("zip close: %w", err)
	}
	return buf.Bytes(), nil
}

// safeWGFilename returns a path-safe filename, falling back to a
// publicKey prefix when remark is empty / all-stripped.
func safeWGFilename(remark, publicKey string) string {
	clean := make([]rune, 0, len(remark))
	for _, r := range remark {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-' || r == '_':
			clean = append(clean, r)
		case r == ' ':
			clean = append(clean, '-')
		}
	}
	if len(clean) == 0 {
		// Last 8 chars of the public key, dropping = padding.
		pk := strings.TrimRight(publicKey, "=")
		if len(pk) > 8 {
			pk = pk[len(pk)-8:]
		}
		return "wg-" + pk
	}
	return string(clean)
}

// clashWGNode emits a Mihomo `type: wireguard` outbound entry.
// Schema reference: https://wiki.metacubex.one/config/proxies/wireguard/.
func clashWGNode(l Link) map[string]any {
	if l.WGPeer == nil {
		return nil
	}
	p := l.WGPeer
	out := map[string]any{
		"name":        l.Remark,
		"type":        "wireguard",
		"server":      l.Host,
		"port":        l.Port,
		"ip":          p.AllocatedIP,
		"private-key": p.PrivateKey,
		"public-key":  p.ServerPublicKey,
		"udp":         true,
	}
	if p.MTU > 0 {
		out["mtu"] = p.MTU
	}
	return out
}

// singboxWGOutbound emits a sing-box `type: wireguard` outbound
// entry. Reference: https://sing-box.sagernet.org/configuration/outbound/wireguard/.
func singboxWGOutbound(l Link) map[string]any {
	if l.WGPeer == nil {
		return nil
	}
	p := l.WGPeer
	out := map[string]any{
		"type":         "wireguard",
		"tag":          l.Remark,
		"server":       l.Host,
		"server_port":  l.Port,
		"local_address": []string{p.AllocatedIP + "/32"},
		"private_key":  p.PrivateKey,
		"peer_public_key": p.ServerPublicKey,
	}
	if p.MTU > 0 {
		out["mtu"] = p.MTU
	}
	return out
}
