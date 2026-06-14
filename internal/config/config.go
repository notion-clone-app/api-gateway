package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port            string
	ClusterDomain   string   // Базовый DNS суффикс для K8s, например: staging.svc.cluster.local
	OtelCollector   string   // Адрес коллектора треков, например: localhost:4317
	AllowedServices []string // Разрешенные к проксированию микросервисы
}

func MustLoad() (*Config, error) {
	otelURL := getEnv("OTEL_COLLECTOR_URL", "")
	if otelURL == "" {
		otelURL = getEnv("OTEL_COLLECTOR", "localhost:4317")
	}

	allowedServicesStr := getEnv("ALLOWED_SERVICES", "auth-service,mock-service,user-service,notion-service")
	var allowedServices []string
	for _, s := range strings.Split(allowedServicesStr, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			allowedServices = append(allowedServices, s)
		}
	}

	cfg := &Config{
		Port:            getEnv("PORT", "8080"),
		ClusterDomain:   getEnv("CLUSTER_DOMAIN", ""), // По умолчанию локально стучимся напрямую по имени
		OtelCollector:   otelURL,
		AllowedServices: allowedServices,
	}

	if cfg.Port == "" {
		return nil, fmt.Errorf("PORT variable is missing")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
