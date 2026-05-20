// Package app wires the dashboard's service graph: repositories,
// services, handlers, jobs. It is the single source of truth for
// "what gets registered on the gin engine" — both the cmd/dashboard
// binary and the internal/e2e integration tests construct the same
// graph through Build().
package app

import (
	"context"
	"log/slog"
	"strings"
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
	"github.com/cern/3xui-dashboard/internal/metrics"
	"github.com/cern/3xui-dashboard/internal/service/inbound"
	"github.com/cern/3xui-dashboard/internal/service/payment"
	"github.com/cern/3xui-dashboard/internal/service/payment/alipay"
	"github.com/cern/3xui-dashboard/internal/service/payment/stripe"
	nodesvc "github.com/cern/3xui-dashboard/internal/service/node"
	"github.com/cern/3xui-dashboard/internal/service/notify"
	"github.com/cern/3xui-dashboard/internal/service/notify/channels"
	"github.com/cern/3xui-dashboard/internal/service/traffic"
	usersvc "github.com/cern/3xui-dashboard/internal/service/user"
	"github.com/cern/3xui-dashboard/internal/service/verification"
	"github.com/cern/3xui-dashboard/internal/service/webhook"
	"github.com/cern/3xui-dashboard/internal/service/wgcrypto"
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
	PaymentPollJob *job.PaymentPollJob // exposed for tests to trigger RunOnce

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
	// CORS — empty AllowedOrigins means "permissive (echo Origin,
	// no creds)" which is what dev wants. In prod the operator
	// pins this to the panel's public origin.
	engine.Use(middleware.CORS(cfg.Server.AllowedOrigins))
	// Prometheus instrumentation — runs after Recovery so panics
	// still get counted (with their 500 status) but before route
	// handlers so duration includes the handler body.
	engine.Use(metrics.Middleware())

	// /metrics scrape endpoint. Mounted at the root for compatibility
	// with default Prometheus discovery. No auth — the operator
	// should firewall it off if their scraper isn't trusted.
	engine.GET("/metrics", metrics.Handler())

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
	// Login endpoints get a per-IP rate limit so a password-spray
	// attacker can't brute-force from a single source. Defaults:
	// 10 attempts/min/IP. Other routes are unthrottled; auth
	// failures past the limit return 429 with Retry-After.
	loginLimiter := middleware.LoginRateLimiter(10, 10, time.Minute)
	apiAdmin := engine.Group("/api/admin", loginLimiter)
	adminAuth.RegisterRoutes(apiAdmin)
	apiAdminAuthed := engine.Group("/api/admin", middleware.RequireAdmin(authSvc))
	apiUser := engine.Group("/api/user", loginLimiter)
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

	// WireGuard provisioning — only wired when WG_MASTER_KEY is
	// configured. The provisioner stays nil otherwise; downstream
	// handlers / billing branches MUST treat nil as "WG features
	// unavailable on this deployment" rather than panicking.
	wgPeerRepo := repository.NewWGPeerRepo(db)
	var wgProvisioner *clientsvc.WGProvisioner
	if cfg.WireGuard.Enabled() {
		cipher, err := wgcrypto.NewCipherFromHexKey(cfg.WireGuard.MasterKey)
		if err != nil {
			// Misconfigured key is louder than silent disable —
			// the operator clearly intended to enable WG.
			logger.Error("WG_MASTER_KEY rejected; wireguard features disabled", slog.String("err", err.Error()))
		} else if prov, err := clientsvc.NewWGProvisioner(rtManager, ownershipRepo, wgPeerRepo, cipher); err != nil {
			logger.Error("WGProvisioner init failed", slog.String("err", err.Error()))
		} else {
			wgProvisioner = prov
			clientService.SetWGProvisioner(prov)
			logger.Info("wireguard provisioning enabled")
		}
	} else {
		logger.Info("WG_MASTER_KEY not set — wireguard features disabled")
	}

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
	if wgProvisioner != nil {
		subAsm.SetWGPeerSource(&subWGAdapter{prov: wgProvisioner})
	}
	publichandler.NewSubHandler(subAsm, settingRepo, "", logger).RegisterRoutes(engine)

	// User accounts.
	userService := usersvc.New(userRepo, settingRepo, bus, cfg, logger)
	mailerSvc := mailer.New(cfg.SMTP, logger)
	verifyService := verification.New(db, mailerSvc, logger)
	userhandler.NewAuthHandler(userService, authSvc, verifyService, cfg.OIDC, cfg.SMTP.Enabled()).RegisterRoutes(apiUser)
	userhandler.NewAccountHandler(userService, userRepo).RegisterRoutes(apiUserAuthed)
	adminhandler.NewUserHandler(userService, userRepo).RegisterRoutes(apiAdminAuthed)
	adminhandler.NewSettingHandler(settingRepo, cfg).RegisterRoutes(apiAdminAuthed)

	// Billing + payment gateways.
	orderRepo := repository.NewOrderRepo(db)
	paymentRegistry := payment.NewRegistry()
	paymentRegistry.Register(alipay.New(cfg.Alipay))
	paymentRegistry.Register(stripe.New(cfg.Stripe))
	billingService := billing.New(planRepo, orderRepo, userRepo, clientService, bus, paymentRegistry, logger)
	adminhandler.NewPlanHandler(billingService).RegisterRoutes(apiAdminAuthed)
	userhandler.NewBillingHandler(billingService).RegisterRoutes(apiUserAuthed)
	// Public payment notify endpoints — RSA-signed callbacks from
	// the gateway. Mounted on the engine root, not under /api,
	// because alipay requires a plain-text "success" response.
	publichandler.NewPaymentNotifyHandler(billingService, logger).RegisterRoutes(engine)
	userhandler.NewInboundHandler(inboundService).RegisterRoutes(apiUserAuthed)

	// Admin stats overview — server-side aggregates so the page
	// doesn't pull thousands of rows just to render 4 KPI cards.
	statsRepo := repository.NewStatsRepo(db)
	adminhandler.NewStatsHandler(statsRepo).RegisterRoutes(apiAdminAuthed)

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
	if wgProvisioner != nil {
		expiryJob.SetWGRemover(wgProvisioner)
	}
	_ = scheduler.Add("expiry", "@every 5m", expiryJob.RunOnce)
	paymentPollJob := job.NewPaymentPollJob(billingService, paymentRegistry, 15*time.Minute, logger)
	_ = scheduler.Add("payment-poll", "@every 30s", paymentPollJob.RunOnce)

	// Notify service — multi-channel fanout. Channels not configured
	// (empty env vars) report Enabled()=false and the dispatch loop
	// silently skips them. Router parsed from NOTIFY_ROUTES; empty
	// → legacy email-only behavior for client lifecycle events.
	notifyRouter, routerErr := notify.ParseRoutes(cfg.Notify.Routes)
	if routerErr != nil {
		// Misconfigured routes are a hard boot error — operator should
		// see this immediately, not silently.
		logger.Error("invalid NOTIFY_ROUTES",
			"error", routerErr.Error(),
			"value", cfg.Notify.Routes,
		)
		panic("invalid NOTIFY_ROUTES: " + routerErr.Error())
	}
	notifyChannels := []notify.Channel{
		channels.NewEmail(mailerSvc, cfg.Notify.OpsRecipient),
		channels.NewTelegram(cfg.Notify.Telegram.BotToken, cfg.Notify.Telegram.ChatID),
		channels.NewDiscord(cfg.Notify.Discord.WebhookURL),
		channels.NewFeishu(cfg.Notify.Feishu.WebhookURL, cfg.Notify.Feishu.CardTemplate),
	}
	// Warn for channels referenced in routes but unconfigured — helps
	// operators catch missing env vars without crashing the app.
	enabledByName := map[string]bool{}
	for _, c := range notifyChannels {
		enabledByName[c.Name()] = c.Enabled()
	}
	for _, name := range notifyRouter.ConfiguredChannels() {
		if !enabledByName[name] {
			logger.Warn("notify route references unconfigured channel",
				"channel", name,
				"hint", "events routed only to this channel will be dropped")
		}
	}
	// Per-event check: if email is routed for ops events (anything
	// not per-user) but NOTIFY_OPS_RECIPIENT is empty, those events
	// land in /dev/null. Surface at boot so the operator can fix.
	if cfg.Notify.OpsRecipient == "" && enabledByName["email"] {
		for _, eventType := range notify.OpsEventTypes() {
			for _, c := range notifyRouter.Channels(eventType) {
				if c == "email" {
					logger.Warn("notify email routed for ops event but NOTIFY_OPS_RECIPIENT is empty",
						"event", eventType,
						"hint", "set NOTIFY_OPS_RECIPIENT or remove email from this route")
					break
				}
			}
		}
	}
	notify.New(bus, notifyRouter, notifyChannels, userRepo, ownershipRepo, notifyLogRepo, logger).Start()

	// Configured payment-provider currencies — surface at INFO so an
	// operator who forgot STRIPE_CURRENCY=cny sees the active value
	// before a real customer hits a USD checkout for a CNY plan.
	if cfg.Stripe.Enabled() {
		logger.Info("stripe gateway configured",
			"currency", strings.ToLower(cfg.Stripe.Currency),
			"hint", "plan price_cents is interpreted as smallest unit of this currency")
	}
	if cfg.Alipay.Enabled() {
		logger.Info("alipay gateway configured (currency: cny, fixed by gateway)")
	}

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
		PaymentPollJob: paymentPollJob,
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

// subWGAdapter bridges *clientsvc.WGProvisioner to the sub
// package's WGPeerSource interface.
type subWGAdapter struct{ prov *clientsvc.WGProvisioner }

func (a *subWGAdapter) PeerForOwnership(ctx context.Context, ownershipID int64) (*sub.WGPeerView, error) {
	v, err := a.prov.PeerForOwnership(ctx, ownershipID)
	if err != nil || v == nil {
		return nil, err
	}
	return &sub.WGPeerView{
		PrivateKey:      v.PrivateKey,
		PublicKey:       v.PublicKey,
		ServerPublicKey: v.ServerPublicKey,
		AllocatedIP:     v.AllocatedIP,
		MTU:             v.MTU,
	}, nil
}
