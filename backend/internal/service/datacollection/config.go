package datacollection

import (
	"context"
	"log/slog"
	"time"

	"github.com/cern/3xui-dashboard/internal/model"
)

const (
	MinInterval             = 5 * time.Second
	MinTimeout              = 1 * time.Second
	MaxTimeout              = 5 * time.Minute
	DefaultConcurrency      = 8
	MaxConcurrency          = 64
	DefaultRetryAttempts    = 0
	MaxRetryAttempts        = 5
	DefaultHealthInterval   = 60 * time.Second
	DefaultHealthTimeout    = 12 * time.Second
	DefaultHealthRetention  = 6 * time.Hour
	DefaultTrafficInterval  = 60 * time.Second
	DefaultTrafficTimeout   = 30 * time.Second
	DefaultTrafficRetention = 30 * 24 * time.Hour
)

// CollectorConfig is the runtime policy used by background node data
// collectors. Retention == 0 means "do not trim".
type CollectorConfig struct {
	Enabled       bool
	Interval      time.Duration
	Retention     time.Duration
	Concurrency   int
	Timeout       time.Duration
	RetryAttempts int
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
		concurrencyKey:   model.SettingOpsCollectConcurrency,
		timeoutKey:       model.SettingOpsCollectTimeoutSeconds,
		retryAttemptsKey: model.SettingOpsCollectRetryAttempts,
		retentionKey:     model.SettingOpsRetentionSeconds,
		defaultInterval:  DefaultHealthInterval,
		defaultTimeout:   DefaultHealthTimeout,
		defaultRetention: DefaultHealthRetention,
		logName:          "health",
	})
}

func (s *ConfigService) Traffic(ctx context.Context) CollectorConfig {
	return s.collectorConfig(ctx, collectorSpec{
		enabledKey:       model.SettingTrafficCollectEnabled,
		intervalKey:      model.SettingTrafficCollectIntervalSecs,
		concurrencyKey:   model.SettingTrafficCollectConcurrency,
		timeoutKey:       model.SettingTrafficCollectTimeoutSecs,
		retryAttemptsKey: model.SettingTrafficCollectRetryAttempts,
		retentionKey:     model.SettingTrafficRetentionSeconds,
		defaultInterval:  DefaultTrafficInterval,
		defaultTimeout:   DefaultTrafficTimeout,
		defaultRetention: DefaultTrafficRetention,
		logName:          "traffic",
	})
}

type collectorSpec struct {
	enabledKey       string
	intervalKey      string
	concurrencyKey   string
	timeoutKey       string
	retryAttemptsKey string
	retentionKey     string
	defaultInterval  time.Duration
	defaultTimeout   time.Duration
	defaultRetention time.Duration
	logName          string
}

func (s *ConfigService) collectorConfig(ctx context.Context, spec collectorSpec) CollectorConfig {
	cfg := CollectorConfig{
		Enabled:       true,
		Interval:      spec.defaultInterval,
		Retention:     spec.defaultRetention,
		Concurrency:   DefaultConcurrency,
		Timeout:       spec.defaultTimeout,
		RetryAttempts: DefaultRetryAttempts,
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

	concurrency, err := s.repo.GetInt(ctx, spec.concurrencyKey, DefaultConcurrency)
	if err != nil {
		s.log.Warn("read data collection concurrency setting",
			slog.String("collector", spec.logName),
			slog.String("error", err.Error()))
	} else {
		cfg.Concurrency = int(concurrency)
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = DefaultConcurrency
	}
	if cfg.Concurrency > MaxConcurrency {
		cfg.Concurrency = MaxConcurrency
	}

	timeoutSeconds, err := s.repo.GetInt(ctx, spec.timeoutKey, int64(spec.defaultTimeout/time.Second))
	if err != nil {
		s.log.Warn("read data collection timeout setting",
			slog.String("collector", spec.logName),
			slog.String("error", err.Error()))
	} else {
		cfg.Timeout = time.Duration(timeoutSeconds) * time.Second
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = spec.defaultTimeout
	}
	if cfg.Timeout < MinTimeout {
		cfg.Timeout = MinTimeout
	}
	if cfg.Timeout > MaxTimeout {
		cfg.Timeout = MaxTimeout
	}
	if cfg.Timeout > cfg.Interval {
		cfg.Timeout = cfg.Interval
	}

	retryAttempts, err := s.repo.GetInt(ctx, spec.retryAttemptsKey, DefaultRetryAttempts)
	if err != nil {
		s.log.Warn("read data collection retry setting",
			slog.String("collector", spec.logName),
			slog.String("error", err.Error()))
	} else {
		cfg.RetryAttempts = int(retryAttempts)
	}
	if cfg.RetryAttempts < 0 {
		cfg.RetryAttempts = DefaultRetryAttempts
	}
	if cfg.RetryAttempts > MaxRetryAttempts {
		cfg.RetryAttempts = MaxRetryAttempts
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
