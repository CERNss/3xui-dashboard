// Package netsafe provides an SSRF-guarded dialer used by the node
// runtime transport (talking to admin-configured 3x-ui panels) and
// the webhook delivery transport (talking to admin-configured external
// URLs).
//
// The guard refuses to dial loopback, link-local, private (RFC1918,
// RFC4193, RFC6598/CGNAT), multicast, and unspecified addresses by
// default, so a misconfigured webhook URL cannot be used to probe
// internal infrastructure or the metadata service.
//
// Callers that legitimately need to reach a private host — typically
// node-runtime traffic to a homelab 3x-ui install on 10.0.0.0/8 —
// attach a sentinel to the context with WithAllowPrivate(); the
// dialer detects it and skips the check for that single dial.
package netsafe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"
)

type allowPrivateKey struct{}

// WithAllowPrivate returns a context whose dials may target private,
// loopback, or link-local addresses. Use only for dials initiated by
// trusted, admin-configured destinations (node panels), not user-
// provided URLs (webhook targets without the allow_private flag).
func WithAllowPrivate(ctx context.Context) context.Context {
	return context.WithValue(ctx, allowPrivateKey{}, true)
}

// IsAllowPrivate reports whether the context carries the
// allow-private sentinel.
func IsAllowPrivate(ctx context.Context) bool {
	v, _ := ctx.Value(allowPrivateKey{}).(bool)
	return v
}

// ErrPrivateAddress is returned by the dialer when SSRF protection
// refuses a host. Tests assert on errors.Is.
var ErrPrivateAddress = errors.New("netsafe: refused dial to non-public address")

// DialerOptions tunes the underlying net.Dialer. Zero values pick
// safe defaults (10s connect timeout, 30s keepalive).
type DialerOptions struct {
	Timeout   time.Duration
	KeepAlive time.Duration
	Resolver  *net.Resolver // nil = net.DefaultResolver
}

// DialContext is the function shape an *http.Transport expects.
type DialContext = func(ctx context.Context, network, addr string) (net.Conn, error)

// NewDialContext returns a DialContext that performs DNS resolution
// itself and rejects any address that fails the policy. The DNS
// lookup is done up front so a host with a public + private A record
// is rejected on the private record.
func NewDialContext(opts DialerOptions) DialContext {
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Second
	}
	if opts.KeepAlive == 0 {
		opts.KeepAlive = 30 * time.Second
	}
	if opts.Resolver == nil {
		opts.Resolver = net.DefaultResolver
	}
	d := &net.Dialer{Timeout: opts.Timeout, KeepAlive: opts.KeepAlive, Resolver: opts.Resolver}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("netsafe: split %q: %w", addr, err)
		}

		if IsAllowPrivate(ctx) {
			return d.DialContext(ctx, network, addr)
		}

		// If the host is already a literal IP, validate directly.
		if ip := net.ParseIP(host); ip != nil {
			if !IsPublic(ip) {
				return nil, fmt.Errorf("%w: %s", ErrPrivateAddress, ip)
			}
			return d.DialContext(ctx, network, addr)
		}

		// Resolve and require every returned address to be public.
		ips, err := opts.Resolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("netsafe: resolve %q: %w", host, err)
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("netsafe: %q has no IPs", host)
		}
		for _, ip := range ips {
			if !IsPublic(ip.IP) {
				return nil, fmt.Errorf("%w: %s resolves to %s", ErrPrivateAddress, host, ip.IP)
			}
		}
		// Dial the first resolved IP directly to defeat last-second
		// DNS rebinding races.
		return d.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
	}
}

// IsPublic reports whether ip is routable on the public Internet
// (per the policy this package enforces). Exported for tests.
func IsPublic(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() ||
		ip.IsUnspecified() {
		return false
	}
	if v4 := ip.To4(); v4 != nil {
		// CGNAT 100.64.0.0/10.
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return false
		}
		// "This network" 0.0.0.0/8.
		if v4[0] == 0 {
			return false
		}
		// Future-use 240.0.0.0/4 incl. broadcast 255.255.255.255.
		if v4[0] >= 240 {
			return false
		}
	}
	return true
}

// NewHTTPTransport returns an *http.Transport that uses the guarded
// dialer. Callers who need additional tweaks (TLS config, proxies)
// can copy the result and mutate.
func NewHTTPTransport(opts DialerOptions) *http.Transport {
	t := &http.Transport{
		DialContext:           NewDialContext(opts),
		MaxIdleConns:          50,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
	}
	return t
}

// MustParsePort is a tiny helper used by callers that want to fail
// fast if a port string is malformed. Returns 0 on parse error so
// callers can compare for invalid input.
func MustParsePort(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}
