package sub

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/cern/3xui-dashboard/internal/runtime"
)

// BuildLink dispatches to the protocol-specific builder. Returns an
// empty string when the protocol isn't supported yet (the caller
// drops it silently).
func BuildLink(host string, port int, in *runtime.Inbound, c *runtime.Client, remark string) string {
	switch strings.ToLower(in.Protocol) {
	case "vless":
		return vlessLink(host, port, in, c, remark)
	case "vmess":
		return vmessLink(host, port, in, c, remark)
	case "trojan":
		return trojanLink(host, port, in, c, remark)
	case "shadowsocks":
		return shadowsocksLink(host, port, in, c, remark)
	case "hysteria", "hysteria2":
		return hysteriaLink(host, port, in, c, remark)
	default:
		return ""
	}
}

// ---- Hysteria 2 -----------------------------------------------------------
//
// hysteria2://<auth>@<host>:<port>/?sni=<sni>&alpn=h3&insecure=0#<remark>
//
// Spec: https://hysteria.network/docs/developers/URI-Scheme/.
// The fork stores Hysteria's per-client credential in client.Auth;
// streamSettings carries the tlsSettings.serverName (SNI) and
// hysteriaSettings.{version, udpIdleTimeout}. v1 only emits version 2
// links — v1-emitting nodes get a warning and an empty link.
func hysteriaLink(host string, port int, in *runtime.Inbound, c *runtime.Client, remark string) string {
	if c.Auth == "" {
		return ""
	}
	ss := parseStreamSettings(in.StreamSettings)
	hys, _ := ss["hysteriaSettings"].(map[string]any)
	version, _ := hys["version"].(float64)
	if version != 0 && version != 2 {
		return ""
	}
	tls, _ := ss["tlsSettings"].(map[string]any)
	sni, _ := tls["serverName"].(string)

	q := url.Values{}
	q.Set("alpn", "h3")
	q.Set("insecure", "0")
	if sni != "" {
		q.Set("sni", sni)
	}
	return fmt.Sprintf(
		"hysteria2://%s@%s:%d/?%s#%s",
		url.QueryEscape(c.Auth),
		host, port,
		q.Encode(),
		url.QueryEscape(remark),
	)
}

// ---- VLESS ----------------------------------------------------------------
//
// vless://<id>@<host>:<port>?type=<net>&security=<sec>&... #<remark>
//
// The query string carries the transport (type), TLS (security), and
// transport-specific fields parsed from streamSettings. Supports tcp /
// ws / grpc / reality / xhttp at a level sufficient for 99% of v1
// installations; exotic combos pass the raw fields through.
func vlessLink(host string, port int, in *runtime.Inbound, c *runtime.Client, remark string) string {
	if c.ID == "" {
		return ""
	}
	q := url.Values{}
	ss := parseStreamSettings(in.StreamSettings)

	network := stringFrom(ss, "network", "tcp")
	q.Set("type", network)

	security := stringFrom(ss, "security", "none")
	q.Set("security", security)

	if c.Flow != "" {
		q.Set("flow", c.Flow)
	}

	switch security {
	case "tls", "xtls":
		if tls, ok := ssObj(ss, "tlsSettings"); ok {
			if sni := stringFrom(tls, "serverName", ""); sni != "" {
				q.Set("sni", sni)
			}
			if alpn := stringSliceFrom(tls, "alpn"); len(alpn) > 0 {
				q.Set("alpn", strings.Join(alpn, ","))
			}
			if fp := stringFrom(tls, "fingerprint", ""); fp != "" {
				q.Set("fp", fp)
			}
		}
	case "reality":
		if r, ok := ssObj(ss, "realitySettings"); ok {
			if pbk := stringFrom(r, "publicKey", ""); pbk != "" {
				q.Set("pbk", pbk)
			}
			if sid := firstString(r, "shortIds"); sid != "" {
				q.Set("sid", sid)
			}
			if srv := firstString(r, "serverNames"); srv != "" {
				q.Set("sni", srv)
			}
			if fp := stringFrom(r, "fingerprint", "chrome"); fp != "" {
				q.Set("fp", fp)
			}
		}
	}

	switch network {
	case "ws":
		if ws, ok := ssObj(ss, "wsSettings"); ok {
			if p := stringFrom(ws, "path", ""); p != "" {
				q.Set("path", p)
			}
			if hostHdr := wsHost(ws); hostHdr != "" {
				q.Set("host", hostHdr)
			}
		}
	case "grpc":
		if g, ok := ssObj(ss, "grpcSettings"); ok {
			if sn := stringFrom(g, "serviceName", ""); sn != "" {
				q.Set("serviceName", sn)
			}
			if mode := stringFrom(g, "multiMode", ""); mode == "true" {
				q.Set("mode", "multi")
			}
		}
	}

	u := url.URL{
		Scheme:   "vless",
		User:     url.User(c.ID),
		Host:     fmt.Sprintf("%s:%d", host, port),
		RawQuery: q.Encode(),
		Fragment: remark,
	}
	return u.String()
}

// ---- VMess ----------------------------------------------------------------
//
// vmess://base64(JSON{...}) — V2RayN spec.
func vmessLink(host string, port int, in *runtime.Inbound, c *runtime.Client, remark string) string {
	if c.ID == "" {
		return ""
	}
	ss := parseStreamSettings(in.StreamSettings)
	obj := map[string]any{
		"v":    "2",
		"ps":   remark,
		"add":  host,
		"port": port,
		"id":   c.ID,
		"aid":  0,
		"scy":  defaultStr(c.Security, "auto"),
		"net":  stringFrom(ss, "network", "tcp"),
		"type": "none",
		"tls":  stringFrom(ss, "security", "none"),
	}
	if obj["net"] == "ws" {
		if ws, ok := ssObj(ss, "wsSettings"); ok {
			if p := stringFrom(ws, "path", ""); p != "" {
				obj["path"] = p
			}
			if h := wsHost(ws); h != "" {
				obj["host"] = h
			}
		}
	}
	if obj["tls"] == "tls" {
		if t, ok := ssObj(ss, "tlsSettings"); ok {
			if sni := stringFrom(t, "serverName", ""); sni != "" {
				obj["sni"] = sni
			}
		}
	}
	b, _ := json.Marshal(obj)
	return "vmess://" + base64URLNoPad(b)
}

// ---- Trojan ---------------------------------------------------------------
//
// trojan://<password>@<host>:<port>?security=tls&sni=...#remark
func trojanLink(host string, port int, in *runtime.Inbound, c *runtime.Client, remark string) string {
	if c.Password == "" {
		return ""
	}
	q := url.Values{}
	ss := parseStreamSettings(in.StreamSettings)
	if sec := stringFrom(ss, "security", ""); sec != "" {
		q.Set("security", sec)
	}
	if t, ok := ssObj(ss, "tlsSettings"); ok {
		if sni := stringFrom(t, "serverName", ""); sni != "" {
			q.Set("sni", sni)
		}
	}
	u := url.URL{
		Scheme:   "trojan",
		User:     url.User(c.Password),
		Host:     fmt.Sprintf("%s:%d", host, port),
		RawQuery: q.Encode(),
		Fragment: remark,
	}
	return u.String()
}

// ---- Shadowsocks ----------------------------------------------------------
//
// ss://base64(method:password)@host:port#remark
// Method is read from inbound.Settings.method, defaults to chacha20-
// ietf-poly1305.
func shadowsocksLink(host string, port int, in *runtime.Inbound, c *runtime.Client, remark string) string {
	if c.Password == "" {
		return ""
	}
	method := "chacha20-ietf-poly1305"
	var settings map[string]any
	_ = json.Unmarshal([]byte(in.Settings), &settings)
	if m, ok := settings["method"].(string); ok && m != "" {
		method = m
	}
	userinfo := base64URLNoPad([]byte(method + ":" + c.Password))
	return fmt.Sprintf("ss://%s@%s:%d#%s", userinfo, host, port, url.PathEscape(remark))
}
