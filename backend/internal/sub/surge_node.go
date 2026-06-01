package sub

import (
	"fmt"
	"strings"

	"github.com/cern/3xui-dashboard/internal/runtime"
)

// surgeNode renders one (Inbound, Client) pair into a Surge [Proxy] line
// body — everything after "name = ". Returns ok=false for protocols
// Surge can't speak (notably VLESS, and WireGuard which needs its own
// section), so the caller skips them: a Surge subscription naturally
// contains only the Surge-supported subset of a user's nodes.
//
// Reuses the streamSettings parsers from streamsettings.go so the layout
// source-of-truth stays shared with the Clash/sing-box renderers.
func surgeNode(host string, port int, in *runtime.Inbound, c *runtime.Client) (string, bool) {
	switch strings.ToLower(in.Protocol) {
	case "vmess":
		return surgeVMess(host, port, in, c), true
	case "trojan":
		return surgeTrojan(host, port, in, c), true
	case "shadowsocks":
		return surgeShadowsocks(host, port, in, c), true
	case "hysteria", "hysteria2":
		return surgeHysteria2(host, port, in, c)
	default:
		return "", false
	}
}

func surgeVMess(host string, port int, in *runtime.Inbound, c *runtime.Client) string {
	ss := parseStreamSettings(in.StreamSettings)
	network := stringFrom(ss, "network", "tcp")
	security := stringFrom(ss, "security", "none")

	parts := []string{"vmess", host, fmt.Sprintf("%d", port), "username=" + c.ID, "vmess-aead=true"}
	if security == "tls" || security == "xtls" {
		parts = append(parts, "tls=true")
		if tls, ok := ssObj(ss, "tlsSettings"); ok {
			if sni := stringFrom(tls, "serverName", ""); sni != "" {
				parts = append(parts, "sni="+sni)
			}
		}
	}
	parts = append(parts, surgeWS(network, ss)...)
	return strings.Join(parts, ", ")
}

func surgeTrojan(host string, port int, in *runtime.Inbound, c *runtime.Client) string {
	ss := parseStreamSettings(in.StreamSettings)
	network := stringFrom(ss, "network", "tcp")

	parts := []string{"trojan", host, fmt.Sprintf("%d", port), "password=" + c.Password}
	if tls, ok := ssObj(ss, "tlsSettings"); ok {
		if sni := stringFrom(tls, "serverName", ""); sni != "" {
			parts = append(parts, "sni="+sni)
		}
		if insecure, _ := tls["allowInsecure"].(bool); insecure {
			parts = append(parts, "skip-cert-verify=true")
		}
	}
	parts = append(parts, surgeWS(network, ss)...)
	return strings.Join(parts, ", ")
}

func surgeShadowsocks(host string, port int, in *runtime.Inbound, c *runtime.Client) string {
	method, password := parseShadowsocksAuth(in, c)
	parts := []string{"ss", host, fmt.Sprintf("%d", port), "encrypt-method=" + method, "password=" + password}
	return strings.Join(parts, ", ")
}

func surgeHysteria2(host string, port int, in *runtime.Inbound, c *runtime.Client) (string, bool) {
	if c.Auth == "" {
		return "", false
	}
	ss := parseStreamSettings(in.StreamSettings)
	tls, _ := ssObj(ss, "tlsSettings")
	parts := []string{"hysteria2", host, fmt.Sprintf("%d", port), "password=" + c.Auth}
	if sni := stringFrom(tls, "serverName", ""); sni != "" {
		parts = append(parts, "sni="+sni)
	}
	if insecure, _ := tls["allowInsecure"].(bool); insecure {
		parts = append(parts, "skip-cert-verify=true")
	}
	return strings.Join(parts, ", "), true
}

// surgeWS appends Surge ws-transport key=values when the inbound uses
// WebSocket transport.
func surgeWS(network string, ss map[string]any) []string {
	if network != "ws" {
		return nil
	}
	ws, ok := ssObj(ss, "wsSettings")
	if !ok {
		return nil
	}
	out := []string{"ws=true"}
	if p := stringFrom(ws, "path", ""); p != "" {
		out = append(out, "ws-path="+p)
	}
	if h := wsHost(ws); h != "" {
		out = append(out, "ws-headers=Host:"+h)
	}
	return out
}
