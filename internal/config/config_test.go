package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	yaml := `
server:
  port: 9090
providers:
  openai:
    api_key: "test-key"
    base_url: "https://api.openai.com/v1"
    models:
      - name: "gpt-4o-mini"
        complexity: simple
tenants:
  - id: "tenant-acme"
    api_key: "acme-key"
    name: "Acme"
`
	f, err := os.CreateTemp("", "flowgate-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString(yaml)
	f.Close()

	cfg, err := Load(f.Name())
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}

	if cfg.Providers["openai"].APIKey != "test-key" {
		t.Errorf("expected openai api key 'test-key', got %s", cfg.Providers["openai"].APIKey)
	}

	// Check defaults were applied
	if cfg.Observability.LogLevel != "info" {
		t.Errorf("expected default log level 'info', got %s", cfg.Observability.LogLevel)
	}

	if cfg.Routing.Complexity.SimpleMaxScore != 30 {
		t.Errorf("expected default simple_max_score 30, got %d", cfg.Routing.Complexity.SimpleMaxScore)
	}
}

func TestEnvOverride(t *testing.T) {
	yaml := `
providers:
  openai:
    api_key: "from-yaml"
    base_url: "https://api.openai.com/v1"
tenants:
  - id: "tenant-acme"
    api_key: "from-yaml"
`
	f, err := os.CreateTemp("", "flowgate-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString(yaml)
	f.Close()

	t.Setenv("OPENAI_API_KEY", "from-env")
	t.Setenv("TENANT_ACME_API_KEY", "acme-from-env")

	cfg, err := Load(f.Name())
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Providers["openai"].APIKey != "from-env" {
		t.Errorf("expected OPENAI_API_KEY env override, got %s", cfg.Providers["openai"].APIKey)
	}

	if cfg.Tenants[0].APIKey != "acme-from-env" {
		t.Errorf("expected tenant API key env override, got %s", cfg.Tenants[0].APIKey)
	}
}
