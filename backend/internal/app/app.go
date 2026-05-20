// Package app wires the dashboard's service graph: repositories,
// services, handlers, jobs. It is the single source of truth for
// "what gets registered on the gin engine" — both the cmd/dashboard
// binary and the internal/e2e integration tests construct the same
// graph through Build().
package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/cern/3xui-dashboard/internal/config"
	adminhandler "github.com/cern/3xui-dashboard/internal/handler/admin"
	publichandler "github.com/cern/3xui-dashboard/internal/handler/public"
	userhandler "github.com/cern/3xui-dashboard/internal/handler/user"
	"github.com/cern/3xui-dashboard/internal/job"
	"github.com/cern/3xui-dashboard/internal/middleware"
	"github.com/cern/3xui-dashboard/internal/model"
	"github.com/cern/3xui-dashboard/internal/repository"
	"github.com/cern/3xui-dashboard/internal/runtime"
	"github.com/cern/3xui-dashboard/internal/service/auth"
	"github.com/cern/3xui-dashboard/internal/service/billing"
	clientsvc "github.com/cern/3xui-dashboard/internal/service/client"
	"github.com/cern/3xui-dashboard/internal/service/event"
	"github.com/cern/3xui-dashboard/internal/mailer"
	"github.com/cern/3xui-dashboard/internal/service/inbound"
	nodesvc "github.com/cern/3xui-dashboard/internal/service/node"
	"github.com/cern/3xui-dashboard/internal/service/notify"
	"github.com/cern/3xui-dashboard/internal/service/traffic"
	usersvc "github.com/cern/3xui-dashboard/internal/service/user"
	"github.com/cern/3xui-dashboard/internal/service/verification"
	"github.com/cern/3xui-dashboard/internal/service/webhook"
	"github.com/cern/3xui-dashboard/internal/sub"
	"github.com/cern/3xui-dashboard/internal/web"
)

// App bundles everything Build assembles so the binary's main.go and
// the e2e tests can both reach the engine + lifecycle hooks.
type App struct {
	Engine    *gin.Engine
	Scheduler *job.Scheduler

	NodeService    *nodesvc.Service
	UserService    *usersvc.Service
	BillingService *billing.Service
	WebhookService *webhook.Service
	RuntimeManager *runtime.Manager // exposed for tests to swap http client

	cfg *config.Config
	log *slog.Logger
}

// Build assembles the service graph and returns the *App. The caller
// owns lifecycle (Start scheduler, mount on http.Server, shutdown).
func Build(cfg *config.Config, db *gorm.DB, logger *slog.Logger) *App {
	if cfg.Env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()
	engine.Use(gin.Recovery())

	// Probes — fast, dep-free / db-ping.
	engine.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	engine.GET("/readyz", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil {
			c.JSON(503, gin.H{"status": "db_unavailable", "error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := sqlDB.PingContext(ctx); err != nil {
			c.JSON(503, gin.H{"status": "db_unreachable", "error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Auth + middleware.
	authSvc := auth.New(cfg.Auth.JWTSecret, cfg.Auth.AccessTokenTTL, cfg.Admin.Username, cfg.Admin.Password)
	adminAuth := adminhandler.NewAuthHandler(authSvc)
	apiAdmin := engine.Group("/api/admin")
	adminAuth.RegisterRoutes(apiAdmin)
	apiAdminAuthed := engine.Group("/api/admin", middleware.RequireAdmin(authSvc))
	apiUser := engine.Group("/api/user")
	apiUserAuthed := engine.Group("/api/user", middleware.RequireUser(authSvc))

	// Event bus.
	bus := event.New()

	// Runtime + nodes.
	rtManager := runtime.NewManager(&runtime.GormNodeLoader{DB: db}, logger)
	metricsStore := nodesvc.NewMetricsStore(0)
	nodeService := nodesvc.New(db, rtManager, metricsStore, logger)
	adminhandler.NewNodeHandler(nodeService).RegisterRoutes(apiAdminAuthed)

	// Inbound.
	inboundService := inbound.New(rtManager, &nodeListAdapter{svc: nodeService}, logger)
	adminhandler.NewInboundHandler(inboundService).RegisterRoutes(apiAdminAuthed)

	// Repos + client provisioning.
	userRepo := repository.NewUserRepo(db)
	planRepo := repository.NewPlanRepo(db)
	ownershipRepo := repository.NewClientOwnershipRepo(db)
	clientService := clientsvc.New(rtManager, ownershipRepo, &userLookupAdapter{repo: userRepo}, &planLookupAdapter{repo: planRepo}, logger)
	adminhandler.NewClientHandler(clientService).RegisterRoutes(apiAdminAuthed)

	// Traffic.
	trafficRepo := repository.NewTrafficSampleRepo(db)
	trafficService := traffic.New(rtManager, trafficRepo, ownershipRepo, &trafficNodeSource{svc: nodeService}, bus, logger)
	adminhandler.NewTrafficHandler(trafficService, ownershipRepo).RegisterRoutes(apiAdminAuthed)
	userhandler.NewTrafficHandler(trafficService).RegisterRoutes(apiUserAuthed)

	// Settings repo — also used by the subscription handler so admin
	// template overrides take effect without a restart.
	settingRepo := repository.NewSettingRepo(db)

	// Subscription.
	subAsm := sub.New(userRepo, ownershipRepo, &subNodeLookup{svc: nodeService}, rtManager, logger, 0)
	publichandler.NewSubHandler(subAsm, settingRepo, "", logger).RegisterRoutes(engine)

	// User accounts.
	userService := usersvc.New(userRepo, settingRepo, bus, cfg, logger)
	mailerSvc := mailer.New(cfg.SMTP, logger)
	verifyService := verification.New(db, mailerSvc, logger)
	userhandler.NewAuthHandler(userService, authSvc, verifyService, cfg.OIDC, cfg.SMTP.Enabled()).RegisterRoutes(apiUser)
	userhandler.NewAccountHandler(userService, userRepo).RegisterRoutes(apiUserAuthed)
	adminhandler.NewUserHandler(userService, userRepo).RegisterRoutes(apiAdminAuthed)
	adminhandler.NewSettingHandler(settingRepo, cfg).RegisterRoutes(apiAdminAuthed)

	// Billing.
	orderRepo := repository.NewOrderRepo(db)
	billingService := billing.New(planRepo, orderRepo, userRepo, clientService, bus, logger)
	adminhandler.NewPlanHandler(billingService).RegisterRoutes(apiAdminAuthed)
	userhandler.NewBillingHandler(billingService).RegisterRoutes(apiUserAuthed)
	userhandler.NewInboundHandler(inboundService).RegisterRoutes(apiUserAuthed)

	// Webhooks.
	webhookRepo := repository.NewWebhookRepo(db)
	webhookDeliveryRepo := repository.NewWebhookDeliveryRepo(db)
	webhookService := webhook.New(webhookRepo, webhookDeliveryRepo, bus, webhook.Options{}, logger)
	adminhandler.NewWebhookHandler(webhookService).RegisterRoutes(apiAdminAuthed)

	// Scheduler. Build it but don't Start — the caller decides.
	scheduler := job.NewScheduler(logger)
	probeJob := job.NewProbeJob(nodeService, bus, logger, 0, 0)
	_ = scheduler.Add("probe", "@every 30s", probeJob.RunOnce)
	trafficJob := job.NewTrafficJob(trafficService, logger)
	_ = scheduler.Add("traffic", "@every 60s", trafficJob.RunOnce)
	webhookRetryJob := job.NewWebhookRetryJob(webhookService, 0, logger)
	_ = scheduler.Add("webhook-retry", "@every 15s", webhookRetryJob.RunOnce)
	notifyLogRepo := repository.NewNotificationLogRepo(db)
	expiryJob := job.NewExpiryJob(ownershipRepo, settingRepo, userRepo, notifyLogRepo, rtManager, bus, logger)
	_ = scheduler.Add("expiry", "@every 5m", expiryJob.RunOnce)

	// Notify service — subscribes to client lifecycle events and
	// dispatches emails to the owning user via mailer. Wired AFTER
	// the bus + mailer + repos exist; subscriptions register on
	// Start() and live for the process lifetime.
	notify.New(bus, mailerSvc, userRepo, ownershipRepo, notifyLogRepo, logger).Start()

	// SPA.
	web.Register(engine)

	return &App{
		Engine:         engine,
		Scheduler:      scheduler,
		NodeService:    nodeService,
		UserService:    userService,
		BillingService: billingService,
		WebhookService: webhookService,
		RuntimeManager: rtManager,
		cfg:            cfg,
		log:            logger,
	}
}

// Shutdown stops the scheduler and drains in-flight webhook
// deliveries within the supplied context. Callers should pass a
// context with cfg.Server.ShutdownTimeout already applied.
func (a *App) Shutdown(ctx context.Context) {
	select {
	case <-a.Scheduler.Stop().Done():
	case <-ctx.Done():
		a.log.Warn("scheduler stop deadline exceeded")
	}
	a.WebhookService.Drain(ctx)
}

// ---- adapters (kept private — main + tests share them) ---------------------

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

type userLookupAdapter struct{ repo *repository.UserRepo }

func (a *userLookupAdapter) GetUser(ctx context.Context, id int64) (*model.User, error) {
	return a.repo.Get(ctx, id)
}

type planLookupAdapter struct{ repo *repository.PlanRepo }

func (a *planLookupAdapter) GetPlan(ctx context.Context, id int64) (*model.Plan, error) {
	return a.repo.Get(ctx, id)
}

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

type subNodeLookup struct{ svc *nodesvc.Service }

func (a *subNodeLookup) GetNode(ctx context.Context, id int64) (*model.Node, error) {
	n, err := a.svc.Get(ctx, id)
	if err != nil {
		return nil, nil
	}
	return n, nil
}
