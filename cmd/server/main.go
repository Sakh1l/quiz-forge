package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/quizforge/quiz-forge/internal/config"
	"github.com/quizforge/quiz-forge/internal/logger"
	"github.com/quizforge/quiz-forge/internal/router"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	cfg := config.Load()

	log := logger.Init(cfg)
	defer log.Info("server shutdown complete")

	log.Info("starting Quiz Forge",
		"version", version,
		"build_time", buildTime,
		"environment", cfg.Env,
		"log_level", cfg.LogLevel,
	)

	if cfg.IsDevelopment() {
		log.Warn("running in development mode - verbose logging enabled")
	}

	r := router.New(cfg)

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)

	server := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("server listening",
			"address", addr,
			"host_dashboard", fmt.Sprintf("http://localhost:%s/host", cfg.Port),
			"join_url", fmt.Sprintf("http://localhost:%s/join/ROOMCODE", cfg.Port),
		)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	log.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	log.Info("server stopped gracefully")
}

func init() {
	flag.Parse()
	if v := os.Getenv("VERSION"); v != "" {
		version = v
	}
	if t := os.Getenv("BUILD_TIME"); t != "" {
		buildTime = t
	}
}
