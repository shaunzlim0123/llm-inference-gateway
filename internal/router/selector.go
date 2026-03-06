package router

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/shaunzlim0123/llm-inference-gateway/internal/config"
	"github.com/shaunzlim0123/llm-inference-gateway/pkg/openai"
)

// RouteDecision holds the selected provider and model.
type RouteDecision struct {
	ProviderName string
	Model        string
	Complexity   string
	Score        int
}

// Selector picks a provider/model based on complexity scoring.
type Selector struct {
	cfg       config.RoutingConfig
	providers map[string]config.ProviderConfig
}

func NewSelector(routingCfg config.RoutingConfig, providers map[string]config.ProviderConfig) *Selector {
	return &Selector{
		cfg:       routingCfg,
		providers: providers,
	}
}

// Select picks the best provider and model for the given request.
// If the request already specifies a model, it finds which provider serves it.
func (s *Selector) Select(_ context.Context, req *openai.ChatCompletionRequest, tenant *config.TenantConfig) (*RouteDecision, error) {
	// If client explicitly requested a model, route to its provider
	if req.Model != "" {
		return s.routeExplicitModel(req.Model, tenant)
	}

	// Score complexity
	score := ScoreComplexity(req.Messages)
	complexity := s.classifyScore(score)

	slog.Debug("complexity scored", "score", score, "complexity", complexity)

	// Find best matching model across providers in fallback order
	for _, provName := range s.cfg.FallbackOrder {
		pcfg, ok := s.providers[provName]
		if !ok {
			continue
		}

		for _, m := range pcfg.Models {
			if m.Complexity == complexity {
				if !s.tenantAllowsModel(tenant, m.Name) {
					continue
				}
				return &RouteDecision{
					ProviderName: provName,
					Model:        m.Name,
					Complexity:   complexity,
					Score:        score,
				}, nil
			}
		}
	}

	// Fallback: use first available model from first available provider
	for _, provName := range s.cfg.FallbackOrder {
		pcfg, ok := s.providers[provName]
		if !ok {
			continue
		}
		if len(pcfg.Models) > 0 {
			return &RouteDecision{
				ProviderName: provName,
				Model:        pcfg.Models[0].Name,
				Complexity:   complexity,
				Score:        score,
			}, nil
		}
	}

	return nil, fmt.Errorf("no suitable provider/model found for complexity %q", complexity)
}

func (s *Selector) classifyScore(score int) string {
	switch {
	case score <= s.cfg.Complexity.SimpleMaxScore:
		return "simple"
	case score <= s.cfg.Complexity.MediumMaxScore:
		return "medium"
	default:
		return "complex"
	}
}

func (s *Selector) routeExplicitModel(model string, tenant *config.TenantConfig) (*RouteDecision, error) {
	if !s.tenantAllowsModel(tenant, model) {
		return nil, fmt.Errorf("model %q not allowed for tenant", model)
	}

	for _, provName := range s.cfg.FallbackOrder {
		pcfg, ok := s.providers[provName]
		if !ok {
			continue
		}
		for _, m := range pcfg.Models {
			if m.Name == model {
				return &RouteDecision{
					ProviderName: provName,
					Model:        model,
					Complexity:   m.Complexity,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("model %q not found in any provider", model)
}

func (s *Selector) tenantAllowsModel(tenant *config.TenantConfig, model string) bool {
	if tenant == nil || len(tenant.AllowedModels) == 0 {
		return true // no restrictions
	}
	for _, m := range tenant.AllowedModels {
		if m == model {
			return true
		}
	}
	return false
}
