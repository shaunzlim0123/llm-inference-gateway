package observability

import "github.com/prometheus/client_golang/prometheus"

var (
	RequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "flowgate_request_duration_seconds",
			Help:    "Request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status", "tenant"},
	)

	TokensTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "flowgate_tokens_total",
			Help: "Total tokens consumed.",
		},
		[]string{"tenant", "provider", "model", "type"},
	)

	CacheHitsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "flowgate_cache_hits_total",
			Help: "Total cache hits.",
		},
		[]string{"cache_type"},
	)

	CacheMissesTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "flowgate_cache_misses_total",
			Help: "Total cache misses.",
		},
	)

	ProviderErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "flowgate_provider_errors_total",
			Help: "Total provider errors.",
		},
		[]string{"provider"},
	)

	CostDollarsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "flowgate_cost_dollars_total",
			Help: "Estimated cost in dollars.",
		},
		[]string{"tenant", "provider", "model"},
	)

	CircuitBreakerState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "flowgate_circuit_breaker_state",
			Help: "Circuit breaker state (0=closed, 1=half-open, 2=open).",
		},
		[]string{"provider"},
	)
)

func init() {
	prometheus.MustRegister(
		RequestDuration,
		TokensTotal,
		CacheHitsTotal,
		CacheMissesTotal,
		ProviderErrorsTotal,
		CostDollarsTotal,
		CircuitBreakerState,
	)
}
