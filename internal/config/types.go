package config

import "time"

type Config struct {
	Server        ServerConfig              `yaml:"server"`
	Redis         RedisConfig               `yaml:"redis"`
	Cache         CacheConfig               `yaml:"cache"`
	Providers     map[string]ProviderConfig `yaml:"providers"`
	Routing       RoutingConfig             `yaml:"routing"`
	Tenants       []TenantConfig            `yaml:"tenants"`
	Observability ObservabilityConfig       `yaml:"observability"`
}

type ServerConfig struct {
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
}

type CacheConfig struct {
	Enabled             bool          `yaml:"enabled"`
	SimilarityThreshold float64       `yaml:"similarity_threshold"`
	TTL                 time.Duration `yaml:"ttl"`
	EmbeddingModel      string        `yaml:"embedding_model"`
	EmbeddingDimensions int           `yaml:"embedding_dimensions"`
}

type ProviderConfig struct {
	APIKey         string              `yaml:"api_key"`
	BaseURL        string              `yaml:"base_url"`
	Models         []ModelConfig       `yaml:"models"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
}

type ModelConfig struct {
	Name            string  `yaml:"name"`
	Complexity      string  `yaml:"complexity"`
	CostPer1kInput  float64 `yaml:"cost_per_1k_input"`
	CostPer1kOutput float64 `yaml:"cost_per_1k_output"`
}

type CircuitBreakerConfig struct {
	MaxRequests      uint32        `yaml:"max_requests"`
	Interval         time.Duration `yaml:"interval"`
	Timeout          time.Duration `yaml:"timeout"`
	FailureThreshold uint32        `yaml:"failure_threshold"`
}

type RoutingConfig struct {
	Strategy      string            `yaml:"strategy"`
	Complexity    ComplexityConfig  `yaml:"complexity"`
	FallbackOrder []string          `yaml:"fallback_order"`
}

type ComplexityConfig struct {
	SimpleMaxScore int `yaml:"simple_max_score"`
	MediumMaxScore int `yaml:"medium_max_score"`
}

type TenantConfig struct {
	ID               string           `yaml:"id"`
	APIKey           string           `yaml:"api_key"`
	Name             string           `yaml:"name"`
	TokenBudget      TokenBudget      `yaml:"token_budget"`
	RateLimit        RateLimitConfig  `yaml:"rate_limit"`
	AllowedModels    []string         `yaml:"allowed_models"`
	DefaultModel     string           `yaml:"default_model"`
	MaxContextTokens int              `yaml:"max_context_tokens"`
}

type TokenBudget struct {
	DailyLimit   int64 `yaml:"daily_limit"`
	MonthlyLimit int64 `yaml:"monthly_limit"`
}

type RateLimitConfig struct {
	RequestsPerMinute int `yaml:"requests_per_minute"`
}

type ObservabilityConfig struct {
	MetricsPath string `yaml:"metrics_path"`
	LogLevel    string `yaml:"log_level"`
	LogFormat   string `yaml:"log_format"`
}
