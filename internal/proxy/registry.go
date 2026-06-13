package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type DynamicRegistry struct {
	mu            sync.RWMutex
	proxies       map[string]*httputil.ReverseProxy
	clusterDomain string
}

func NewDynamicRegistry(clusterDomain string) *DynamicRegistry {
	return &DynamicRegistry{
		proxies:       make(map[string]*httputil.ReverseProxy),
		clusterDomain: clusterDomain,
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

		if !strings.HasSuffix(serviceName, "-service") {
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
