package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/srjn45/pocket-money/backend/internal/auth"
	"github.com/srjn45/pocket-money/backend/internal/config"
	"github.com/srjn45/pocket-money/backend/internal/db"
	"github.com/srjn45/pocket-money/backend/internal/handlers"
	"github.com/srjn45/pocket-money/backend/internal/middleware"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Run database migrations
	if err := db.RunMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Create database connection pool
	pool, err := db.NewPool(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to create database pool: %v", err)
	}
	defer pool.Close()

	// Create repositories
	userRepo := db.NewUserRepo(pool)
	groupRepo := db.NewGroupRepo(pool)
	choreRepo := db.NewChoreRepo(pool)
	ledgerRepo := db.NewLedgerRepo(pool)
	settlementRepo := db.NewSettlementRepo(pool)
	inviteRepo := db.NewInviteRepo(pool)

	// Create handlers
	authHandler := handlers.NewAuthHandler(userRepo, cfg.JWTSecret)
	groupHandler := handlers.NewGroupHandler(groupRepo, inviteRepo)
	choreHandler := handlers.NewChoreHandler(choreRepo, groupRepo)
	ledgerHandler := handlers.NewLedgerHandler(ledgerRepo, groupRepo, choreRepo)
	settlementHandler := handlers.NewSettlementHandler(settlementRepo, groupRepo)

	// Setup router
	router := gin.Default()

	// Apply CORS middleware
	router.Use(middleware.CORSMiddleware(cfg.CORSOrigins))

	// Health check (no prefix)
	router.GET("/health", handlers.Health)

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Public auth routes
		v1.POST("/auth/register", authHandler.Register)
		v1.POST("/auth/login", authHandler.Login)

		// Protected routes
		protected := v1.Group("")
		protected.Use(auth.AuthMiddleware(cfg.JWTSecret))
		{
			// Auth routes
			protected.GET("/auth/me", authHandler.Me)

			// Group routes
			protected.POST("/groups", groupHandler.CreateGroup)
			protected.GET("/groups", groupHandler.ListGroups)
			protected.GET("/groups/:id", groupHandler.GetGroup)
			protected.GET("/groups/:id/members", groupHandler.ListMembers)
			protected.POST("/groups/:id/invite", groupHandler.CreateInvite)
			protected.POST("/groups/join", groupHandler.JoinGroup)

			// Chore routes
			protected.GET("/groups/:id/chores", choreHandler.ListChores)
			protected.POST("/groups/:id/chores", choreHandler.CreateChore)
			protected.PATCH("/chores/:id", choreHandler.UpdateChore)
			protected.DELETE("/chores/:id", choreHandler.DeleteChore)

			// Ledger routes
			protected.GET("/groups/:id/ledger", ledgerHandler.ListLedger)
			protected.POST("/groups/:id/ledger", ledgerHandler.CreateLedger)
			protected.POST("/ledger/:id/approve", ledgerHandler.ApproveLedger)
			protected.POST("/ledger/:id/reject", ledgerHandler.RejectLedger)
			protected.GET("/groups/:id/pending", ledgerHandler.ListPending)
			protected.GET("/groups/:id/balance", ledgerHandler.GetBalance)

			// Settlement routes
			protected.GET("/groups/:id/settlements", settlementHandler.ListSettlements)
			protected.POST("/groups/:id/settlements", settlementHandler.CreateSettlement)
		}
	}

	// Start server
	addr := ":" + cfg.Port
	log.Printf("Starting server on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
