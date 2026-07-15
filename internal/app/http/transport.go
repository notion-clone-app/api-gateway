package http

import (
	"context"
	"errors"
	"fmt"
	"log"
	stdhttp "net/http"
	"strings"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/notion-clone-app/api-gateway/internal/auth"
	"github.com/notion-clone-app/api-gateway/internal/config"
	"github.com/notion-clone-app/api-gateway/internal/registry"
	ssov1 "github.com/notion-clone-app/protos/gen/go/proto/sso"
	_ "google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GRPCHandler interface {
	ServeHTTP(stdhttp.ResponseWriter, *stdhttp.Request)
}

// Transport owns the public HTTP listener, HTTP/JSON gateway and OpenAPI routes.
type Transport struct {
	server *stdhttp.Server
}

func New(
	ctx context.Context,
	cfg *config.Config,
	validator auth.Validator,
	grpcHandler GRPCHandler,
) (*Transport, error) {
	authCookies, err := newAuthCookies(cfg.Auth.Cookies)
	if err != nil {
		return nil, fmt.Errorf("configure auth cookies: %w", err)
	}

	gateway := runtime.NewServeMux(
		runtime.WithDisableHTTPMethodOverride(),
		runtime.WithMetadata(authCookies.requestMetadata),
		runtime.WithForwardResponseOption(authCookies.writeResponse),
	)

	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(auth.UnaryClientInterceptor(validator, registry.AuthenticationMode)),
	}
	if err := registerGatewayServices(ctx, gateway, cfg, dialOptions); err != nil {
		return nil, err
	}

	router := stdhttp.NewServeMux()
	registerDocs(router, ssov1.SwaggerJSON)
	registerHealth(router)
	router.Handle("/", gateway)

	rootHandler := stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if isGRPCRequest(r) {
			grpcHandler.ServeHTTP(w, r)
			return
		}
		router.ServeHTTP(w, r)
	})
	corsHandler, err := newCORSMiddleware(cfg.HTTP.CORS, rootHandler)
	if err != nil {
		return nil, fmt.Errorf("configure CORS: %w", err)
	}

	protocols := new(stdhttp.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	return &Transport{server: &stdhttp.Server{
		Addr:              cfg.HTTP.Port,
		Handler:           corsHandler,
		ReadTimeout:       cfg.HTTP.Timeout,
		WriteTimeout:      cfg.HTTP.Timeout,
		ReadHeaderTimeout: cfg.HTTP.Timeout,
		IdleTimeout:       60 * time.Second,
		Protocols:         protocols,
	}}, nil
}

func (t *Transport) Run(ctx context.Context) error {
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("HTTP and gRPC gateway is running on %s", t.server.Addr)
		log.Printf("API documentation is available on http://localhost%s/docs", t.server.Addr)
		serverErr <- t.server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if errors.Is(err, stdhttp.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
	}

	log.Println("Shutting down gateway gracefully...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := t.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

func registerGatewayServices(
	ctx context.Context,
	mux *runtime.ServeMux,
	cfg *config.Config,
	options []grpc.DialOption,
) error {
	for _, service := range registry.PublicHTTPServices() {
		upstream, exists := cfg.Upstreams[service.Upstream]
		if !exists {
			return fmt.Errorf(
				"service %q references unknown upstream %q",
				service.Name,
				service.Upstream,
			)
		}
		if err := service.Register(ctx, mux, upstream.GRPCAddress, options); err != nil {
			return fmt.Errorf("register service %q: %w", service.Name, err)
		}
	}
	return nil
}

func isGRPCRequest(r *stdhttp.Request) bool {
	return r.ProtoMajor == 2 && strings.HasPrefix(
		strings.ToLower(r.Header.Get("Content-Type")),
		"application/grpc",
	)
}

func registerHealth(mux *stdhttp.ServeMux) {
	mux.HandleFunc("/healthz", func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		w.WriteHeader(stdhttp.StatusOK)
	})
	mux.HandleFunc("/ready", func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		w.WriteHeader(stdhttp.StatusOK)
	})
}
