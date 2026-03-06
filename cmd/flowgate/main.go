package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/shaunzlim0123/llm-inference-gateway/internal/budget"
	"github.com/shaunzlim0123/llm-inference-gateway/internal/cache"
	"github.com/shaunzlim0123/llm-inference-gateway/internal/config"
	"github.com/shaunzlim0123/llm-inference-gateway/internal/handler"
	"github.com/shaunzlim0123/llm-inference-gateway/internal/middleware"
	_ "github.com/shaunzlim0123/llm-inference-gateway/internal/observability" // register metrics
	"github.com/shaunzlim0123/llm-inference-gateway/internal/provider"
	"github.com/shaunzlim0123/llm-inference-gateway/internal/router"
)

func main() {
	// Load config
	cfgPath := "config/flowgate.yaml"
	if v := os.Getenv("FLOWGATE_CONFIG"); v != "" {
		cfgPath = v
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Setup structured logging
	setupLogging(cfg.Observability)

	// Connect to Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
	})

	// Build provider registry with circuit breakers
	registry := provider.NewRegistry()
	registerProviders(registry, cfg)

	// Build selector + handler
	sel := router.NewSelector(cfg.Routing, cfg.Providers)
	chatHandler := handler.NewChatCompletionHandler(registry, sel, cfg.Routing.FallbackOrder)

	// Build budget tracker
	tracker := budget.NewTracker(rdb)

	// Build caches
	var exactCache *cache.ExactCache
	var semanticCache *cache.SemanticCache
	if cfg.Cache.Enabled {
		exactCache = cache.NewExactCache(rdb, cfg.Cache.TTL)

		// Embedder uses OpenAI API for embeddings
		openaiCfg, ok := cfg.Providers["openai"]
		if ok && openaiCfg.APIKey != "" {
			embedder := cache.NewEmbedder(
				openaiCfg.APIKey,
				openaiCfg.BaseURL,
				cfg.Cache.EmbeddingModel,
				cfg.Cache.EmbeddingDimensions,
			)
			semanticCache = cache.NewSemanticCache(rdb, embedder, cfg.Cache.SimilarityThreshold, cfg.Cache.TTL)
		} else {
			slog.Warn("semantic cache disabled: no OpenAI API key for embeddings")
		}
	}

	// Build router
	r := chi.NewRouter()

	// Middleware chain (order matters)
	r.Use(middleware.RequestID)  // 1. Assign correlation ID
	r.Use(middleware.Metrics)    // 2. Start timer — captures full lifecycle
	r.Use(middleware.Logger)     // 3. Log request — with request ID

	// Health + metrics endpoints (no auth required)
	r.Get("/health", handler.Health)
	r.Get("/ready", handler.Ready)
	r.Handle(cfg.Observability.MetricsPath, promhttp.Handler())

	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(cfg.Tenants))    // 4. API key → tenant
		r.Use(middleware.RateLimit(tracker))    // 5. Budget check

		// 6. Semantic cache (if enabled)
		if cfg.Cache.Enabled && exactCache != nil && semanticCache != nil {
			r.Use(middleware.SemanticCacheMiddleware(exactCache, semanticCache))
		}

		r.Post("/v1/chat/completions", chatHandler.ServeHTTP)
	})

	// Start server
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		slog.Info("shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("server shutdown error", "error", err)
		}
		rdb.Close()
	}()

	slog.Info("starting FlowGate", "port", cfg.Server.Port)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}

func registerProviders(registry *provider.Registry, cfg *config.Config) {
	for name, pcfg := range cfg.Providers {
		var p provider.Provider
		switch name {
		case "openai":
			if pcfg.APIKey == "" {
				slog.Warn("skipping provider (no API key)", "provider", name)
				continue
			}
			p = provider.NewOpenAI(pcfg)
		case "anthropic":
			if pcfg.APIKey == "" {
				slog.Warn("skipping provider (no API key)", "provider", name)
				continue
			}
			p = provider.NewAnthropic(pcfg)
		case "ollama":
			p = provider.NewOllama(pcfg)
		default:
			slog.Warn("unknown provider type", "provider", name)
			continue
		}

		guarded := provider.NewGuardedProvider(p, pcfg.CircuitBreaker)
		registry.Register(guarded)
		slog.Info("registered provider", "provider", name)
	}
}

func setupLogging(cfg config.ObservabilityConfig) {
	var level slog.Level
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var h slog.Handler
	if cfg.LogFormat == "json" {
		h = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		h = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	slog.SetDefault(slog.New(h))
}
