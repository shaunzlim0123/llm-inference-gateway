package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricsRegistered(t *testing.T) {
	// Verify that all metrics are registered and can be gathered
	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}

	expected := map[string]bool{
		"flowgate_request_duration_seconds": false,
		"flowgate_tokens_total":             false,
		"flowgate_cache_hits_total":         false,
		"flowgate_cache_misses_total":       false,
		"flowgate_provider_errors_total":    false,
		"flowgate_cost_dollars_total":       false,
		"flowgate_circuit_breaker_state":    false,
	}

	for _, mf := range metrics {
		if _, ok := expected[mf.GetName()]; ok {
			expected[mf.GetName()] = true
		}
	}

	// Note: metrics without observations won't appear in Gather().
	// We verify the metrics are functional by writing a value and checking.

	// Write test values
	RequestDuration.WithLabelValues("POST", "/v1/chat/completions", "200", "test-tenant").Observe(0.5)
	TokensTotal.WithLabelValues("test-tenant", "openai", "gpt-4o", "prompt").Add(100)
	CacheHitsTotal.WithLabelValues("exact").Inc()
	CacheMissesTotal.Inc()
	ProviderErrorsTotal.WithLabelValues("openai").Inc()
	CostDollarsTotal.WithLabelValues("test-tenant", "openai", "gpt-4o").Add(0.01)
	CircuitBreakerState.WithLabelValues("openai").Set(0)

	// Gather again and verify
	metrics, err = prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics after writing: %v", err)
	}

	found := make(map[string]bool)
	for _, mf := range metrics {
		found[mf.GetName()] = true
	}

	for name := range expected {
		if !found[name] {
			t.Errorf("metric %q not found after writing a value", name)
		}
	}
}

func TestRequestDuration_Labels(t *testing.T) {
	// Verify we can create observers with specific label values
	obs := RequestDuration.WithLabelValues("GET", "/health", "200", "")
	obs.Observe(0.001)
	obs.Observe(0.050)

	// Gather and verify the metric exists with observations
	metrics, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("failed to gather: %v", err)
	}

	found := false
	for _, mf := range metrics {
		if mf.GetName() == "flowgate_request_duration_seconds" {
			found = true
			break
		}
	}
	if !found {
		t.Error("flowgate_request_duration_seconds not found after observing values")
	}
}
