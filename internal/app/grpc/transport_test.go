package grpc

import (
	"testing"

	"github.com/notion-clone-app/api-gateway/internal/config"
)

func TestBuildRoutesRejectsMissingUpstream(t *testing.T) {
	_, connections, err := buildRoutes(&config.Config{
		Upstreams: map[string]config.UpstreamConfig{},
	})
	if err == nil {
		t.Fatal("buildRoutes() expected an error")
	}
	if len(connections) != 0 {
		t.Fatalf("connections = %d", len(connections))
	}
}
