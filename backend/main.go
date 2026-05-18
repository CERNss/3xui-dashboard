package main

import (
	"fmt"
	"log"
	"os"

	"github.com/cern/3xui-dashboard/config"
	"github.com/cern/3xui-dashboard/handler"
	adminhandler "github.com/cern/3xui-dashboard/handler/admin"
	userhandler "github.com/cern/3xui-dashboard/handler/user"
	"github.com/cern/3xui-dashboard/internal/web"
	"github.com/cern/3xui-dashboard/middleware"
	"github.com/cern/3xui-dashboard/model"
	"github.com/cern/3xui-dashboard/service"
	"github.com/cern/3xui-dashboard/service/xui"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func main() {
	// Load configuration
	cfgFile := ""
	if len(os.Args) > 1 {
		cfgFile = os.Args[1]
	}
	config.Load(cfgFile)

	// Ensure data directory exists
	if err := os.MkdirAll("./data", 0750); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Connect to database
	db, err := gorm.Open(sqlite.Open(config.C.Database.Path), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Auto-migrate schema
	if err := db.AutoMigrate(&model.User{}); err != nil {
		log.Fatalf("Database migration failed: %v", err)
	}

	// Build services
	userSvc := service.NewUserService(db)
	xuiClient := xui.NewClient()

	// Build handlers
	authH := handler.NewAuthHandler(userSvc)
	adminInboundH := adminhandler.NewInboundHandler(xuiClient)
	adminClientH := adminhandler.NewClientHandler(xuiClient)
	adminNodeH := adminhandler.NewNodeHandler(xuiClient)
	adminStatsH := adminhandler.NewStatsHandler(xuiClient)
	adminUsersH := adminhandler.NewUsersHandler(userSvc)
	userProfileH := userhandler.NewProfileHandler(userSvc)
	userSubH := userhandler.NewSubscriptionHandler(userSvc)
	userTrafficH := userhandler.NewTrafficHandler(userSvc, xuiClient)

	// Set up router
	r := gin.Default()
	r.Use(middleware.CORS())

	// Auth routes
	authGroup := r.Group("/api/auth")
	{
		authGroup.POST("/login", authH.Login)
		authGroup.POST("/register", authH.Register)
	}

	// Admin routes
	adminGroup := r.Group("/api/admin", middleware.Auth(), middleware.RequireAdmin())
	{
		adminGroup.GET("/stats", adminStatsH.Get)

		adminGroup.GET("/inbounds", adminInboundH.List)
		adminGroup.POST("/inbounds", adminInboundH.Create)
		adminGroup.PUT("/inbounds/:id", adminInboundH.Update)
		adminGroup.DELETE("/inbounds/:id", adminInboundH.Delete)

		adminGroup.GET("/clients", adminClientH.List)
		adminGroup.POST("/clients", adminClientH.Create)
		adminGroup.PUT("/clients/:uuid", adminClientH.Update)
		adminGroup.DELETE("/clients/:uuid", adminClientH.Delete)

		adminGroup.GET("/nodes", adminNodeH.List)

		adminGroup.GET("/users", adminUsersH.List)
		adminGroup.PUT("/users/:id", adminUsersH.Update)
		adminGroup.DELETE("/users/:id", adminUsersH.Delete)
	}

	// User routes
	userGroup := r.Group("/api/user", middleware.Auth())
	{
		userGroup.GET("/profile", userProfileH.Get)
		userGroup.PUT("/profile", userProfileH.Update)
		userGroup.POST("/change-password", userProfileH.ChangePassword)
		userGroup.GET("/subscription", userSubH.Get)
		userGroup.GET("/traffic", userTrafficH.Get)
	}

	// Serve frontend static files (SPA)
	web.RegisterStaticFiles(r)

	addr := fmt.Sprintf(":%d", config.C.Server.Port)
	log.Printf("3xui-dashboard starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
