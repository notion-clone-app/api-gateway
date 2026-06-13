package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port          string
	ClusterDomain string // Базовый DNS суффикс для K8s, например: staging.svc.cluster.local
	OtelCollector string // Адрес коллектора треков, например: localhost:4317
}

func MustLoad() (*Config, error) {
	cfg := &Config{
		Port:          getEnv("PORT", "8080"),
		ClusterDomain: getEnv("CLUSTER_DOMAIN", ""), // По умолчанию локально стучимся напрямую по имени
		OtelCollector: getEnv("OTEL_COLLECTOR_URL", "localhost:4317"),
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
