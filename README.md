# FlowGate

An LLM inference gateway that unifies multiple AI providers behind a single OpenAI-compatible API. Route requests across OpenAI, Anthropic, and Ollama with automatic complexity-based model selection, circuit breaker failover, semantic caching, and per-tenant budget controls.

## Features

- **Unified API** — Single `/v1/chat/completions` endpoint compatible with the OpenAI API format, supporting both standard and streaming (SSE) responses
- **Multi-provider routing** — OpenAI, Anthropic, and Ollama providers with automatic protocol translation
- **Complexity-based model selection** — Heuristic scoring analyzes message length, keyword patterns, and code blocks to route simple queries to cheaper models and complex queries to more capable ones
- **Circuit breaker failover** — Per-provider circuit breakers (via [gobreaker](https://github.com/sony/gobreaker)) with configurable failure thresholds and automatic recovery, plus ordered failover chains with exponential backoff retries
- **Semantic caching** — Two-tier caching with Redis: exact-match fast path plus vector similarity search using OpenAI embeddings for near-duplicate queries
- **Per-tenant rate limiting & budgets** — API key authentication with per-tenant rate limits (requests/minute) and daily/monthly token budget enforcement via Redis
- **Observability** — Prometheus metrics (request latency, token usage, provider errors, cache hits), structured JSON logging, and request ID correlation

## Architecture

```
Client
  |
  v
[RequestID] -> [Metrics] -> [Logger] -> [Auth] -> [RateLimit] -> [Cache] -> [Handler]
                                                                                |
                                                                          [Selector]
                                                                          (complexity
                                                                            scoring)
                                                                                |
                                                                    +-----------+-----------+
                                                                    |           |           |
                                                                 [OpenAI]  [Anthropic]  [Ollama]
                                                                    |           |           |
                                                                 [Circuit   [Circuit    [Circuit
                                                                  Breaker]   Breaker]    Breaker]
```

## Quick Start

### Prerequisites

- Go 1.25+
- Redis 8+ (for caching and rate limiting)
- At least one provider API key (OpenAI, Anthropic) or a local Ollama instance

### Run with Docker Compose

```bash
export OPENAI_API_KEY=sk-...
export ANTHROPIC_API_KEY=sk-ant-...
export TENANT_ACME_API_KEY=your-tenant-key

make docker-up
```

This starts FlowGate, Redis, and Prometheus. The gateway is available at `http://localhost:8080`.

### Run locally

```bash
# Edit config/flowgate.yaml with your API keys and tenant keys
make run
```

### Make a request

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_ACME_API_KEY" \
  -d '{
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

The gateway automatically selects a model based on query complexity. To target a specific model, include the `model` field as usual.

### Streaming

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TENANT_ACME_API_KEY" \
  -d '{
    "messages": [{"role": "user", "content": "Explain quicksort"}],
    "stream": true
  }'
```

## Configuration

FlowGate is configured via YAML (default: `config/flowgate.yaml`, override with `FLOWGATE_CONFIG` env var).

| Section | Key options |
|---|---|
| `server` | `port`, `read_timeout`, `write_timeout` |
| `redis` | `addr`, `password` |
| `cache` | `enabled`, `similarity_threshold`, `ttl`, `embedding_model` |
| `providers` | Per-provider `api_key`, `base_url`, `models` (with complexity tier and cost), `circuit_breaker` settings |
| `routing` | `strategy` (`complexity`), score thresholds, `fallback_order` |
| `tenants` | `api_key`, `token_budget` (daily/monthly limits), `rate_limit`, `allowed_models` |
| `observability` | `metrics_path`, `log_level`, `log_format` |

See [`config/flowgate.yaml`](config/flowgate.yaml) for a complete example.

## Complexity Routing

When no `model` is specified, FlowGate scores the request on a 0-100 scale:

| Factor | Impact |
|---|---|
| Message count | +3 per message |
| Estimated tokens | +5 / +15 / +30 based on length |
| Complex keywords (`analyze`, `architecture`, `implement`, ...) | +4 to +10 |
| Simple keywords (`hello`, `yes`, `thanks`, ...) | -5 |
| Code blocks | +10 |

The score maps to a complexity tier (`simple` / `medium` / `complex`) based on configurable thresholds, and the gateway selects the cheapest model in that tier.

## Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/v1/chat/completions` | Yes | Chat completion (standard + streaming) |
| `GET` | `/health` | No | Liveness check |
| `GET` | `/ready` | No | Readiness check |
| `GET` | `/metrics` | No | Prometheus metrics |

## Project Structure

```
cmd/flowgate/          Entry point
internal/
  config/              YAML config loading and types
  handler/             HTTP handlers (chat completions, health)
  middleware/          Request ID, logging, auth, rate limiting, metrics, cache
  provider/            Provider implementations (OpenAI, Anthropic, Ollama) + circuit breaker
  router/              Complexity scoring and model selection
  cache/               Exact-match and semantic cache with Redis
  budget/              Token budget tracking via Redis
  observability/       Prometheus metric definitions
pkg/openai/            OpenAI-compatible request/response types
config/                Default configuration
deployments/           Dockerfile, docker-compose, Prometheus config
```

## Development

```bash
make build          # Build binary to bin/flowgate
make run            # Build and run
make test           # Run all tests
make clean          # Remove build artifacts
make docker-up      # Start all services with Docker Compose
make docker-down    # Stop all services
```

## License

MIT
