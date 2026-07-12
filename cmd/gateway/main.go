package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/notion-clone-app/api-gateway/internal/app"
	"github.com/notion-clone-app/api-gateway/internal/config"
)

func main() {
	if err := run(); err != nil {
		log.Printf("Gateway stopped: %v", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.MustLoad()
	log.Printf("Starting gateway in [%s] mode...", cfg.Env)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	container, err := app.NewContainer(ctx, cfg)
	if err != nil {
		return fmt.Errorf("initialize gateway: %w", err)
	}
	defer container.Close()

	if err := container.Run(ctx); err != nil {
		return fmt.Errorf("run gateway: %w", err)
	}
	return nil
}
