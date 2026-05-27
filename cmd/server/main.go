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
	"openforge/internal/server"
	"openforge/internal/shared/profile"
)

func main() {
	// Initialize structured logging
	logHandler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(logHandler))

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
	jwtSvc := service.NewJWTService(cfg.JWT.Secret, accessTTL, refreshTTL)

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
