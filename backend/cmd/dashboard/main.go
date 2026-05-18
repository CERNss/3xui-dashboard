// Command dashboard is the 3xui-dashboard central control panel
// HTTP server. It loads configuration, sets up structured logging,
// boots a Gin engine with the embedded Vue SPA, and shuts down
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
	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/config"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/web"
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

	if cfg.Env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

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

	engine := gin.New()
	engine.Use(gin.Recovery(), requestLogger(logger))

	// Health probe — fast, dependency-free.
	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Readiness probe — verifies the DB is reachable.
	engine.GET("/readyz", func(c *gin.Context) { readyz(c, db) })

	// Real API routes will be registered here in later groups (admin
	// auth, nodes, inbounds, …). For now only the SPA + probes are
	// mounted so the server is runnable end-to-end after groups 1 + 2.

	web.Register(engine)

	srv := &http.Server{
		Addr:         cfg.Server.ListenAddr,
		Handler:      engine,
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
		return fmt.Errorf("graceful shutdown: %w", err)
	}
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

func readyz(c *gin.Context, db *gorm.DB) {
	sqlDB, err := db.DB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "db_unavailable", "error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "db_unreachable", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// requestLogger is a slim Gin middleware that emits one structured log
// line per request once it has been served.
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
