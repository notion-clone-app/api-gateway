package proxy

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type DynamicRegistry struct {
	mu              sync.RWMutex
	proxies         map[string]*httputil.ReverseProxy
	clusterDomain   string
	allowedServices map[string]bool
}

func NewDynamicRegistry(clusterDomain string, allowedServices []string) *DynamicRegistry {
	allowedMap := make(map[string]bool)
	for _, s := range allowedServices {
		allowedMap[s] = true
	}

	return &DynamicRegistry{
		proxies:         make(map[string]*httputil.ReverseProxy),
		clusterDomain:   clusterDomain,
		allowedServices: allowedMap,
	}
}

func (r *DynamicRegistry) GetOrCreateProxy(serviceName string) (*httputil.ReverseProxy, error) {
	r.mu.RLock()
	proxy, exists := r.proxies[serviceName]
	r.mu.RUnlock()

	if exists {
		return proxy, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if proxy, exists = r.proxies[serviceName]; exists {
		return proxy, nil
	}

	targetURL := fmt.Sprintf("http://%s:8080", serviceName)
	if r.clusterDomain != "" {
		targetURL = fmt.Sprintf("http://%s.%s:8080", serviceName, r.clusterDomain)
	}

	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	newProxy := httputil.NewSingleHostReverseProxy(target)
	newProxy.Transport = otelhttp.NewTransport(http.DefaultTransport)

	// Custom error handler to structured-log proxy errors and return clean JSON
	newProxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		slog.Error("reverse proxy connection error",
			"service", serviceName,
			"path", req.URL.Path,
			"error", err,
		)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"Service Temporarily Unavailable"}`))
	}

	originalDirector := newProxy.Director
	newProxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Header.Set("X-Forwarded-Host", req.Host)
		req.Host = target.Host
	}

	r.proxies[serviceName] = newProxy
	return newProxy, nil
}

func (r *DynamicRegistry) InterceptRoute() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		parts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
		if len(parts) < 3 {
			http.Error(w, "Invalid Routing Path", http.StatusBadRequest)
			return
		}

		serviceName := parts[2]

		// Whitelist validation prevents SSRF, cross-namespace routing, and unbounded memory consumption
		r.mu.RLock()
		allowed := r.allowedServices[serviceName]
		r.mu.RUnlock()

		if !allowed {
			slog.Warn("access denied to unmapped or blocked service", "serviceName", serviceName)
			http.Error(w, "Access Denied: Invalid Service Name", http.StatusForbidden)
			return
		}

		serviceProxy, err := r.GetOrCreateProxy(serviceName)
		if err != nil {
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
			return
		}

		serviceProxy.ServeHTTP(w, req)
	}
}
