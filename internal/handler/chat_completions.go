package handler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/shaunzlim0123/llm-inference-gateway/internal/middleware"
	"github.com/shaunzlim0123/llm-inference-gateway/internal/provider"
	"github.com/shaunzlim0123/llm-inference-gateway/internal/router"
	"github.com/shaunzlim0123/llm-inference-gateway/pkg/openai"
)

type ChatCompletionHandler struct {
	registry *provider.Registry
	selector *router.Selector
	fallback []string
}

func NewChatCompletionHandler(registry *provider.Registry, selector *router.Selector, fallbackOrder []string) *ChatCompletionHandler {
	return &ChatCompletionHandler{
		registry: registry,
		selector: selector,
		fallback: fallbackOrder,
	}
}

func (h *ChatCompletionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req openai.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), "invalid_request_error")
		return
	}

	if len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "messages is required and must not be empty", "invalid_request_error")
		return
	}

	tenant := middleware.GetTenant(r.Context())

	// Route: select provider + model based on complexity
	decision, err := h.selector.Select(r.Context(), &req, tenant)
	if err != nil {
		slog.Error("routing failed", "error", err)
		writeError(w, http.StatusBadRequest, err.Error(), "invalid_request_error")
		return
	}

	slog.Info("routed request",
		"provider", decision.ProviderName,
		"model", decision.Model,
		"complexity", decision.Complexity,
		"score", decision.Score,
	)

	req.Model = decision.Model

	if req.Stream {
		h.handleStream(w, r, &req, decision)
	} else {
		h.handleNonStream(w, r, &req, decision)
	}
}

func (h *ChatCompletionHandler) handleNonStream(w http.ResponseWriter, r *http.Request, req *openai.ChatCompletionRequest, decision *router.RouteDecision) {
	providerOrder := h.buildProviderOrder(decision.ProviderName)

	var lastErr error
	for _, name := range providerOrder {
		p, err := h.registry.Get(name)
		if err != nil {
			continue
		}
		if !p.Healthy() {
			slog.Warn("skipping unhealthy provider", "provider", name)
			continue
		}

		resp, err := p.ChatCompletion(r.Context(), req)
		if err != nil {
			slog.Error("provider call failed", "provider", name, "error", err)
			lastErr = err
			continue
		}

		w.Header().Set("X-FlowGate-Provider", name)
		w.Header().Set("X-FlowGate-Model", decision.Model)
		w.Header().Set("X-FlowGate-Complexity", fmt.Sprintf("%s (score: %d)", decision.Complexity, decision.Score))
		writeJSON(w, http.StatusOK, resp)
		return
	}

	msg := "all providers failed"
	if lastErr != nil {
		msg = "all providers failed: " + lastErr.Error()
	}
	writeError(w, http.StatusBadGateway, msg, "server_error")
}

func (h *ChatCompletionHandler) handleStream(w http.ResponseWriter, r *http.Request, req *openai.ChatCompletionRequest, decision *router.RouteDecision) {
	providerOrder := h.buildProviderOrder(decision.ProviderName)

	var lastErr error
	for _, name := range providerOrder {
		p, err := h.registry.Get(name)
		if err != nil {
			continue
		}
		if !p.Healthy() {
			continue
		}

		resp, err := p.ChatCompletionStream(r.Context(), req)
		if err != nil {
			slog.Error("stream provider call failed", "provider", name, "error", err)
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-FlowGate-Provider", name)
		w.Header().Set("X-FlowGate-Model", decision.Model)
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "streaming not supported", "server_error")
			return
		}

		// Proxy SSE stream from provider to client
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintf(w, "%s\n", line)
			if line == "" {
				flusher.Flush()
			}
		}
		// Final flush
		io.WriteString(w, "\n")
		flusher.Flush()
		return
	}

	msg := "all providers failed"
	if lastErr != nil {
		msg = "all providers failed: " + lastErr.Error()
	}
	writeError(w, http.StatusBadGateway, msg, "server_error")
}

func (h *ChatCompletionHandler) buildProviderOrder(selected string) []string {
	order := []string{selected}
	for _, name := range h.fallback {
		if name != selected {
			order = append(order, name)
		}
	}
	return order
}
