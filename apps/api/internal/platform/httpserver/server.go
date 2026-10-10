package httpserver

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"beta-go/api/internal/auth"
	"beta-go/api/internal/commission"
	"beta-go/api/internal/identity"
	"beta-go/api/internal/launch"
	"beta-go/api/internal/ops"
	"beta-go/api/internal/order"
	"beta-go/api/internal/payment"
	"beta-go/api/internal/platform/config"
	"beta-go/api/internal/platform/httpx"
	"beta-go/api/internal/vendor"
)

// New builds the HTTP server. New product areas register their routes here.
func New(cfg config.Config, pool *pgxpool.Pool, authHandler *auth.Handler, vendors *vendor.Handler, orders *order.Handler, payments *payment.Handler, commissions *commission.Handler, operations *ops.Handler, launchHandler *launch.Handler) *gin.Engine {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery(), requestLog(), cors(cfg.WebOrigin))
	_ = router.SetTrustedProxies([]string{"127.0.0.1", "::1"})

	registerRoutes(router, cfg, pool, authHandler, vendors, orders, payments, commissions, operations, launchHandler)
	return router
}

func registerRoutes(router *gin.Engine, cfg config.Config, pool *pgxpool.Pool, authHandler *auth.Handler, vendors *vendor.Handler, orders *order.Handler, payments *payment.Handler, commissions *commission.Handler, operations *ops.Handler, launchHandler *launch.Handler) {
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/readyz", func(c *gin.Context) {
		if err := pool.Ping(c.Request.Context()); err != nil {
			httpx.Error(c, http.StatusServiceUnavailable, "not_ready", "Database is not ready.")
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	api := router.Group("/api/v1")

	signInLimit := auth.NewLimiter(20, time.Minute)
	signIn := api.Group("/auth")
	signIn.POST("/google", signInLimit.Limit(), authHandler.Google)
	signIn.POST("/refresh", authHandler.Refresh)
	signIn.POST("/logout", authHandler.Logout)

	authed := api.Group("")
	authed.Use(auth.Authenticate(cfg, authHandler.Sessions()))
	authed.GET("/me", authHandler.Me)
	authed.PATCH("/me", authHandler.UpdateProfile)

	if vendors != nil {
		api.GET("/vendors", vendors.PublicList)
		api.GET("/vendors/:id", vendors.PublicGet)
	}

	if vendors != nil {
		staff := auth.RequireRole(identity.RoleSupport, identity.RoleSuperAdmin)
		adminVendors := authed.Group("/admin/vendors", staff)
		adminVendors.POST("", vendors.Create)
		adminVendors.GET("", vendors.List)
		adminVendors.GET("/:id", vendors.Get)
		adminVendors.PATCH("/:id", vendors.Update)
		adminVendors.POST("/:id/invitations", vendors.Invite)
		adminVendors.POST("/:id/destination", vendors.SetDestination)
		adminVendors.POST("/:id/destination/verify", vendors.VerifyDestination)
		adminVendors.POST("/:id/approve", vendors.Approve)
		adminVendors.POST("/:id/reject", vendors.Reject)
		adminVendors.POST("/:id/pause", vendors.Pause)
		adminVendors.POST("/:id/suspend", vendors.Suspend)
		adminVendors.POST("/:id/reactivate", vendors.Reactivate)

		authed.POST("/vendor/invitations/accept", vendors.AcceptInvitation)
		shop := authed.Group("/vendor", auth.RequireRole(identity.RoleVendor))
		shop.GET("/shop", vendors.GetShop)
		shop.PATCH("/shop", vendors.UpdateShop)
		shop.PUT("/hours", vendors.ReplaceHours)
		shop.POST("/categories", vendors.AddCategory)
		shop.POST("/items", vendors.AddItem)
		shop.PATCH("/items/:id", vendors.UpdateItem)
	}
	authed.GET("/vendor/home", auth.RequireRole(identity.RoleVendor), workspace("vendor"))

	if orders != nil || payments != nil {
		customer := authed.Group("", auth.RequireRole(identity.RoleCustomer))
		if orders != nil {
			customer.POST("/orders", orders.Create)
			customer.GET("/orders", orders.List)
			customer.GET("/orders/:id", orders.Get)
			if operations != nil {
				customer.POST("/orders/:id/cancel", operations.CustomerCancel)
				customer.POST("/orders/:id/confirm", operations.Confirm)
				customer.POST("/orders/:id/disputes", operations.OpenDispute)
			} else {
				customer.POST("/orders/:id/cancel", orders.Cancel)
			}
		}
		if payments != nil {
			customer.POST("/orders/:id/payment", payments.Initiate)
			customer.GET("/orders/:id/payment", payments.Get)
		}
	}
	authed.GET("/customer/home", auth.RequireRole(identity.RoleCustomer), workspace("customer"))

	if payments != nil {
		api.POST("/payments/webhook", payments.Webhook)
		authed.POST("/admin/payments/reconcile", auth.RequireRole(identity.RoleFinance, identity.RoleSuperAdmin), payments.Reconcile)
	}

	if commissions != nil {
		poolGroup := authed.Group("/vendor", auth.RequireRole(identity.RoleVendor))
		poolGroup.GET("/commission", commissions.GetMine)
		poolGroup.GET("/commission/ledger", commissions.LedgerMine)
		poolGroup.POST("/commission/top-ups", commissions.StartTopUp)
		poolGroup.GET("/commission/top-ups/:id", commissions.GetTopUp)
		api.POST("/commission/top-ups/webhook", commissions.TopUpWebhook)
		money := auth.RequireRole(identity.RoleFinance, identity.RoleSuperAdmin)
		authed.GET("/admin/vendors/:id/commission", money, commissions.GetAdmin)
		authed.POST("/admin/vendors/:id/commission/adjustments", money, commissions.RequestAdjustment)
		authed.POST("/admin/commission/adjustments/:id/approve", money, commissions.ApproveAdjustment)
	}

	if operations != nil {
		kitchen := authed.Group("/vendor", auth.RequireRole(identity.RoleVendor))
		kitchen.GET("/orders", operations.ListVendor)
		kitchen.GET("/orders/:id", operations.GetVendor)
		kitchen.POST("/orders/:id/accept", operations.Accept)
		kitchen.POST("/orders/:id/reject", operations.Reject)
		kitchen.POST("/orders/:id/preparing", operations.Preparing)
		kitchen.POST("/orders/:id/ready", operations.Ready)
		kitchen.POST("/orders/:id/fulfilled", operations.Fulfilled)
		kitchen.POST("/orders/:id/cancel", operations.VendorCancel)
		kitchen.POST("/orders/:id/cancel-approve", operations.ApproveCancel)
		staff := auth.RequireRole(identity.RoleSupport, identity.RoleFinance, identity.RoleSuperAdmin)
		authed.POST("/admin/orders/:id/disputes/resolve", staff, operations.ResolveDispute)
		authed.POST("/admin/orders/reconcile", staff, operations.Reconcile)
	}

	authed.GET("/admin/home", auth.RequireRole(identity.RoleSupport, identity.RoleFinance, identity.RoleSuperAdmin), workspace("admin"))

	if launchHandler != nil {
		prep := authed.Group("/admin/launch", auth.RequireRole(identity.RoleSuperAdmin))
		prep.GET("/security", launchHandler.Security)
		prep.GET("/audit", launchHandler.ListAudit)
		prep.GET("/backups", launchHandler.ListBackups)
		prep.POST("/backups", launchHandler.RecordBackup)
		prep.POST("/staff", launchHandler.GrantStaff)
	}
}

func workspace(name string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"workspace": name})
	}
}

func requestLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		if c.Request.URL.Path == "/healthz" {
			return
		}
		slog.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"ms", time.Since(start).Milliseconds(),
		)
	}
}

func cors(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("Origin") == origin {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
