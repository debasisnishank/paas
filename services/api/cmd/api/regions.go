package main

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/threemates/antariksh/services/api/internal/regions"
)

// regionsResponse is the GET /v1/regions wire envelope.
type regionsResponse struct {
	Regions []regions.Entry `json:"regions"`
}

// regionsHandler serves the platform region catalog with jurisdiction profiles.
func regionsHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(regionsResponse{Regions: regions.Catalog()}); err != nil {
		slog.Error("encode regions response", "err", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
	}
}
