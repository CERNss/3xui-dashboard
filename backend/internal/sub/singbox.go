package sub

import (
	"strings"

	"github.com/cern/3xui-dashboard/internal/runtime"
)

// singboxOutbound renders (Inbound, Client) into a sing-box outbound
// JSON shape. Returns nil + false when the protocol isn't supported.
//
// sing-box outbound types: vless, vmess, trojan, shadowsocks. Field
// names differ from Clash (tag instead of name; uuid stays; flow stays).
func singboxOutbound(host string, port int, in *runtime.Inbound, c *runtime.Client, remark string) (map[string]any, bool) {
	switch strings.ToLower(in.Protocol) {
	case "vless":
		return singboxVLESS(host, port, in, c, remark), true
	case "vmess":
		return singboxVMess(host, port, in, c, remark), true
	case "trojan":
		return singboxTrojan(host, port, in, c, remark), true
	case "shadowsocks":
		return singboxShadowsocks(host, port, in, c, remark), true
	case "hysteria", "hysteria2":
		return singboxHysteria2(host, port, in, c, remark), true
	default:
		return nil, false
	}
}

// singboxHysteria2 emits a sing-box `type: hysteria2` outbound.
// Reference: https://sing-box.sagernet.org/configuration/outbound/hysteria2/.
// The fork stores Hysteria's per-client credential in client.Auth →
// sing-box wants it in `password`. TLS is mandatory; the fork
// rejects Hysteria inbounds without security=tls.
func singboxHysteria2(host string, port int, in *runtime.Inbound, c *runtime.Client, remark string) map[string]any {
	if c.Auth == "" {
		return nil
	}
	ss := parseStreamSettings(in.StreamSettings)
	tlsCfg, _ := ss["tlsSettings"].(map[string]any)
	sni, _ := tlsCfg["serverName"].(string)
	allowInsecure, _ := tlsCfg["allowInsecure"].(bool)

	tlsBlock := map[string]any{
		"enabled":    true,
		"alpn":       []string{"h3"},
		"server_name": sni,
		"insecure":   allowInsecure,
	}
	if sni == "" {
		// sing-box requires a non-empty server_name when enabled=true.
		// Fall back to the connect host so the config still loads.
		tlsBlock["server_name"] = host
	}

	return map[string]any{
		"type":        "hysteria2",
		"tag":         remark,
		"server":      host,
		"server_port": port,
		"password":    c.Auth,
		"tls":         tlsBlock,
	}
}

func singboxVLESS(host string, port int, in *runtime.Inbound, c *runtime.Client, remark string) map[string]any {
	ss := parseStreamSettings(in.StreamSettings)
	network := stringFrom(ss, "network", "tcp")
	security := stringFrom(ss, "security", "none")

	node := map[string]any{
		"type":        "vless",
		"tag":         remark,
		"server":      host,
		"server_port": port,
		"uuid":        c.ID,
	}
	if c.Flow != "" {
		node["flow"] = c.Flow
	}

	singboxAttachTLS(node, security, ss)
	singboxAttachTransport(node, network, ss)
	return node
}

func singboxVMess(host string, port int, in *runtime.Inbound, c *runtime.Client, remark string) map[string]any {
	ss := parseStreamSettings(in.StreamSettings)
	network := stringFrom(ss, "network", "tcp")
	security := stringFrom(ss, "security", "none")

	node := map[string]any{
		"type":           "vmess",
		"tag":            remark,
		"server":         host,
		"server_port":    port,
		"uuid":           c.ID,
		"security":       "auto",
		"alter_id":       0, // modern VMess AEAD requires alterId=0
		"global_padding": false,
	}
	singboxAttachTLS(node, security, ss)
	singboxAttachTransport(node, network, ss)
	return node
}

func singboxTrojan(host string, port int, in *runtime.Inbound, c *runtime.Client, remark string) map[string]any {
	ss := parseStreamSettings(in.StreamSettings)
	network := stringFrom(ss, "network", "tcp")
	security := stringFrom(ss, "security", "none")

	node := map[string]any{
		"type":        "trojan",
		"tag":         remark,
		"server":      host,
		"server_port": port,
		"password":    c.Password,
	}
	singboxAttachTLS(node, security, ss)
	singboxAttachTransport(node, network, ss)
	return node
}

func singboxShadowsocks(host string, port int, in *runtime.Inbound, c *runtime.Client, remark string) map[string]any {
	method, password := parseShadowsocksAuth(in, c)
	return map[string]any{
		"type":        "shadowsocks",
		"tag":         remark,
		"server":      host,
		"server_port": port,
		"method":      method,
		"password":    password,
	}
}

// ---- attachers ------------------------------------------------------------

func singboxAttachTLS(node map[string]any, security string, ss map[string]any) {
	switch security {
	case "tls", "xtls":
		tlsCfg := map[string]any{"enabled": true}
		if tls, ok := ssObj(ss, "tlsSettings"); ok {
			if sni := stringFrom(tls, "serverName", ""); sni != "" {
				tlsCfg["server_name"] = sni
			}
			if alpn := stringSliceFrom(tls, "alpn"); len(alpn) > 0 {
				tlsCfg["alpn"] = alpn
			}
		}
		node["tls"] = tlsCfg
	case "reality":
		tlsCfg := map[string]any{
			"enabled":     true,
			"utls":        map[string]any{"enabled": true, "fingerprint": "chrome"},
			"reality":     map[string]any{"enabled": true},
		}
		if r, ok := ssObj(ss, "realitySettings"); ok {
			realityCfg := tlsCfg["reality"].(map[string]any)
			if pbk := stringFrom(r, "publicKey", ""); pbk != "" {
				realityCfg["public_key"] = pbk
			}
			if sid := firstString(r, "shortIds"); sid != "" {
				realityCfg["short_id"] = sid
			}
			if srv := firstString(r, "serverNames"); srv != "" {
				tlsCfg["server_name"] = srv
			}
		}
		node["tls"] = tlsCfg
	}
}

func singboxAttachTransport(node map[string]any, network string, ss map[string]any) {
	switch network {
	case "ws":
		transport := map[string]any{"type": "ws"}
		if ws, ok := ssObj(ss, "wsSettings"); ok {
			if p := stringFrom(ws, "path", ""); p != "" {
				transport["path"] = p
			}
			if h := wsHost(ws); h != "" {
				transport["headers"] = map[string]any{"Host": h}
			}
		}
		node["transport"] = transport
	case "grpc":
		transport := map[string]any{"type": "grpc"}
		if g, ok := ssObj(ss, "grpcSettings"); ok {
			if sn := stringFrom(g, "serviceName", ""); sn != "" {
				transport["service_name"] = sn
			}
		}
		node["transport"] = transport
	case "h2":
		transport := map[string]any{"type": "http"}
		if h, ok := ssObj(ss, "httpSettings"); ok {
			if hosts := stringSliceFrom(h, "host"); len(hosts) > 0 {
				transport["host"] = hosts
			}
			if p := stringFrom(h, "path", ""); p != "" {
				transport["path"] = p
			}
		}
		node["transport"] = transport
	case "httpupgrade":
		transport := map[string]any{"type": "httpupgrade"}
		if h, ok := ssObj(ss, "httpupgradeSettings"); ok {
			if p := stringFrom(h, "path", ""); p != "" {
				transport["path"] = p
			}
			if hh := stringFrom(h, "host", ""); hh != "" {
				transport["host"] = hh
			}
		}
		node["transport"] = transport
	}
	// tcp / kcp / quic / xhttp: emit no transport block (sing-box
	// treats absent transport as raw TCP).
}
