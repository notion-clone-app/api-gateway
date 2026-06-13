package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

type GatewayProxy struct {
	reverseProxy *httputil.ReverseProxy
	targetHost   string
}

// NewGatewayProxy создает объект прокси для конкретного микросервиса
func NewGatewayProxy(targetURL string) (*GatewayProxy, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Настраиваем Director для корректной передачи заголовков
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
	}

	// Кастомная обработка ошибок (если микросервис недоступен)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "Service Temporarily Unavailable", http.StatusServiceUnavailable)
	}

	return &GatewayProxy{
		reverseProxy: proxy,
		targetHost:   target.Host,
	}, nil
}

// Handler возвращает HTTP-обработчик для использования в роутере Chi
func (p *GatewayProxy) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p.reverseProxy.ServeHTTP(w, r)
	}
}
