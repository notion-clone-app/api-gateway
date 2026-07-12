package grpc

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/notion-clone-app/api-gateway/internal/auth"
	"github.com/notion-clone-app/api-gateway/internal/config"
	grpcproxy "github.com/notion-clone-app/api-gateway/internal/proxy"
	"github.com/notion-clone-app/api-gateway/internal/registry"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Transport owns the native gRPC server and its upstream connections.
type Transport struct {
	server      *googlegrpc.Server
	connections []*googlegrpc.ClientConn
	closeOnce   sync.Once
}

func New(cfg *config.Config, validator auth.Validator) (*Transport, error) {
	routes, connections, err := buildRoutes(cfg)
	if err != nil {
		return nil, err
	}

	proxyHandler := grpcproxy.NewHandler(routes, validator)
	server := googlegrpc.NewServer(
		grpcproxy.Codec(),
		googlegrpc.UnknownServiceHandler(proxyHandler.Handle),
	)

	return &Transport{
		server:      server,
		connections: connections,
	}, nil
}

// ServeHTTP allows native gRPC and HTTP/JSON to share one HTTP/2 listener.
func (t *Transport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.server.ServeHTTP(w, r)
}

func (t *Transport) Close() {
	t.closeOnce.Do(func() {
		t.server.Stop()
		for _, connection := range t.connections {
			_ = connection.Close()
		}
	})
}

func buildRoutes(cfg *config.Config) (map[string]grpcproxy.Route, []*googlegrpc.ClientConn, error) {
	routes := make(map[string]grpcproxy.Route)
	connections := make([]*googlegrpc.ClientConn, 0)
	byUpstream := make(map[string]*googlegrpc.ClientConn)

	closeConnections := func() {
		for _, connection := range connections {
			_ = connection.Close()
		}
	}

	for _, service := range registry.PublicGRPCServices() {
		upstream, exists := cfg.Upstreams[service.Upstream]
		if !exists {
			closeConnections()
			return nil, nil, fmt.Errorf(
				"service %q references unknown upstream %q",
				service.Name,
				service.Upstream,
			)
		}

		connection := byUpstream[service.Upstream]
		if connection == nil {
			var err error
			connection, err = googlegrpc.NewClient(
				upstream.GRPCAddress,
				googlegrpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				closeConnections()
				return nil, nil, fmt.Errorf("connect upstream %q: %w", service.Upstream, err)
			}
			byUpstream[service.Upstream] = connection
			connections = append(connections, connection)
		}

		if _, duplicate := routes[service.GRPCService]; duplicate {
			closeConnections()
			return nil, nil, fmt.Errorf("duplicate public service %q", service.GRPCService)
		}

		routes[service.GRPCService] = grpcproxy.Route{
			Connection:  connection,
			RequireAuth: service.AuthRequired,
		}
	}

	return routes, connections, nil
}
