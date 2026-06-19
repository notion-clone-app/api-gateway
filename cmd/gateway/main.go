package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/notion-clone-app/api-gateway/internal/app/docs"
	"github.com/notion-clone-app/api-gateway/internal/config"
	ssov1 "github.com/notion-clone-app/protos/gen/go/proto/sso"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	cfg := config.MustLoad()
	log.Printf("Starting gateway in [%s] mode...", cfg.Env)

	if err := run(cfg); err != nil {
		log.Printf("Critical error: %v", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	grpcMux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	err := ssov1.RegisterAuthHandlerFromEndpoint(ctx, grpcMux, cfg.SSOService.GRPCAddress, opts)
	if err != nil {
		return err
	}

	mainRouter := http.NewServeMux()

	docs.RegisterRoutes(mainRouter, ssov1.SwaggerJSON)
	mainRouter.Handle("/", grpcMux)

	srv := &http.Server{
		Addr:         cfg.HTTP.Port,
		Handler:      mainRouter,
		ReadTimeout:  cfg.HTTP.Timeout,
		WriteTimeout: cfg.HTTP.Timeout,
	}

	go func() {
		log.Printf("HTTP Gateway is running on %s", cfg.HTTP.Port)
		log.Printf("API Documentation is available on http://localhost%s/docs", cfg.HTTP.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("listen error: %s\n", err)
			cancel()
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down gateway gracefully...")
	if err := srv.Shutdown(ctx); err != nil {
		return err
	}

	return nil
}
