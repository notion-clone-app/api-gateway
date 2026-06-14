package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
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

	registry := proxy.NewDynamicRegistry(cfg.ClusterDomain, cfg.AllowedServices)

	// Атомарный флаг готовности (1 = готов, 0 = не готов)
	// Используем atomic для потокобезопасного переключения между горутинами
	var isReady int32 = 1

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// Liveness Probe: Жив ли сам процесс приложения
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Readiness Probe: Готов ли сервис принимать трафик извне
	r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&isReady) == 1 {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Ready"))
			return
		}
		// Если поймали SIGTERM — отвечаем 503, сообщая K8s, что трафик сюда слать нельзя
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Shutting Down"))
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

	slog.Info("Received stop signal. Initiating graceful shutdown...")

	// 1. Мгновенно переключаем готовность. Новые проверки K8s начнут возвращать 503.
	atomic.StoreInt32(&isReady, 0)
	slog.Info("Service marked as NOT ready. Waiting for K8s to update routing tables...")

	// 2. Делаем паузу (обычно 5-10 секунд).
	// Это критически важно в Kubernetes: K8s должен успеть обновить свои DNS/iptables
	// и перестать перенаправлять новые запросы на этот под.
	time.Sleep(5 * time.Second)

	slog.Info("Stopping HTTP server and draining active requests...")

	// 3. Плавно закрываем сервер, давая 10 секунд на обработку старых, уже начавшихся запросов.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown due to timeout", "error", err)
	}

	slog.Info("gateway exited cleanly")
}
