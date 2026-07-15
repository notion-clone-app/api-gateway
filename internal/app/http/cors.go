package http

import (
	"fmt"
	stdhttp "net/http"
	"strings"

	"github.com/notion-clone-app/api-gateway/internal/config"
)

const (
	allowedMethods = "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS"
	allowedHeaders = "Authorization, Content-Type, Accept, X-Request-Id"
	exposedHeaders = "Grpc-Status, Grpc-Message, X-Request-Id"
)

type corsMiddleware struct {
	next             stdhttp.Handler
	allowedOrigins   map[string]struct{}
	allowAll         bool
	allowCredentials bool
}

func newCORSMiddleware(cfg config.CORSConfig, next stdhttp.Handler) (stdhttp.Handler, error) {
	middleware := &corsMiddleware{
		next:             next,
		allowedOrigins:   make(map[string]struct{}, len(cfg.AllowedOrigins)),
		allowCredentials: cfg.AllowCredentials,
	}

	for _, configuredOrigin := range cfg.AllowedOrigins {
		origin := strings.TrimSpace(configuredOrigin)
		if origin == "" {
			return nil, fmt.Errorf("allowed origin must not be empty")
		}
		if origin == "*" {
			middleware.allowAll = true
			continue
		}
		middleware.allowedOrigins[origin] = struct{}{}
	}

	if middleware.allowAll && middleware.allowCredentials {
		return nil, fmt.Errorf("wildcard origin cannot be combined with credentials")
	}

	return middleware, nil
}

func (m *corsMiddleware) ServeHTTP(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		m.next.ServeHTTP(w, r)
		return
	}

	allowOrigin, allowed := m.resolveOrigin(origin)
	if !allowed {
		stdhttp.Error(w, "origin is not allowed", stdhttp.StatusForbidden)
		return
	}

	appendVary(w.Header(), "Origin")
	w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
	w.Header().Set("Access-Control-Expose-Headers", exposedHeaders)
	if m.allowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	if r.Method == stdhttp.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
		appendVary(w.Header(), "Access-Control-Request-Method")
		appendVary(w.Header(), "Access-Control-Request-Headers")
		w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
		w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
		w.Header().Set("Access-Control-Max-Age", "600")
		w.WriteHeader(stdhttp.StatusNoContent)
		return
	}

	m.next.ServeHTTP(w, r)
}

func (m *corsMiddleware) resolveOrigin(origin string) (string, bool) {
	if m.allowAll {
		return "*", true
	}
	_, allowed := m.allowedOrigins[origin]
	return origin, allowed
}

func appendVary(header stdhttp.Header, value string) {
	for _, current := range header.Values("Vary") {
		for item := range strings.SplitSeq(current, ",") {
			if strings.EqualFold(strings.TrimSpace(item), value) {
				return
			}
		}
	}
	header.Add("Vary", value)
}
