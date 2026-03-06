package handler

import (
	"encoding/json"
	"net/http"

	"github.com/shaunzlim0123/llm-inference-gateway/pkg/openai"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg, errType string) {
	writeJSON(w, status, openai.ErrorResponse{
		Error: openai.ErrorDetail{
			Message: msg,
			Type:    errType,
		},
	})
}
