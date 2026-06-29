// Package schedhttp exposes the scheduler's deploy orchestration over HTTP so
// the control-plane API can trigger it. (NATS/Temporal-driven deploys replace
// this direct call later; the orchestration behind it is unchanged.)
package schedhttp

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/threemates/antariksh/services/scheduler/internal/orchestrator"
)

// Deployer is the orchestration entrypoint (satisfied by *orchestrator.Orchestrator).
type Deployer interface {
	Deploy(ctx context.Context, service, image string) (orchestrator.Result, error)
}

type deployReq struct {
	Service string `json:"service"`
	Image   string `json:"image"`
}

// Handler builds the scheduler's HTTP API.
func Handler(d Deployer) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("/internal/deploy", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var req deployReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Service == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "service is required"})
			return
		}
		res, err := d.Deploy(r.Context(), req.Service, req.Image)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, res)
	})

	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
