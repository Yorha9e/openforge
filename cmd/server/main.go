package main

import (
	"context"
	"flag"
	"log"
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
		log.Printf("OpenForge server listening on %s (profile: %s)", *addr, cfg.Profile)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
