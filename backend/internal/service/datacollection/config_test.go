package datacollection

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/cern/3xui-dashboard/internal/model"
)

func TestConfigService_Defaults(t *testing.T) {
	svc := NewConfigService(nil, slog.Default())

	health := svc.Health(context.Background())
	if !health.Enabled ||
		health.Interval != DefaultHealthInterval ||
		health.Retention != DefaultHealthRetention ||
		health.Concurrency != DefaultConcurrency ||
		health.Timeout != DefaultHealthTimeout ||
		health.RetryAttempts != DefaultRetryAttempts {
		t.Fatalf("health defaults = %+v", health)
	}

	traffic := svc.Traffic(context.Background())
	if !traffic.Enabled ||
		traffic.Interval != DefaultTrafficInterval ||
		traffic.Retention != DefaultTrafficRetention ||
		traffic.Concurrency != DefaultConcurrency ||
		traffic.Timeout != DefaultTrafficTimeout ||
		traffic.RetryAttempts != DefaultRetryAttempts {
		t.Fatalf("traffic defaults = %+v", traffic)
	}
}

func TestConfigService_ReadsAndNormalizesSettings(t *testing.T) {
	ctx := context.Background()
	repo := fakeSettings{
		bools: map[string]bool{
			model.SettingOpsCollectEnabled: false,
		},
		ints: map[string]int64{
			model.SettingOpsCollectIntervalSeconds:   1,
			model.SettingOpsCollectConcurrency:       999,
			model.SettingOpsCollectTimeoutSeconds:    0,
			model.SettingOpsCollectRetryAttempts:     12,
			model.SettingOpsRetentionSeconds:         0,
			model.SettingTrafficCollectIntervalSecs:  120,
			model.SettingTrafficCollectConcurrency:   4,
			model.SettingTrafficCollectTimeoutSecs:   15,
			model.SettingTrafficCollectRetryAttempts: 2,
			model.SettingTrafficRetentionSeconds:     300,
		},
	}

	svc := NewConfigService(repo, slog.Default())
	health := svc.Health(ctx)
	if health.Enabled {
		t.Fatal("health collection should be disabled")
	}
	if health.Interval != MinInterval {
		t.Fatalf("health interval = %s, want %s", health.Interval, MinInterval)
	}
	if health.Concurrency != MaxConcurrency {
		t.Fatalf("health concurrency = %d, want %d", health.Concurrency, MaxConcurrency)
	}
	if health.Timeout != MinInterval {
		t.Fatalf("health timeout = %s, want %s", health.Timeout, MinInterval)
	}
	if health.RetryAttempts != MaxRetryAttempts {
		t.Fatalf("health retry attempts = %d, want %d", health.RetryAttempts, MaxRetryAttempts)
	}
	if health.Retention != 0 {
		t.Fatalf("health retention = %s, want 0", health.Retention)
	}

	traffic := svc.Traffic(ctx)
	if !traffic.Enabled {
		t.Fatal("traffic collection should default enabled")
	}
	if traffic.Interval != 120*time.Second {
		t.Fatalf("traffic interval = %s", traffic.Interval)
	}
	if traffic.Concurrency != 4 {
		t.Fatalf("traffic concurrency = %d", traffic.Concurrency)
	}
	if traffic.Timeout != 15*time.Second {
		t.Fatalf("traffic timeout = %s", traffic.Timeout)
	}
	if traffic.RetryAttempts != 2 {
		t.Fatalf("traffic retry attempts = %d", traffic.RetryAttempts)
	}
	if traffic.Retention != 300*time.Second {
		t.Fatalf("traffic retention = %s", traffic.Retention)
	}
}

func TestConfigService_TimeoutCannotExceedInterval(t *testing.T) {
	ctx := context.Background()
	repo := fakeSettings{
		ints: map[string]int64{
			model.SettingOpsCollectIntervalSeconds:  10,
			model.SettingOpsCollectTimeoutSeconds:   30,
			model.SettingTrafficCollectIntervalSecs: 20,
			model.SettingTrafficCollectTimeoutSecs:  45,
		},
	}

	svc := NewConfigService(repo, slog.Default())
	health := svc.Health(ctx)
	if health.Timeout != health.Interval {
		t.Fatalf("health timeout = %s, want clamped to interval %s", health.Timeout, health.Interval)
	}

	traffic := svc.Traffic(ctx)
	if traffic.Timeout != traffic.Interval {
		t.Fatalf("traffic timeout = %s, want clamped to interval %s", traffic.Timeout, traffic.Interval)
	}
}

type fakeSettings struct {
	bools map[string]bool
	ints  map[string]int64
}

func (f fakeSettings) GetBool(_ context.Context, key string, fallback bool) (bool, error) {
	if value, ok := f.bools[key]; ok {
		return value, nil
	}
	return fallback, nil
}

func (f fakeSettings) GetInt(_ context.Context, key string, fallback int64) (int64, error) {
	if value, ok := f.ints[key]; ok {
		return value, nil
	}
	return fallback, nil
}
