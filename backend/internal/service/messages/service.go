// Package messages delivers user-facing transactional emails over
// SMTP. It is the "messages" surface in the messages/notifications
// split: single channel (SMTP), single recipient (the user), no
// multi-channel routing, no admin webhooks. Ops-facing alerts go
// through service/notifications instead.
//
// Dedup: per (model.SurfaceMessage, kind, ownership_id) in
// notification_log when the caller provides both an ownership ID
// and a kind. Transactional one-shots (verification codes,
// password reset) skip the dedup log entirely — rate-limiting
// lives upstream where the code/token is generated.
package messages

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/cern/3xui-dashboard/internal/mailer"
	"github.com/cern/3xui-dashboard/internal/model"
)

// NotificationLogStore is the subset of repository.NotificationLogRepo
// this service needs. Defined locally so tests can stub it.
type NotificationLogStore interface {
	AlreadySent(ctx context.Context, surface, kind string, ownershipID int64) (bool, error)
	MarkSent(ctx context.Context, surface, kind string, ownershipID int64, userEmail string) error
}

// Service wraps the mailer with messages-surface semantics. The
// mailer may be nil or unconfigured — Send becomes a no-op in
// that case (consistent with the rest of the codebase's "SMTP
// optional" stance).
type Service struct {
	mailer *mailer.Mailer
	logs   NotificationLogStore
	log    *slog.Logger
}

// New wires the service. lg must not be nil.
func New(m *mailer.Mailer, logs NotificationLogStore, lg *slog.Logger) *Service {
	return &Service{
		mailer: m,
		logs:   logs,
		log:    lg.With(slog.String("component", "service.messages")),
	}
}

// Enabled reports whether the underlying mailer is configured.
// Callers can branch on this for dev-mode behavior (e.g.
// verification logs the code to stderr when SMTP is off).
func (s *Service) Enabled() bool {
	return s.mailer != nil && s.mailer.Enabled()
}

// Send delivers one transactional email. Returns nil + logs at
// debug when the mailer is disabled — callers treat that as a
// soft success.
//
// Dedup: when dedupOwnershipID > 0 AND dedupKind != "", the
// service checks notification_log for an existing
// (SurfaceMessage, dedupKind, dedupOwnershipID) row and skips if
// found. After a successful send, a row is recorded. Pass zero
// values for dedupOwnershipID / empty dedupKind to disable dedup
// (rate-limited one-shots like verification codes do this).
//
// Errors from mailer.Send are returned wrapped; dedup-log errors
// after a successful send are logged but not returned (delivery
// already happened — the caller shouldn't retry).
func (s *Service) Send(
	ctx context.Context,
	to, subject, body string,
	dedupKind string,
	dedupOwnershipID int64,
) error {
	if !s.Enabled() {
		s.log.Debug("messages.Send: mailer disabled, dropping",
			slog.String("to", to), slog.String("subject", subject))
		return nil
	}
	if to == "" {
		return fmt.Errorf("messages.Send: empty recipient")
	}

	if dedupOwnershipID > 0 && dedupKind != "" {
		already, err := s.logs.AlreadySent(ctx, model.SurfaceMessage, dedupKind, dedupOwnershipID)
		if err != nil {
			// Fall through — better to risk a dup than miss the message.
			s.log.Warn("messages: dedup check failed (proceeding)",
				slog.String("kind", dedupKind), slog.Int64("ownership_id", dedupOwnershipID),
				slog.String("err", err.Error()))
		}
		if already {
			s.log.Debug("messages: dedup hit, skipping",
				slog.String("kind", dedupKind), slog.Int64("ownership_id", dedupOwnershipID))
			return nil
		}
	}

	if err := s.mailer.Send(to, subject, body); err != nil {
		return fmt.Errorf("messages.Send mailer: %w", err)
	}

	if dedupOwnershipID > 0 && dedupKind != "" {
		if err := s.logs.MarkSent(ctx, model.SurfaceMessage, dedupKind, dedupOwnershipID, to); err != nil {
			s.log.Warn("messages: MarkSent failed (delivery succeeded)",
				slog.String("kind", dedupKind), slog.Int64("ownership_id", dedupOwnershipID),
				slog.String("err", err.Error()))
		}
	}

	s.log.Info("messages delivered",
		slog.String("to", to), slog.String("subject", subject),
		slog.String("kind", dedupKind))
	return nil
}
