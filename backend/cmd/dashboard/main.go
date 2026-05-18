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
	adminhandler "github.com/cern/3xui-dashboard/internal/handler/admin"
	"github.com/cern/3xui-dashboard/internal/job"
	"github.com/cern/3xui-dashboard/internal/middleware"
	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/runtime"
	publichandler "github.com/cern/3xui-dashboard/internal/handler/public"
	userhandler "github.com/cern/3xui-dashboard/internal/handler/user"
	"github.com/cern/3xui-dashboard/internal/service/auth"
	"github.com/cern/3xui-dashboard/internal/service/billing"
	clientsvc "github.com/cern/3xui-dashboard/internal/service/client"
	"github.com/cern/3xui-dashboard/internal/service/event"
	"github.com/cern/3xui-dashboard/internal/service/inbound"
	nodesvc "github.com/cern/3xui-dashboard/internal/service/node"
	"github.com/cern/3xui-dashboard/internal/service/traffic"
	usersvc "github.com/cern/3xui-dashboard/internal/service/user"
	"github.com/cern/3xui-dashboard/internal/sub"
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

	// Auth service + handlers + middleware.
	authSvc := auth.New(cfg.Auth.JWTSecret, cfg.Auth.AccessTokenTTL, cfg.Admin.Username, cfg.Admin.Password)
	adminAuth := adminhandler.NewAuthHandler(authSvc)

	// Route groups. /api/admin/auth/login is public (the only admin
	// endpoint that is); every other admin route requires a valid
	// admin token. /api/user/* is the user portal; group 10 fills in
	// the auth + feature endpoints.
	apiAdmin := engine.Group("/api/admin")
	adminAuth.RegisterRoutes(apiAdmin)
	apiAdminAuthed := engine.Group("/api/admin", middleware.RequireAdmin(authSvc))

	apiUser := engine.Group("/api/user")
	_ = apiUser // user-portal login/register handlers land here in group 10
	apiUserAuthed := engine.Group("/api/user", middleware.RequireUser(authSvc))
	_ = apiUserAuthed

	// Event bus shared by node/order/user services; webhooks subscribe in group 12.
	bus := event.New()

	// Node management: runtime manager → metrics store → service → handler.
	rtManager := runtime.NewManager(&runtime.GormNodeLoader{DB: db}, logger)
	metricsStore := nodesvc.NewMetricsStore(0)
	nodeService := nodesvc.New(db, rtManager, metricsStore, logger)
	adminhandler.NewNodeHandler(nodeService).RegisterRoutes(apiAdminAuthed)

	// Inbound service walks across nodes — wire it to the same node
	// enumerator so fleet-wide list stays consistent.
	inboundService := inbound.New(rtManager, &nodeListAdapter{svc: nodeService}, logger)
	adminhandler.NewInboundHandler(inboundService).RegisterRoutes(apiAdminAuthed)

	// User + Plan repositories + Client provisioning service.
	userRepo := repository.NewUserRepo(db)
	planRepo := repository.NewPlanRepo(db)
	ownershipRepo := repository.NewClientOwnershipRepo(db)
	clientService := clientsvc.New(rtManager, ownershipRepo, &userLookupAdapter{repo: userRepo}, &planLookupAdapter{repo: planRepo}, logger)
	adminhandler.NewClientHandler(clientService).RegisterRoutes(apiAdminAuthed)

	// Traffic statistics: repo + service + handlers.
	trafficRepo := repository.NewTrafficSampleRepo(db)
	trafficService := traffic.New(rtManager, trafficRepo, ownershipRepo, &trafficNodeSource{svc: nodeService}, bus, logger)
	adminhandler.NewTrafficHandler(trafficService, ownershipRepo).RegisterRoutes(apiAdminAuthed)
	userhandler.NewTrafficHandler(trafficService).RegisterRoutes(apiUserAuthed)

	// Subscription assembler + public /sub routes (no auth).
	subAsm := sub.New(userRepo, ownershipRepo, &subNodeLookup{svc: nodeService}, rtManager, logger, 0)
	publichandler.NewSubHandler(subAsm, "").RegisterRoutes(engine)

	// User accounts: service + handlers (admin + portal).
	settingRepo := repository.NewSettingRepo(db)
	userService := usersvc.New(userRepo, settingRepo, bus, cfg, logger)
	userhandler.NewAuthHandler(userService, authSvc).RegisterRoutes(apiUser)
	userhandler.NewAccountHandler(userService, userRepo).RegisterRoutes(apiUserAuthed)
	adminhandler.NewUserHandler(userService, userRepo).RegisterRoutes(apiAdminAuthed)

	// Billing: plan admin + Purchase orchestration + order history.
	orderRepo := repository.NewOrderRepo(db)
	billingService := billing.New(planRepo, orderRepo, userRepo, clientService, bus, logger)
	adminhandler.NewPlanHandler(billingService).RegisterRoutes(apiAdminAuthed)
	userhandler.NewBillingHandler(billingService).RegisterRoutes(apiUserAuthed)
	_ = settingRepo

	// Periodic jobs: probe (~30s) + traffic collection (~60s).
	scheduler := job.NewScheduler(logger)
	probeJob := job.NewProbeJob(nodeService, bus, logger, 0, 0)
	if err := scheduler.Add("probe", "@every 30s", probeJob.RunOnce); err != nil {
		return fmt.Errorf("schedule probe job: %w", err)
	}
	trafficJob := job.NewTrafficJob(trafficService, logger)
	if err := scheduler.Add("traffic", "@every 60s", trafficJob.RunOnce); err != nil {
		return fmt.Errorf("schedule traffic job: %w", err)
	}
	scheduler.Start()
	defer func() {
		select {
		case <-scheduler.Stop().Done():
		case <-time.After(cfg.Server.ShutdownTimeout):
			logger.Warn("scheduler stop deadline exceeded")
		}
	}()

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

// nodeListAdapter adapts *node.Service to inbound.NodeListSource.
// Keeps the inbound package free of model.Node imports.
type nodeListAdapter struct{ svc *nodesvc.Service }

func (a *nodeListAdapter) ListEnabledNodes(ctx context.Context) ([]inbound.NodeRef, error) {
	rows, err := a.svc.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]inbound.NodeRef, len(rows))
	for i, n := range rows {
		out[i] = inbound.NodeRef{ID: n.ID, Name: n.Name}
	}
	return out, nil
}

// userLookupAdapter satisfies clientsvc.UserLookup over *UserRepo.
type userLookupAdapter struct{ repo *repository.UserRepo }

func (a *userLookupAdapter) GetUser(ctx context.Context, id int64) (*model.User, error) {
	return a.repo.Get(ctx, id)
}

// planLookupAdapter satisfies clientsvc.PlanLookup over *PlanRepo.
type planLookupAdapter struct{ repo *repository.PlanRepo }

func (a *planLookupAdapter) GetPlan(ctx context.Context, id int64) (*model.Plan, error) {
	return a.repo.Get(ctx, id)
}

// trafficNodeSource satisfies traffic.NodeListSource over *node.Service.
type trafficNodeSource struct{ svc *nodesvc.Service }

func (a *trafficNodeSource) ListEnabledNodes(ctx context.Context) ([]traffic.NodeRef, error) {
	rows, err := a.svc.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]traffic.NodeRef, len(rows))
	for i, n := range rows {
		out[i] = traffic.NodeRef{ID: n.ID, Name: n.Name}
	}
	return out, nil
}

// subNodeLookup satisfies sub.NodeLookup over *node.Service.
type subNodeLookup struct{ svc *nodesvc.Service }

func (a *subNodeLookup) GetNode(ctx context.Context, id int64) (*model.Node, error) {
	n, err := a.svc.Get(ctx, id)
	if err != nil {
		// Don't surface ErrNotFound from the service layer as a hard
		// error here — let sub treat it as "no inbound source".
		return nil, nil
	}
	return n, nil
}
