// Package repository owns the database connection and the persistence
// implementations for every domain model. The connection is opened by
// Open() at startup; the *gorm.DB returned should be threaded into
// every constructor that needs DB access.
package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/cern/3xui-dashboard/internal/config"
)

// Open establishes the Postgres connection, configures the pool from
// cfg, and verifies reachability with a Ping. A brief retry loop
// absorbs the common "db starting alongside the app" race seen in
// docker-compose: total wait is capped near 30 s.
func Open(ctx context.Context, cfg *config.Config, lg *slog.Logger) (*gorm.DB, error) {
	if cfg.DB.URL == "" {
		return nil, errors.New("repository.Open: DATABASE_URL is empty")
	}

	gormCfg := &gorm.Config{
		Logger:                                   buildGormLogger(lg, cfg.Server.LogLevel),
		PrepareStmt:                              true,
		DisableForeignKeyConstraintWhenMigrating: true,
		NowFunc:                                  func() time.Time { return time.Now().UTC() },
	}

	const maxAttempts = 10
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("repository.Open: %w", err)
		}
		db, err := gorm.Open(postgres.Open(cfg.DB.URL), gormCfg)
		if err == nil {
			sqlDB, sErr := db.DB()
			if sErr == nil {
				if pErr := sqlDB.PingContext(ctx); pErr == nil {
					sqlDB.SetMaxOpenConns(cfg.DB.MaxOpenConns)
					sqlDB.SetMaxIdleConns(cfg.DB.MaxIdleConns)
					sqlDB.SetConnMaxLifetime(30 * time.Minute)
					lg.Info("database connected",
						slog.Int("max_open", cfg.DB.MaxOpenConns),
						slog.Int("max_idle", cfg.DB.MaxIdleConns),
						slog.Int("attempt", attempt),
					)
					return db, nil
				} else {
					err = pErr
				}
			} else {
				err = sErr
			}
		}
		lastErr = err
		lg.Warn("database connection attempt failed",
			slog.Int("attempt", attempt),
			slog.Int("max_attempts", maxAttempts),
			slog.String("error", err.Error()),
		)
		// Backoff: 250ms, 500ms, 1s, 1.5s, 2s, 2s, 2s, …
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff(attempt)):
		}
	}
	return nil, fmt.Errorf("repository.Open: %d attempts exhausted: %w", maxAttempts, lastErr)
}

// Close gracefully closes the underlying *sql.DB.
func Close(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func backoff(attempt int) time.Duration {
	switch attempt {
	case 1:
		return 250 * time.Millisecond
	case 2:
		return 500 * time.Millisecond
	case 3:
		return time.Second
	case 4:
		return 1500 * time.Millisecond
	default:
		return 2 * time.Second
	}
}

func buildGormLogger(lg *slog.Logger, level string) logger.Interface {
	var l logger.LogLevel
	switch level {
	case "debug":
		l = logger.Info
	case "warn":
		l = logger.Warn
	case "error":
		l = logger.Error
	default:
		l = logger.Warn
	}
	return logger.New(
		slogWriter{lg: lg.With(slog.String("component", "gorm"))},
		logger.Config{
			SlowThreshold:             500 * time.Millisecond,
			LogLevel:                  l,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			Colorful:                  false,
		},
	)
}

// slogWriter adapts *slog.Logger to GORM's logger.Writer interface.
type slogWriter struct{ lg *slog.Logger }

func (w slogWriter) Printf(format string, args ...any) {
	w.lg.Info(fmt.Sprintf(format, args...))
}
