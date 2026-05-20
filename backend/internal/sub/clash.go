package sub

import (
	"strings"

	"github.com/cern/3xui-dashboard/internal/runtime"
)

// clashNode renders one (Inbound, Client) pair into the
// map-shape a Clash YAML proxy entry expects. Returns nil + false
// when the protocol can't be represented (e.g. dokodemo-door).
//
// Reuses the parsers from links.go (parseStreamSettings, ssObj, etc.)
// so the source-of-truth for streamSettings layout stays in one place.
func clashNode(host string, port int, in *runtime.Inbound, c *runtime.Client, remark string) (map[string]any, bool) {
	switch strings.ToLower(in.Protocol) {
	case "vless":
		return clashVLESS(host, port, in, c, remark), true
	case "vmess":
		return clashVMess(host, port, in, c, remark), true
	case "trojan":
		return clashTrojan(host, port, in, c, remark), true
	case "shadowsocks":
		return clashShadowsocks(host, port, in, c, remark), true
	case "hysteria", "hysteria2":
		return clashHysteria2(host, port, in, c, remark), true
	default:
		return nil, false
	}
}

// ---- Hysteria 2 -----------------------------------------------------------
//
// Mihomo schema: https://wiki.metacubex.one/config/proxies/hysteria2/.
// The `password` field on the proxy entry is what the fork stores in
// client.Auth; SNI + skip-cert-verify come from streamSettings.
func clashHysteria2(host string, port int, in *runtime.Inbound, c *runtime.Client, remark string) map[string]any {
	if c.Auth == "" {
		return nil
	}
	ss := parseStreamSettings(in.StreamSettings)
	tls, _ := ss["tlsSettings"].(map[string]any)
	sni, _ := tls["serverName"].(string)
	allowInsecure, _ := tls["allowInsecure"].(bool)

	node := map[string]any{
		"name":     remark,
		"type":     "hysteria2",
		"server":   host,
		"port":     port,
		"password": c.Auth,
		"alpn":     []string{"h3"},
	}
	if sni != "" {
		node["sni"] = sni
	}
	if allowInsecure {
		node["skip-cert-verify"] = true
	}
	return node
}

// ---- VLESS -----------------------------------------------------------------

func clashVLESS(host string, port int, in *runtime.Inbound, c *runtime.Client, remark string) map[string]any {
	ss := parseStreamSettings(in.StreamSettings)
	network := stringFrom(ss, "network", "tcp")
	security := stringFrom(ss, "security", "none")

	node := map[string]any{
		"name":              remark,
		"type":              "vless",
		"server":            host,
		"port":              port,
		"uuid":              c.ID,
		"network":           network,
		"udp":               true,
		"client-fingerprint": "chrome",
	}

	if c.Flow != "" {
		node["flow"] = c.Flow
	}

	switch security {
	case "tls", "xtls":
		node["tls"] = true
		if tls, ok := ssObj(ss, "tlsSettings"); ok {
			if sni := stringFrom(tls, "serverName", ""); sni != "" {
				node["servername"] = sni
			}
			if alpn := stringSliceFrom(tls, "alpn"); len(alpn) > 0 {
				node["alpn"] = alpn
			}
		}
	case "reality":
		node["tls"] = true
		if r, ok := ssObj(ss, "realitySettings"); ok {
			opts := map[string]any{}
			if pbk := stringFrom(r, "publicKey", ""); pbk != "" {
				opts["public-key"] = pbk
			}
			if sid := firstString(r, "shortIds"); sid != "" {
				opts["short-id"] = sid
			}
			if srv := firstString(r, "serverNames"); srv != "" {
				node["servername"] = srv
			}
			if fp := stringFrom(r, "fingerprint", "chrome"); fp != "" {
				node["client-fingerprint"] = fp
			}
			if len(opts) > 0 {
				node["reality-opts"] = opts
			}
		}
	}

	clashAttachTransport(node, network, ss)
	return node
}

// ---- VMess -----------------------------------------------------------------

func clashVMess(host string, port int, in *runtime.Inbound, c *runtime.Client, remark string) map[string]any {
	ss := parseStreamSettings(in.StreamSettings)
	network := stringFrom(ss, "network", "tcp")
	security := stringFrom(ss, "security", "none")

	node := map[string]any{
		"name":    remark,
		"type":    "vmess",
		"server":  host,
		"port":    port,
		"uuid":    c.ID,
		"alterId": 0, // modern VMess AEAD requires alterId=0
		"cipher":  "auto",
		"network": network,
		"udp":     true,
	}

	if security == "tls" || security == "xtls" {
		node["tls"] = true
		if tls, ok := ssObj(ss, "tlsSettings"); ok {
			if sni := stringFrom(tls, "serverName", ""); sni != "" {
				node["servername"] = sni
			}
			if alpn := stringSliceFrom(tls, "alpn"); len(alpn) > 0 {
				node["alpn"] = alpn
			}
		}
	}

	clashAttachTransport(node, network, ss)
	return node
}

// ---- Trojan ----------------------------------------------------------------

func clashTrojan(host string, port int, in *runtime.Inbound, c *runtime.Client, remark string) map[string]any {
	ss := parseStreamSettings(in.StreamSettings)
	network := stringFrom(ss, "network", "tcp")

	node := map[string]any{
		"name":             remark,
		"type":             "trojan",
		"server":           host,
		"port":             port,
		"password":         c.Password,
		"network":          network,
		"udp":              true,
		"skip-cert-verify": false,
	}

	if tls, ok := ssObj(ss, "tlsSettings"); ok {
		if sni := stringFrom(tls, "serverName", ""); sni != "" {
			node["sni"] = sni
		}
		if alpn := stringSliceFrom(tls, "alpn"); len(alpn) > 0 {
			node["alpn"] = alpn
		}
	}

	clashAttachTransport(node, network, ss)
	return node
}

// ---- Shadowsocks -----------------------------------------------------------

func clashShadowsocks(host string, port int, in *runtime.Inbound, c *runtime.Client, remark string) map[string]any {
	// 3x-ui stores method + password on the inbound's settings JSON,
	// not the client. parseClientFromSS does this dance in links.go;
	// we read the same fields directly.
	method, password := parseShadowsocksAuth(in, c)
	return map[string]any{
		"name":     remark,
		"type":     "ss",
		"server":   host,
		"port":     port,
		"cipher":   method,
		"password": password,
		"udp":      true,
	}
}

// ---- Transport attachment --------------------------------------------------

// clashAttachTransport adds network-specific keys (ws-opts, grpc-opts,
// h2-opts) to node based on the parsed streamSettings.
func clashAttachTransport(node map[string]any, network string, ss map[string]any) {
	switch network {
	case "ws":
		if ws, ok := ssObj(ss, "wsSettings"); ok {
			opts := map[string]any{}
			if p := stringFrom(ws, "path", ""); p != "" {
				opts["path"] = p
			}
			if h := wsHost(ws); h != "" {
				opts["headers"] = map[string]any{"Host": h}
			}
			if len(opts) > 0 {
				node["ws-opts"] = opts
			}
		}
	case "grpc":
		if g, ok := ssObj(ss, "grpcSettings"); ok {
			opts := map[string]any{}
			if sn := stringFrom(g, "serviceName", ""); sn != "" {
				opts["grpc-service-name"] = sn
			}
			if len(opts) > 0 {
				node["grpc-opts"] = opts
			}
		}
	case "h2":
		if h, ok := ssObj(ss, "httpSettings"); ok {
			opts := map[string]any{}
			if hosts := stringSliceFrom(h, "host"); len(hosts) > 0 {
				opts["host"] = hosts
			}
			if p := stringFrom(h, "path", ""); p != "" {
				opts["path"] = p
			}
			if len(opts) > 0 {
				node["h2-opts"] = opts
			}
		}
	case "httpupgrade":
		if h, ok := ssObj(ss, "httpupgradeSettings"); ok {
			opts := map[string]any{}
			if p := stringFrom(h, "path", ""); p != "" {
				opts["path"] = p
			}
			if hh := stringFrom(h, "host", ""); hh != "" {
				opts["host"] = hh
			}
			if len(opts) > 0 {
				node["http-upgrade-opts"] = opts
			}
		}
	}
}

// parseShadowsocksAuth extracts the SS cipher + password from
// in.Settings + the client. 3x-ui pulls method from the inbound's
// settings.method and password from the client struct (or settings
// for single-user inbounds).
func parseShadowsocksAuth(in *runtime.Inbound, c *runtime.Client) (method, password string) {
	method = "aes-256-gcm" // safe fallback
	settings := parseStreamSettings(in.Settings)
	if m := stringFrom(settings, "method", ""); m != "" {
		method = m
	}
	password = c.Password
	if password == "" {
		// Single-user SS inbounds store the password on the inbound.
		password = stringFrom(settings, "password", "")
	}
	return method, password
}
