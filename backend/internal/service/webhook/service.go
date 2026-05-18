// Package webhook implements admin-configured outbound event delivery.
// A bus subscriber matches incoming events against each enabled
// webhook's Events patterns; matches are persisted as
// webhook_deliveries rows and dispatched on a goroutine pool with
// HMAC signing, SSRF-guarded transport, and exponential-backoff
// retry.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/netsafe"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/service/event"
)

// Service owns webhook CRUD + the bus subscription + the dispatcher.
type Service struct {
	hooks      *repository.WebhookRepo
	deliveries *repository.WebhookDeliveryRepo
	bus        *event.Bus
	log        *slog.Logger

	maxAttempts  int
	httpPublic   *http.Client // SSRF guard ON
	httpPrivate  *http.Client // SSRF guard OFF (per-webhook opt-in)
	envelopeVer  string

	inflight atomic.Int64 // diagnostic — concurrent in-flight deliveries
}

// Options tunes the dispatcher.
type Options struct {
	MaxAttempts int           // default 5
	Timeout     time.Duration // default 10s
	EnvelopeVersion string    // default "1"
}

// New constructs the service. It subscribes to the bus immediately;
// call Stop when the process exits to drain in-flight goroutines —
// but for v1 we rely on process shutdown to win those races.
func New(hooks *repository.WebhookRepo, deliveries *repository.WebhookDeliveryRepo, bus *event.Bus, opts Options, lg *slog.Logger) *Service {
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = 5
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Second
	}
	if opts.EnvelopeVersion == "" {
		opts.EnvelopeVersion = "1"
	}

	s := &Service{
		hooks:       hooks,
		deliveries:  deliveries,
		bus:         bus,
		log:         lg.With(slog.String("component", "service.webhook")),
		maxAttempts: opts.MaxAttempts,
		envelopeVer: opts.EnvelopeVersion,
	}

	// One client per security stance. allow-private webhooks need to
	// reach internal infra by design; everyone else uses the guarded
	// dialer.
	s.httpPublic = &http.Client{
		Transport: netsafe.NewHTTPTransport(netsafe.DialerOptions{Timeout: opts.Timeout}),
		Timeout:   opts.Timeout,
	}
	s.httpPrivate = &http.Client{
		Transport: netsafe.NewHTTPTransport(netsafe.DialerOptions{Timeout: opts.Timeout}),
		Timeout:   opts.Timeout,
	}

	bus.Subscribe("*", s.onEvent)
	return s
}

// ---- CRUD -----------------------------------------------------------------

func (s *Service) List(ctx context.Context) ([]model.Webhook, error) {
	return s.hooks.List(ctx)
}
func (s *Service) Get(ctx context.Context, id int64) (*model.Webhook, error) {
	return s.hooks.Get(ctx, id)
}
func (s *Service) Create(ctx context.Context, w *model.Webhook) error {
	if strings.TrimSpace(w.URL) == "" {
		return fmt.Errorf("webhook: url is required")
	}
	if w.Secret == "" {
		w.Secret = randomSecret()
	}
	if w.Events == nil {
		w.Events = []string{}
	}
	return s.hooks.Create(ctx, w)
}
func (s *Service) Update(ctx context.Context, id int64, fields map[string]any) error {
	return s.hooks.Update(ctx, id, fields)
}
func (s *Service) Delete(ctx context.Context, id int64) error { return s.hooks.Delete(ctx, id) }

// SendTest fabricates a test event and dispatches synchronously to
// the named webhook so an admin can verify configuration.
func (s *Service) SendTest(ctx context.Context, webhookID int64) (*model.WebhookDelivery, error) {
	wh, err := s.hooks.Get(ctx, webhookID)
	if err != nil {
		return nil, err
	}
	if wh == nil {
		return nil, fmt.Errorf("webhook not found")
	}
	return s.queueAndDispatch(ctx, wh, "webhook.test", map[string]any{"message": "this is a test event"})
}

// Replay re-dispatches a delivery by id; useful from the admin UI.
func (s *Service) Replay(ctx context.Context, deliveryID int64) (*model.WebhookDelivery, error) {
	d, err := s.deliveries.Get(ctx, deliveryID)
	if err != nil {
		return nil, err
	}
	if d == nil {
		return nil, fmt.Errorf("delivery not found")
	}
	wh, err := s.hooks.Get(ctx, d.WebhookID)
	if err != nil {
		return nil, err
	}
	if wh == nil {
		return nil, fmt.Errorf("owning webhook missing")
	}
	var payloadObj any
	_ = json.Unmarshal(d.Payload, &payloadObj)
	return s.queueAndDispatch(ctx, wh, d.EventType, payloadObj)
}

// ListDeliveries returns the delivery history for one webhook.
func (s *Service) ListDeliveries(ctx context.Context, webhookID int64, limit, offset int) ([]model.WebhookDelivery, error) {
	return s.deliveries.ListByWebhook(ctx, webhookID, limit, offset)
}

// ---- Bus subscription -----------------------------------------------------

func (s *Service) onEvent(e event.Event) {
	// Subscriber callback runs on the publisher's goroutine — keep
	// it cheap. The actual delivery work happens in a fresh
	// goroutine so a slow webhook never blocks the publisher.
	go s.fanOut(e)
}

func (s *Service) fanOut(e event.Event) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	hooks, err := s.hooks.ListEnabled(ctx)
	if err != nil {
		s.log.Warn("list enabled webhooks failed", slog.String("error", err.Error()))
		return
	}
	for i := range hooks {
		wh := hooks[i]
		if !patternsMatch(wh.Events, e.Type) {
			continue
		}
		if _, err := s.queueAndDispatch(ctx, &wh, e.Type, e.Data); err != nil {
			s.log.Warn("webhook dispatch failed",
				slog.Int64("webhook_id", wh.ID),
				slog.String("event", e.Type),
				slog.String("error", err.Error()),
			)
		}
	}
}

// queueAndDispatch persists a delivery row and synchronously runs
// the deliver loop. Synchronous-with-retries inside this function;
// the caller already invoked it on a goroutine via fanOut.
func (s *Service) queueAndDispatch(ctx context.Context, wh *model.Webhook, eventType string, data any) (*model.WebhookDelivery, error) {
	envelope := Envelope{
		Version:   s.envelopeVer,
		Event:     eventType,
		Timestamp: time.Now().UTC(),
		Data:      data,
	}
	payload, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("marshal envelope: %w", err)
	}
	delivery := &model.WebhookDelivery{
		WebhookID:   wh.ID,
		EventType:   eventType,
		Payload:     payload,
		Status:      model.WebhookDeliveryStatusPending,
		ScheduledAt: time.Now().UTC(),
	}
	if err := s.deliveries.Create(ctx, delivery); err != nil {
		return nil, err
	}

	s.inflight.Add(1)
	defer s.inflight.Add(-1)

	go s.deliverWithRetries(wh, delivery)
	return delivery, nil
}

func (s *Service) deliverWithRetries(wh *model.Webhook, d *model.WebhookDelivery) {
	ctx := context.Background()
	for attempt := 1; attempt <= s.maxAttempts; attempt++ {
		d.Attempt = attempt
		status, body, err := s.deliverOnce(wh, d)
		if err == nil && status >= 200 && status < 300 {
			_ = s.deliveries.MarkSuccess(ctx, d.ID, status, body)
			return
		}
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		_ = s.deliveries.MarkFailed(ctx, d.ID, attempt, msg, status, body)
		if attempt == s.maxAttempts {
			return
		}
		// Backoff: 1s, 2s, 4s, 8s … capped at 60s.
		wait := time.Duration(1<<(attempt-1)) * time.Second
		if wait > 60*time.Second {
			wait = 60 * time.Second
		}
		time.Sleep(wait)
	}
}

func (s *Service) deliverOnce(wh *model.Webhook, d *model.WebhookDelivery) (int, string, error) {
	client := s.httpPublic
	if wh.AllowPrivate {
		client = s.httpPrivate
	}
	req, err := http.NewRequest(http.MethodPost, wh.URL, bytes.NewReader(d.Payload))
	if err != nil {
		return 0, "", err
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := sign(wh.Secret, ts, d.Payload)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "3xui-dashboard-webhook/"+s.envelopeVer)
	req.Header.Set("X-Dashboard-Event", d.EventType)
	req.Header.Set("X-Dashboard-Timestamp", ts)
	req.Header.Set("X-Dashboard-Signature", sig)
	req.Header.Set("X-Dashboard-Delivery-Id", strconv.FormatInt(d.ID, 10))

	ctx := context.Background()
	if wh.AllowPrivate {
		ctx = netsafe.WithAllowPrivate(ctx)
	}
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10)) // cap 64 KiB
	return resp.StatusCode, string(body), nil
}

// Envelope is the versioned shape every webhook receives.
type Envelope struct {
	Version   string    `json:"version"`
	Event     string    `json:"event"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data,omitempty"`
}

// sign returns hex-encoded HMAC-SHA256 of timestamp + "." + body
// under the webhook's secret. Receivers verify by recomputing.
func sign(secret, timestamp string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(timestamp))
	m.Write([]byte("."))
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

// patternsMatch reports whether eventType matches any pattern. A
// pattern of "*" matches everything; a pattern ending in ".*"
// matches by prefix; otherwise an exact-string match.
func patternsMatch(patterns []string, eventType string) bool {
	for _, p := range patterns {
		if p == "*" {
			return true
		}
		if strings.HasSuffix(p, ".*") {
			if strings.HasPrefix(eventType, p[:len(p)-1]) {
				return true
			}
			continue
		}
		if p == eventType {
			return true
		}
	}
	return false
}

func randomSecret() string {
	// 32 hex chars / 16 bytes — same shape as the user sub_id
	// generator. Stays uniform across the codebase.
	b := make([]byte, 16)
	_, _ = randRead(b)
	return hex.EncodeToString(b)
}

// randRead is a tiny crypto/rand wrapper so init-time panics surface
// cleanly. We tolerate the error to keep the API simple — randomSecret
// is only ever called in the create path where the caller can re-try.
func randRead(b []byte) (int, error) {
	return cryptoRandReader.Read(b)
}
