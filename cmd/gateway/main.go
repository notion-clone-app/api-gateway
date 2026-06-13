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

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/notion-clone-app/api-gateway/internal/config"
	"github.com/notion-clone-app/api-gateway/internal/proxy"
	"github.com/notion-clone-app/api-gateway/internal/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.MustLoad()

	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	tp, err := telemetry.InitTracer(context.Background(), "api-gateway", cfg.OtelCollector)
	if err != nil {
		slog.Warn("telemetry failed to init, running without tracing", "error", err)
	} else {
		defer tp.Shutdown(context.Background())
	}

	registry := proxy.NewDynamicRegistry(cfg.ClusterDomain)

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	r.HandleFunc("/api/{version}/{serviceName}/*", registry.InterceptRoute())

	otelHandler := otelhttp.NewHandler(r, "api-gateway-root")

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      otelHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		slog.Info("API Gateway is online", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server fatal error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful Shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	slog.Info("shutting down gateway...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", "error", err)
	}

	slog.Info("gateway exited cleanly")
}
