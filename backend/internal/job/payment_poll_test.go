package job

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/cern/3xui-dashboard/internal/service/payment"
)

func TestPaymentPollJob_NilRegistryNoop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	j := NewPaymentPollJob(nil, nil, time.Minute, logger)
	// Should not panic — billing being nil is irrelevant when gateways is nil
	j.RunOnce(context.Background())
}

func TestPaymentPollJob_DefaultExpiryWindow(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	j := NewPaymentPollJob(nil, payment.NewRegistry(), 0, logger)
	if j.expiryWindow != 15*time.Minute {
		t.Errorf("expiryWindow = %v, want 15m default", j.expiryWindow)
	}
}

func TestPaymentPollJob_HonorsCustomWindow(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	j := NewPaymentPollJob(nil, payment.NewRegistry(), 5*time.Minute, logger)
	if j.expiryWindow != 5*time.Minute {
		t.Errorf("expiryWindow = %v, want 5m", j.expiryWindow)
	}
}
