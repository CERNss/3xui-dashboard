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
	if !health.Enabled || health.Interval != DefaultHealthInterval || health.Retention != DefaultHealthRetention {
		t.Fatalf("health defaults = %+v", health)
	}

	traffic := svc.Traffic(context.Background())
	if !traffic.Enabled || traffic.Interval != DefaultTrafficInterval || traffic.Retention != DefaultTrafficRetention {
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
			model.SettingOpsCollectIntervalSeconds:  1,
			model.SettingOpsRetentionSeconds:        0,
			model.SettingTrafficCollectIntervalSecs: 120,
			model.SettingTrafficRetentionSeconds:    300,
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
	if traffic.Retention != 300*time.Second {
		t.Fatalf("traffic retention = %s", traffic.Retention)
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
