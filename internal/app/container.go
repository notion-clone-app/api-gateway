package app

import (
	"context"
	"fmt"

	grpctransport "github.com/notion-clone-app/api-gateway/internal/app/grpc"
	httptransport "github.com/notion-clone-app/api-gateway/internal/app/http"
	"github.com/notion-clone-app/api-gateway/internal/auth"
	"github.com/notion-clone-app/api-gateway/internal/config"
)

// Container is the application's manual dependency injection container.
// Construction order is explicit, which keeps dependencies easy to replace in tests.
type Container struct {
	validator auth.Validator
	grpc      *grpctransport.Transport
	http      *httptransport.Transport
}

func NewContainer(ctx context.Context, cfg *config.Config) (*Container, error) {
	validator, err := auth.NewHMACValidator(
		cfg.Auth.HMACSecret,
		cfg.Auth.Issuer,
		cfg.Auth.Audience,
	)
	if err != nil {
		return nil, fmt.Errorf("configure authentication: %w", err)
	}

	grpcTransport, err := grpctransport.New(cfg, validator)
	if err != nil {
		return nil, fmt.Errorf("configure gRPC transport: %w", err)
	}

	httpTransport, err := httptransport.New(ctx, cfg, validator, grpcTransport)
	if err != nil {
		grpcTransport.Close()
		return nil, fmt.Errorf("configure HTTP transport: %w", err)
	}

	return &Container{
		validator: validator,
		grpc:      grpcTransport,
		http:      httpTransport,
	}, nil
}

func (c *Container) Run(ctx context.Context) error {
	return c.http.Run(ctx)
}

func (c *Container) Close() {
	c.grpc.Close()
}
