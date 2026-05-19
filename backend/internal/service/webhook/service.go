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
	"sync"
	"time"

	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/netsafe"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/service/event"
)

// Service owns webhook CRUD + the bus subscription + the dispatcher.
// Retries are persistent: a transient failure pushes next_attempt_at
// out and leaves the row pending, the retry-cron picks it up. A
// process crash never strands an in-memory retry timer.
type Service struct {
	hooks      *repository.WebhookRepo
	deliveries *repository.WebhookDeliveryRepo
	bus        *event.Bus
	log        *slog.Logger

	maxAttempts int
	httpPublic  *http.Client // SSRF guard ON
	httpPrivate *http.Client // SSRF guard OFF (per-webhook opt-in)
	envelopeVer string

	// inflight tracks dispatched goroutines so Drain() can wait for
	// them at shutdown.
	inflight sync.WaitGroup
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

// Replay queues a fresh delivery using the original event_type and
// payload — the previous delivery row is left as historical record.
// Returns the new delivery row.
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

// queueAndDispatch persists a delivery row in the pending queue and
// fires one immediate-best-effort attempt in a goroutine. If that
// attempt fails non-terminally the row stays pending with
// next_attempt_at advanced — RetryDue will pick it up later.
//
// Crash safety: between Create() and the goroutine actually firing,
// the row is already in the queue and the retry-cron will eventually
// dispatch it. Worst case a delivery is delayed by one retry interval.
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
	now := time.Now().UTC()
	delivery := &model.WebhookDelivery{
		WebhookID:     wh.ID,
		EventType:     eventType,
		Payload:       payload,
		Status:        model.WebhookDeliveryStatusPending,
		ScheduledAt:   now,
		NextAttemptAt: now,
	}
	if err := s.deliveries.Create(ctx, delivery); err != nil {
		return nil, err
	}
	s.spawnAttempt(*wh, *delivery)
	return delivery, nil
}

// spawnAttempt fires one delivery attempt on a tracked goroutine.
// The WaitGroup ensures Drain() can wait for in-flight work at
// shutdown.
func (s *Service) spawnAttempt(wh model.Webhook, d model.WebhookDelivery) {
	s.inflight.Add(1)
	go func() {
		defer s.inflight.Done()
		s.deliverAndRecord(&wh, &d)
	}()
}

// deliverAndRecord runs a single attempt and updates the DB row
// based on the outcome — either MarkSuccess (terminal), ScheduleRetry
// (still pending, next_attempt_at advanced), or MarkTerminallyFailed
// (attempt count exhausted).
func (s *Service) deliverAndRecord(wh *model.Webhook, d *model.WebhookDelivery) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	attempt := d.Attempt + 1
	status, body, err := s.deliverOnce(wh, d)
	if err == nil && status >= 200 && status < 300 {
		_ = s.deliveries.MarkSuccess(ctx, d.ID, attempt, status, body)
		return
	}
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	if attempt >= s.maxAttempts {
		_ = s.deliveries.MarkTerminallyFailed(ctx, d.ID, attempt, status, msg, body)
		return
	}
	next := time.Now().UTC().Add(retryBackoff(attempt))
	_ = s.deliveries.ScheduleRetry(ctx, d.ID, attempt, status, msg, body, next)
}

// retryBackoff is the exponential schedule between attempt N and
// attempt N+1: 1s, 2s, 4s, 8s, capped at 60s.
func retryBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}
	wait := time.Duration(1<<(attempt-1)) * time.Second
	if wait > 60*time.Second {
		wait = 60 * time.Second
	}
	return wait
}

// RetryDue is invoked by the cron job — claims a batch of pending+
// due rows under SELECT FOR UPDATE SKIP LOCKED and re-dispatches.
// Safe to run concurrently across multiple instances; SKIP LOCKED
// means each row goes to exactly one worker per pass.
func (s *Service) RetryDue(ctx context.Context, batch int) {
	rows, err := s.deliveries.ClaimDue(ctx, batch)
	if err != nil {
		s.log.Warn("RetryDue claim failed", slog.String("error", err.Error()))
		return
	}
	if len(rows) == 0 {
		return
	}
	for i := range rows {
		d := rows[i]
		wh, err := s.hooks.Get(ctx, d.WebhookID)
		if err != nil || wh == nil {
			s.log.Warn("retry: webhook missing", slog.Int64("delivery_id", d.ID))
			continue
		}
		s.spawnAttempt(*wh, d)
	}
}

// Drain waits for every in-flight delivery goroutine to finish or
// for ctx to expire. Call from main shutdown so SIGTERM doesn't kill
// the goroutines mid-attempt (which would strand the row as pending
// — fine for safety, but produces an extra retry).
func (s *Service) Drain(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		s.inflight.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		s.log.Warn("webhook drain deadline exceeded; some deliveries may retry on next boot")
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
