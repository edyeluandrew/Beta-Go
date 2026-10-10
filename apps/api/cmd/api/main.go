package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"beta-go/api/internal/auth"
	"beta-go/api/internal/commission"
	"beta-go/api/internal/identity"
	"beta-go/api/internal/launch"
	"beta-go/api/internal/ops"
	"beta-go/api/internal/order"
	"beta-go/api/internal/payment"
	"beta-go/api/internal/platform/config"
	"beta-go/api/internal/platform/database"
	"beta-go/api/internal/platform/httpserver"
	"beta-go/api/internal/vendor"
	"beta-go/api/migrations"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration", "err", err.Error())
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database", "err", err.Error())
		os.Exit(1)
	}
	defer pool.Close()

	migrateCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := database.Migrate(migrateCtx, pool, migrations.FS); err != nil {
		slog.Error("migrations", "err", err.Error())
		os.Exit(1)
	}

	users := identity.NewStore(pool)
	sessions := auth.NewSessionStore(pool)
	service := auth.NewService(cfg, users, sessions, auth.NewGoogleVerifier(cfg.GoogleClientID))
	handler := auth.NewHandler(service, cfg)
	vendorDir := vendor.NewStore(pool)
	vendorSvc := vendor.NewService(vendorDir, users)
	vendorHandler := vendor.NewHandler(vendorSvc, service, cfg)
	orderDir := order.NewStore(pool)
	orderHandler := order.NewHandler(order.NewService(vendorSvc, vendorDir, orderDir, orderDir))
	var gateway payment.Provider
	if cfg.Env != "production" && cfg.PaymentProvider == payment.ProviderSandbox && cfg.PaymentWebhookSecret != "" {
		gateway = payment.NewSandbox()
	}
	paySvc := payment.NewService(payment.NewStore(pool), orderDir, orderDir, gateway, cfg.PaymentWebhookSecret)
	paymentHandler := payment.NewHandler(paySvc)
	commissionSvc := commission.NewService(vendorDir, commission.NewStore(pool), orderDir, orderDir, gateway, cfg.PaymentWebhookSecret)
	commissionHandler := commission.NewHandler(commissionSvc)
	opsHandler := ops.NewHandler(ops.NewService(orderDir, vendorDir, commissionSvc, paySvc, ops.NewStore(pool), orderDir))
	launchHandler := launch.NewHandler(launch.NewService(launch.NewStore(pool), users, cfg))
	engine := httpserver.New(cfg, pool, handler, vendorHandler, orderHandler, paymentHandler, commissionHandler, opsHandler, launchHandler)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info("api listening", "addr", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server", "err", err.Error())
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown", "err", err.Error())
	}
}
