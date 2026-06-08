package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"openforge/internal/auth/service"
	"openforge/internal/observability/adapter"
	"openforge/internal/server"
	"openforge/internal/shared/profile"
)

func main() {
	// Initialize structured logging
	logHandler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(logHandler))

	// Initialize OpenTelemetry tracing.  OTLP endpoint is read from
	// OTLP_ENDPOINT (default localhost:4317, plaintext gRPC).  Initialisation
	// failure is non-fatal: the server still starts, but spans will be dropped
	// silently.
	otelEndpoint := os.Getenv("OTLP_ENDPOINT")
	if otelEndpoint == "" {
		otelEndpoint = "localhost:4317"
	}
	otelShutdown, otelErr := adapter.InitOTelTracer(context.Background(), "openforge-server", otelEndpoint)
	if otelErr != nil {
		slog.Warn("OTel init failed; continuing without tracing", "err", otelErr)
	} else {
		slog.Info("OTel tracer initialised", "endpoint", otelEndpoint)
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := otelShutdown(shutdownCtx); err != nil {
				slog.Warn("OTel shutdown failed", "err", err)
			}
		}()
	}

	configPath := flag.String("config", "config/profiles/minimal.yaml", "profile config path")
	addr := flag.String("addr", ":8030", "listen address")
	flag.Parse()

	cfg, err := profile.Load(*configPath, false)
	if err != nil {
		log.Fatalf("failed to load profile: %v", err)
	}

	of, err := profile.Bootstrap(cfg)
	if err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}

	accessTTL, _ := time.ParseDuration(cfg.JWT.AccessTTL)
	if accessTTL == 0 {
		accessTTL = 1 * time.Hour
	}
	refreshTTL, _ := time.ParseDuration(cfg.JWT.RefreshTTL)
	if refreshTTL == 0 {
		refreshTTL = 24 * time.Hour
	}
	jwtSecret := cfg.JWT.Secret
	if envSecret := os.Getenv("OF_JWT_SECRET"); envSecret != "" {
		jwtSecret = envSecret
	}
	jwtSvc, err := service.NewJWTServiceWithValidation(jwtSecret, accessTTL, refreshTTL)
	if err != nil {
		log.Fatalf("JWT configuration error: %v", err)
	}

	mux := server.RegisterRoutes(of, jwtSvc, cfg)

	srv := &http.Server{
		Addr:         *addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("OpenForge server starting", "addr", *addr, "profile", cfg.Profile)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Path C T1: start gRPC server on :50051 (CoordinatorService +
	// GateService + ToolRegistryService + LLMRouterService). The handler
	// is a stub today; full business logic lands in T2 (Coordinator) and
	// T5 (Gate). ToolRegistry + LLMRouter are owned by the Node.js IO
	// process and exposed as noop stubs on the Go side for service
	// discovery / health probes.
	go func() {
		if err := server.StartGRPCServer(of, ":50051"); err != nil {
			slog.Error("gRPC server failed", "addr", ":50051", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down server")

	// G16: Call enterprise adapter shutdown hooks
	if of.Shutdown != nil {
		of.Shutdown()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
