package datacollection

import (
	"context"
	"log/slog"
	"time"

	"github.com/cern/3xui-dashboard/internal/model"
)

const (
	MinInterval             = 5 * time.Second
	DefaultHealthInterval   = 60 * time.Second
	DefaultHealthRetention  = 6 * time.Hour
	DefaultTrafficInterval  = 60 * time.Second
	DefaultTrafficRetention = 30 * 24 * time.Hour
)

// CollectorConfig is the runtime policy used by background node data
// collectors. Retention == 0 means "do not trim".
type CollectorConfig struct {
	Enabled   bool
	Interval  time.Duration
	Retention time.Duration
}

type SettingsReader interface {
	GetBool(ctx context.Context, key string, fallback bool) (bool, error)
	GetInt(ctx context.Context, key string, fallback int64) (int64, error)
}

// ConfigService reads node data-collection runtime settings from the settings
// repo and normalizes them into concrete job policies.
type ConfigService struct {
	repo SettingsReader
	log  *slog.Logger
}

func NewConfigService(repo SettingsReader, lg *slog.Logger) *ConfigService {
	return &ConfigService{
		repo: repo,
		log:  lg.With(slog.String("component", "service.datacollection")),
	}
}

func (s *ConfigService) Health(ctx context.Context) CollectorConfig {
	return s.collectorConfig(ctx, collectorSpec{
		enabledKey:       model.SettingOpsCollectEnabled,
		intervalKey:      model.SettingOpsCollectIntervalSeconds,
		retentionKey:     model.SettingOpsRetentionSeconds,
		defaultInterval:  DefaultHealthInterval,
		defaultRetention: DefaultHealthRetention,
		logName:          "health",
	})
}

func (s *ConfigService) Traffic(ctx context.Context) CollectorConfig {
	return s.collectorConfig(ctx, collectorSpec{
		enabledKey:       model.SettingTrafficCollectEnabled,
		intervalKey:      model.SettingTrafficCollectIntervalSecs,
		retentionKey:     model.SettingTrafficRetentionSeconds,
		defaultInterval:  DefaultTrafficInterval,
		defaultRetention: DefaultTrafficRetention,
		logName:          "traffic",
	})
}

type collectorSpec struct {
	enabledKey       string
	intervalKey      string
	retentionKey     string
	defaultInterval  time.Duration
	defaultRetention time.Duration
	logName          string
}

func (s *ConfigService) collectorConfig(ctx context.Context, spec collectorSpec) CollectorConfig {
	cfg := CollectorConfig{
		Enabled:   true,
		Interval:  spec.defaultInterval,
		Retention: spec.defaultRetention,
	}
	if s == nil || s.repo == nil {
		return cfg
	}

	enabled, err := s.repo.GetBool(ctx, spec.enabledKey, true)
	if err != nil {
		s.log.Warn("read data collection enabled setting",
			slog.String("collector", spec.logName),
			slog.String("error", err.Error()))
	} else {
		cfg.Enabled = enabled
	}

	intervalSeconds, err := s.repo.GetInt(ctx, spec.intervalKey, int64(spec.defaultInterval/time.Second))
	if err != nil {
		s.log.Warn("read data collection interval setting",
			slog.String("collector", spec.logName),
			slog.String("error", err.Error()))
	} else {
		cfg.Interval = time.Duration(intervalSeconds) * time.Second
	}
	if cfg.Interval < MinInterval {
		cfg.Interval = MinInterval
	}

	retentionSeconds, err := s.repo.GetInt(ctx, spec.retentionKey, int64(spec.defaultRetention/time.Second))
	if err != nil {
		s.log.Warn("read data collection retention setting",
			slog.String("collector", spec.logName),
			slog.String("error", err.Error()))
	} else {
		cfg.Retention = time.Duration(retentionSeconds) * time.Second
	}
	if cfg.Retention < 0 {
		cfg.Retention = spec.defaultRetention
	}
	return cfg
}
