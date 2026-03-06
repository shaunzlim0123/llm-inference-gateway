package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	applyDefaults(cfg)
	applyEnvOverrides(cfg)

	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 30 * time.Second
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 120 * time.Second
	}
	if cfg.Redis.Addr == "" {
		cfg.Redis.Addr = "localhost:6379"
	}
	if cfg.Cache.SimilarityThreshold == 0 {
		cfg.Cache.SimilarityThreshold = 0.92
	}
	if cfg.Cache.TTL == 0 {
		cfg.Cache.TTL = 1 * time.Hour
	}
	if cfg.Cache.EmbeddingModel == "" {
		cfg.Cache.EmbeddingModel = "text-embedding-3-small"
	}
	if cfg.Cache.EmbeddingDimensions == 0 {
		cfg.Cache.EmbeddingDimensions = 512
	}
	if cfg.Observability.MetricsPath == "" {
		cfg.Observability.MetricsPath = "/metrics"
	}
	if cfg.Observability.LogLevel == "" {
		cfg.Observability.LogLevel = "info"
	}
	if cfg.Observability.LogFormat == "" {
		cfg.Observability.LogFormat = "json"
	}
	if cfg.Routing.Strategy == "" {
		cfg.Routing.Strategy = "complexity"
	}
	if cfg.Routing.Complexity.SimpleMaxScore == 0 {
		cfg.Routing.Complexity.SimpleMaxScore = 30
	}
	if cfg.Routing.Complexity.MediumMaxScore == 0 {
		cfg.Routing.Complexity.MediumMaxScore = 70
	}
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		cfg.Redis.Addr = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}

	// Provider API keys
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		if p, ok := cfg.Providers["openai"]; ok {
			p.APIKey = v
			cfg.Providers["openai"] = p
		}
	}
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
		if p, ok := cfg.Providers["anthropic"]; ok {
			p.APIKey = v
			cfg.Providers["anthropic"] = p
		}
	}

	// Tenant API keys
	for i := range cfg.Tenants {
		envKey := fmt.Sprintf("TENANT_%s_API_KEY", toEnvName(cfg.Tenants[i].ID))
		if v := os.Getenv(envKey); v != "" {
			cfg.Tenants[i].APIKey = v
		}
	}
}

// toEnvName converts "tenant-acme" to "ACME" for env var lookup.
func toEnvName(id string) string {
	// Strip "tenant-" prefix if present
	const prefix = "tenant-"
	name := id
	if len(name) > len(prefix) && name[:len(prefix)] == prefix {
		name = name[len(prefix):]
	}

	result := make([]byte, 0, len(name))
	for i := range name {
		c := name[i]
		if c == '-' || c == '.' {
			result = append(result, '_')
		} else if c >= 'a' && c <= 'z' {
			result = append(result, c-32)
		} else {
			result = append(result, c)
		}
	}
	return string(result)
}
