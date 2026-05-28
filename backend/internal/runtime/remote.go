package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/netsafe"
)

// ErrTagNotFound is returned when an inbound with the given tag does
// not exist on the remote node. Callers that want idempotent delete
// behaviour can errors.Is on it.
var ErrTagNotFound = errors.New("3xui: inbound tag not found")

// ErrClientNotFound is returned when no client with the given email
// exists on the named inbound. Delete callers should treat this as
// success; update/get callers should surface it.
var ErrClientNotFound = errors.New("3xui: client not found")

// Remote is the *http.Client-backed NodeRuntime implementation.
//
// One Remote is built per node — see Manager. The tag→id cache is
// per-Remote so eviction of one node doesn't disturb others.
type Remote struct {
	nodeID   int64
	baseURL  string // e.g. "https://node.example.com:2053/admin/"
	apiToken string
	http     *http.Client
	tagCache *tagCache
	log      *slog.Logger
}

// NewRemote constructs a Remote from a node row. httpClient must
// already carry an SSRF-guarded transport; build it with
// transport.New(...) at startup and share across Remotes.
func NewRemote(node *model.Node, httpClient *http.Client, lg *slog.Logger) *Remote {
	base := buildBaseURL(node)
	return &Remote{
		nodeID:   node.ID,
		baseURL:  base,
		apiToken: node.APIToken,
		http:     httpClient,
		tagCache: newTagCache(),
		log:      lg.With(slog.Int64("node_id", node.ID), slog.String("node", node.Name)),
	}
}

// NodeID is the dashboard's internal id of the node this Remote is
// bound to.
func (r *Remote) NodeID() int64 { return r.nodeID }

// buildBaseURL composes the panel root URL. BasePath is the visible
// panel path (for example "/panel/"); an empty value uses "/panel/".
func buildBaseURL(n *model.Node) string {
	scheme := n.Scheme
	if scheme == "" {
		scheme = "https"
	}
	base := normalizeBasePath(n.BasePath)
	return fmt.Sprintf("%s://%s:%d%s", scheme, n.Host, n.Port, base)
}

// normalizeBasePath returns p with a leading "/" and trailing "/".
// Empty input is normalized to the stock 3x-ui panel path.
func normalizeBasePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || p == "/" {
		return "/panel/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if !strings.HasSuffix(p, "/") {
		p = p + "/"
	}
	return p
}

// ---------------------------------------------------------------------------
// Low-level transport helpers
// ---------------------------------------------------------------------------

func (r *Remote) url(path string) string {
	return r.baseURL + "api" + path
}

func (r *Remote) do(ctx context.Context, req *http.Request) (*Envelope, error) {
	req.Header.Set("Authorization", "Bearer "+r.apiToken)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "application/json")

	// Nodes are admin-configured destinations — allow private/loopback
	// IPs for homelab deployments.
	req = req.WithContext(netsafe.WithAllowPrivate(ctx))

	resp, err := r.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("3xui transport: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20)) // 8 MiB cap
	if err != nil {
		return nil, fmt.Errorf("3xui read body: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("3xui unauthorized (HTTP %d): check api token", resp.StatusCode)
	}
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("3xui upstream error: HTTP %d: %s", resp.StatusCode, snippet(body))
	}
	// 3x-ui returns 404 when the panel is hiding from a not-XHR caller;
	// we set the header, so 404 here is a real missing endpoint.
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("3xui endpoint not found: HTTP 404: %s", req.URL.Path)
	}

	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("3xui decode envelope: %w (body=%s)", err, snippet(body))
	}
	if !env.Success {
		return nil, &EnvelopeError{Msg: env.Msg, Payload: env.Obj, Path: req.URL.Path}
	}
	return &env, nil
}

func (r *Remote) doGet(ctx context.Context, path string) (*Envelope, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.url(path), nil)
	if err != nil {
		return nil, err
	}
	return r.do(ctx, req)
}

func (r *Remote) doForm(ctx context.Context, path string, form url.Values) (*Envelope, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url(path), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r.do(ctx, req)
}

func (r *Remote) doJSON(ctx context.Context, path string, body any) (*Envelope, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("3xui marshal body: %w", err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url(path), reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return r.do(ctx, req)
}

func (r *Remote) doPostEmpty(ctx context.Context, path string) (*Envelope, error) {
	return r.doJSON(ctx, path, nil)
}

// ---------------------------------------------------------------------------
// Server
// ---------------------------------------------------------------------------

// Probe fetches /server/status. Returns the parsed Status struct.
func (r *Remote) Probe(ctx context.Context) (*Status, error) {
	env, err := r.doGet(ctx, "/server/status")
	if err != nil {
		return nil, err
	}
	var s Status
	if err := env.DecodeObj(&s); err != nil && !errors.Is(err, ErrEmptyObj) {
		return nil, err
	}
	return &s, nil
}

// RestartXray asks the panel to restart its Xray service.
func (r *Remote) RestartXray(ctx context.Context) error {
	_, err := r.doPostEmpty(ctx, "/server/restartXrayService")
	return err
}

// ---------------------------------------------------------------------------
// Inbounds
// ---------------------------------------------------------------------------

// ListInbounds returns every inbound configured on the node, including
// clientStats. Refreshes the local tag→id cache atomically.
func (r *Remote) ListInbounds(ctx context.Context) ([]Inbound, error) {
	env, err := r.doGet(ctx, "/inbounds/list")
	if err != nil {
		return nil, err
	}
	var inbounds []Inbound
	if err := env.DecodeObj(&inbounds); err != nil {
		if errors.Is(err, ErrEmptyObj) {
			return nil, nil
		}
		return nil, err
	}
	// Atomically refresh the tag cache.
	m := make(map[string]int64, len(inbounds))
	for _, in := range inbounds {
		if in.Tag != "" {
			m[in.Tag] = in.ID
		}
	}
	r.tagCache.Replace(m)
	return inbounds, nil
}

// GetInbound returns one inbound by tag.
func (r *Remote) GetInbound(ctx context.Context, tag string) (*Inbound, error) {
	inbounds, err := r.ListInbounds(ctx)
	if err != nil {
		return nil, err
	}
	for i := range inbounds {
		if inbounds[i].Tag == tag {
			return &inbounds[i], nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrTagNotFound, tag)
}

// AddInbound creates a new inbound. The panel returns the created
// inbound (with its assigned id). The tag cache is updated in place.
func (r *Remote) AddInbound(ctx context.Context, in *Inbound) (*Inbound, error) {
	form := r.wireInbound(in)
	env, err := r.doForm(ctx, "/inbounds/add", form)
	if err != nil {
		return nil, err
	}
	var created Inbound
	if err := env.DecodeObj(&created); err != nil && !errors.Is(err, ErrEmptyObj) {
		return nil, err
	}
	if created.Tag != "" {
		r.tagCache.Set(created.Tag, created.ID)
	}
	return &created, nil
}

// UpdateInbound mutates the inbound identified by tag. The full
// settings JSON is sent; the panel replaces the row.
func (r *Remote) UpdateInbound(ctx context.Context, tag string, in *Inbound) (*Inbound, error) {
	id, err := r.resolveTagToID(ctx, tag)
	if err != nil {
		return nil, err
	}
	return r.UpdateInboundByID(ctx, id, in)
}

// UpdateInboundByID mutates the inbound by numeric id. Used by the
// WireGuard peer-mutation RMW path, where the caller already holds
// the id from a GetInbound that ran inside the same transaction
// and doesn't want a redundant /list refresh between GET and POST.
func (r *Remote) UpdateInboundByID(ctx context.Context, id int64, in *Inbound) (*Inbound, error) {
	form := r.wireInbound(in)
	env, err := r.doForm(ctx, "/inbounds/update/"+strconv.FormatInt(id, 10), form)
	if err != nil {
		return nil, err
	}
	var updated Inbound
	if err := env.DecodeObj(&updated); err != nil && !errors.Is(err, ErrEmptyObj) {
		return nil, err
	}
	return &updated, nil
}

// DeleteInbound removes the inbound identified by tag. Idempotent:
// returns nil when the tag does not exist on the node.
func (r *Remote) DeleteInbound(ctx context.Context, tag string) error {
	id, err := r.resolveTagToID(ctx, tag)
	if err != nil {
		if errors.Is(err, ErrTagNotFound) {
			return nil
		}
		return err
	}
	if _, err := r.doPostEmpty(ctx, "/inbounds/del/"+strconv.FormatInt(id, 10)); err != nil {
		return err
	}
	r.tagCache.Delete(tag)
	return nil
}

// SetInboundEnable flips just the enable bit, cheap.
func (r *Remote) SetInboundEnable(ctx context.Context, tag string, enable bool) error {
	id, err := r.resolveTagToID(ctx, tag)
	if err != nil {
		return err
	}
	form := url.Values{}
	form.Set("enable", strconv.FormatBool(enable))
	_, err = r.doForm(ctx, "/inbounds/setEnable/"+strconv.FormatInt(id, 10), form)
	return err
}

// wireInbound assembles the form values an /add / /update call
// expects. Stringified-JSON fields are passed through verbatim after
// streamSettings is run through sanitizeStreamSettingsForRemote.
func (r *Remote) wireInbound(in *Inbound) url.Values {
	form := url.Values{}
	form.Set("total", strconv.FormatInt(in.Total, 10))
	form.Set("remark", in.Remark)
	form.Set("enable", strconv.FormatBool(in.Enable))
	form.Set("expiryTime", strconv.FormatInt(in.ExpiryTime, 10))
	form.Set("listen", in.Listen)
	form.Set("port", strconv.Itoa(in.Port))
	form.Set("protocol", in.Protocol)
	form.Set("settings", in.Settings)
	form.Set("streamSettings", sanitizeStreamSettingsForRemote(in.StreamSettings))
	form.Set("tag", in.Tag)
	form.Set("sniffing", in.Sniffing)
	if in.TrafficReset != "" {
		form.Set("trafficReset", in.TrafficReset)
	}
	return form
}

// ---------------------------------------------------------------------------
// Clients
// ---------------------------------------------------------------------------

// AddClient appends client to the named inbound.
func (r *Remote) AddClient(ctx context.Context, inboundTag string, client Client) error {
	id, err := r.resolveTagToID(ctx, inboundTag)
	if err != nil {
		return err
	}

	body := struct {
		Client     Client `json:"client"`
		InboundIDs []int  `json:"inboundIds"`
	}{Client: client, InboundIDs: []int{int(id)}}
	_, err = r.doJSON(ctx, "/clients/add", body)
	return err
}

// UpdateClient mutates the client identified by Email. The real
// /clients/update route is keyed by email globally; inboundTag is kept
// in the interface because callers operate in an inbound-scoped domain.
func (r *Remote) UpdateClient(ctx context.Context, inboundTag string, client Client) error {
	_ = inboundTag
	if client.Email == "" {
		return fmt.Errorf("UpdateClient: client.Email is required")
	}
	_, err := r.doJSON(ctx, "/clients/update/"+url.PathEscape(client.Email), client)
	return err
}

// DeleteClientByEmail removes the named client. Idempotent: returns
// nil when the client is not present.
func (r *Remote) DeleteClientByEmail(ctx context.Context, inboundTag, email string) error {
	if _, err := r.resolveTagToID(ctx, inboundTag); err != nil {
		if errors.Is(err, ErrTagNotFound) {
			return nil
		}
		return err
	}
	_, err := r.doPostEmpty(ctx, "/clients/del/"+url.PathEscape(email))
	return err
}

// ---------------------------------------------------------------------------
// Traffic
// ---------------------------------------------------------------------------

// GetClientTraffic returns one client's cumulative usage by email.
func (r *Remote) GetClientTraffic(ctx context.Context, email string) (*ClientTraffic, error) {
	env, err := r.doGet(ctx, "/clients/traffic/"+url.PathEscape(email))
	if err != nil {
		return nil, err
	}
	var t ClientTraffic
	if err := env.DecodeObj(&t); err != nil {
		if errors.Is(err, ErrEmptyObj) {
			return nil, fmt.Errorf("%w: email=%q", ErrClientNotFound, email)
		}
		return nil, err
	}
	return &t, nil
}

// FetchTrafficSnapshot composes the three calls the traffic-
// collection job needs into a single result. /onlines and
// /lastOnline are best-effort — failures are logged and the snapshot
// is returned with the data we did manage to retrieve.
func (r *Remote) FetchTrafficSnapshot(ctx context.Context) (*TrafficSnapshot, error) {
	inbounds, err := r.ListInbounds(ctx)
	if err != nil {
		return nil, err
	}
	snap := &TrafficSnapshot{Inbounds: inbounds}

	if env, err := r.doPostEmpty(ctx, "/clients/onlines"); err == nil {
		var emails []string
		if err := env.DecodeObj(&emails); err != nil && !errors.Is(err, ErrEmptyObj) {
			r.log.Warn("decode /onlines obj", slog.String("error", err.Error()))
		}
		snap.OnlineEmails = emails
	} else {
		r.log.Warn("/onlines failed; snapshot proceeds without it", slog.String("error", err.Error()))
	}

	if env, err := r.doPostEmpty(ctx, "/clients/lastOnline"); err == nil {
		var lastOnline map[string]int64
		if err := env.DecodeObj(&lastOnline); err != nil && !errors.Is(err, ErrEmptyObj) {
			r.log.Warn("decode /lastOnline obj", slog.String("error", err.Error()))
		}
		snap.LastOnlineByEmail = lastOnline
	} else {
		r.log.Warn("/lastOnline failed; snapshot proceeds without it", slog.String("error", err.Error()))
	}

	return snap, nil
}

// ResetClientTraffic resets one client's up/down counters.
func (r *Remote) ResetClientTraffic(ctx context.Context, inboundTag, email string) error {
	_ = inboundTag
	_, err := r.doPostEmpty(ctx, "/clients/resetTraffic/"+url.PathEscape(email))
	return err
}

// ResetInboundTraffic resets one inbound's aggregate counters.
func (r *Remote) ResetInboundTraffic(ctx context.Context, inboundTag string) error {
	id, err := r.resolveTagToID(ctx, inboundTag)
	if err != nil {
		return err
	}
	_, err = r.doPostEmpty(ctx, fmt.Sprintf("/inbounds/%d/resetTraffic", id))
	return err
}

// ResetAllClientTraffics zeroes every client on an inbound.
func (r *Remote) ResetAllClientTraffics(ctx context.Context, inboundTag string) error {
	in, err := r.GetInbound(ctx, inboundTag)
	if err != nil {
		return err
	}
	var settings InboundSettings
	if in.Settings != "" {
		if err := json.Unmarshal([]byte(in.Settings), &settings); err != nil {
			return fmt.Errorf("decode inbound settings: %w", err)
		}
	}
	for _, c := range settings.Clients {
		if c.Email == "" {
			continue
		}
		if _, err := r.doPostEmpty(ctx, "/clients/resetTraffic/"+url.PathEscape(c.Email)); err != nil {
			return fmt.Errorf("reset client traffic %q: %w", c.Email, err)
		}
	}
	return nil
}

// ResetAllTraffics zeroes every counter on the node.
func (r *Remote) ResetAllTraffics(ctx context.Context) error {
	_, err := r.doPostEmpty(ctx, "/inbounds/resetAllTraffics")
	return err
}

// ---------------------------------------------------------------------------
// Cache helpers
// ---------------------------------------------------------------------------

// resolveTagToID looks up a tag in the per-Remote cache; on miss it
// refreshes by calling ListInbounds and retries. Returns
// ErrTagNotFound when the tag is still absent after refresh.
func (r *Remote) resolveTagToID(ctx context.Context, tag string) (int64, error) {
	if id, ok := r.tagCache.Get(tag); ok {
		return id, nil
	}
	if _, err := r.ListInbounds(ctx); err != nil {
		return 0, err
	}
	if id, ok := r.tagCache.Get(tag); ok {
		return id, nil
	}
	return 0, fmt.Errorf("%w: %q", ErrTagNotFound, tag)
}

// ---------------------------------------------------------------------------
// Misc
// ---------------------------------------------------------------------------

// snippet returns a short string suitable for log/error messages.
func snippet(b []byte) string {
	const max = 256
	s := string(b)
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}
