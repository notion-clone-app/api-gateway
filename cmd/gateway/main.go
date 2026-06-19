package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/notion-clone-app/api-gateway/internal/config"
	ssov1 "github.com/notion-clone-app/protos/gen/go/proto/sso"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	cfg := config.MustLoad()
	log.Printf("Starting gateway in [%s] mode...", cfg.Env)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	err := ssov1.RegisterAuthHandlerFromEndpoint(ctx, mux, cfg.SSOService.GRPCAddress, opts)
	if err != nil {
		log.Fatalf("failed to register SSO handler: %v", err)
	}

	srv := &http.Server{
		Addr:         cfg.HTTP.Port,
		Handler:      mux,
		ReadTimeout:  cfg.HTTP.Timeout,
		WriteTimeout: cfg.HTTP.Timeout,
	}

	go func() {
		log.Printf("HTTP Gateway is running on %s", cfg.HTTP.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen error: %s\n", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down gateway gracefully...")
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
}
