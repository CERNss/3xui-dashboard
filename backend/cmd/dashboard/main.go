// Command dashboard is the 3xui-dashboard central control panel
// HTTP server. It loads configuration, sets up structured logging,
// boots the gin engine assembled by internal/app, and shuts down
// gracefully on SIGINT / SIGTERM.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cern/3xui-dashboard/internal/app"
	"github.com/cern/3xui-dashboard/internal/config"
	"github.com/cern/3xui-dashboard/internal/repository"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	envFile := flag.String("env", ".env", "path to .env file (optional — real environment variables always win)")
	flag.Parse()

	cfg, err := config.Load(*envFile)
	if err != nil {
		return err
	}

	logger := buildLogger(cfg)
	slog.SetDefault(logger)

	bootCtx, bootCancel := context.WithTimeout(context.Background(), 45*time.Second)
	db, err := repository.Open(bootCtx, cfg, logger)
	bootCancel()
	if err != nil {
		return err
	}
	defer func() {
		if err := repository.Close(db); err != nil {
			logger.Warn("database close failed", slog.String("error", err.Error()))
		}
	}()

	if cfg.DB.MigrateOnBoot {
		if err := repository.MigrateUp(db, logger); err != nil {
			return err
		}
	} else {
		logger.Info("DB_MIGRATE_ON_BOOT=false; skipping schema migration")
	}

	a := app.Build(cfg, db, logger)
	a.Engine.Use(requestLogger(logger))
	a.Scheduler.Start()

	srv := &http.Server{
		Addr:         cfg.Server.ListenAddr,
		Handler:      a.Engine,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("server listening",
			slog.String("addr", cfg.Server.ListenAddr),
			slog.String("env", cfg.Env),
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-serveErr:
		if err != nil {
			return fmt.Errorf("server: %w", err)
		}
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Warn("http shutdown returned error", slog.String("error", err.Error()))
	}
	a.Shutdown(shutdownCtx)
	logger.Info("server stopped")
	return nil
}

func buildLogger(cfg *config.Config) *slog.Logger {
	var level slog.Level
	switch cfg.Server.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if cfg.Server.LogFormat == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}

// requestLogger emits one structured log line per request.
func requestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)

		attrs := []any{
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", c.Writer.Status()),
			slog.Duration("latency", latency),
			slog.String("client_ip", c.ClientIP()),
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("errors", c.Errors.String()))
		}

		switch {
		case c.Writer.Status() >= 500:
			logger.Error("http request", attrs...)
		case c.Writer.Status() >= 400:
			logger.Warn("http request", attrs...)
		default:
			logger.Info("http request", attrs...)
		}
	}
}
