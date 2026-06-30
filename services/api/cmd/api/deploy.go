package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/threemates/antariksh/services/api/internal/deploystore"
	"github.com/threemates/antariksh/services/api/internal/domain"
	"github.com/threemates/antariksh/services/api/internal/scheduler"
)

type deployRequest struct {
	Builder string `json:"builder"`
	Region  string `json:"region"`
	Env     string `json:"env"`
	Image   string `json:"image"`
}

// deployResponse is a Deployment plus the resolved public URL (flattened JSON).
type deployResponse struct {
	domain.Deployment
	URL string `json:"url,omitempty"`
}

// maxDeployMemory is how much of a multipart upload is buffered in memory before
// the rest spills to a temp file (32 MiB).
const maxDeployMemory = 32 << 20

// deployHandler records a deploy intent and, if a scheduler is configured, asks
// it to boot the microVM and route a URL to it. A multipart upload carrying a
// `source` tarball triggers a build-from-source deploy; a JSON body (or empty
// body) boots the default rootfs. With no scheduler it just records the pending
// intent (the scheduler/NATS path fills in later).
func deployHandler(store *deploystore.Store, sched *scheduler.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svcID := domain.ServiceID(chi.URLParam(r, "serviceID"))
		if svcID == "" {
			writeJSONError(w, http.StatusBadRequest, "missing service id")
			return
		}

		var (
			req    deployRequest
			source io.Reader // non-nil → build-from-source
		)
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
			if err := r.ParseMultipartForm(maxDeployMemory); err != nil {
				writeJSONError(w, http.StatusBadRequest, "parse upload: "+err.Error())
				return
			}
			req.Image = r.FormValue("image")
			req.Region = r.FormValue("region")
			req.Env = r.FormValue("env")
			req.Builder = r.FormValue("builder")
			file, _, err := r.FormFile("source")
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "missing source file: "+err.Error())
				return
			}
			defer func() { _ = file.Close() }()
			source = file
		} else if r.Body != nil {
			// An empty body is allowed — defaults apply.
			_ = json.NewDecoder(r.Body).Decode(&req)
		}

		envName := req.Env
		if envName == "" {
			envName = "production"
		}

		dep := domain.Deployment{
			ID:        domain.DeploymentID(newID("dep")),
			ServiceID: svcID,
			EnvID:     domain.EnvID(envName),
			Image:     req.Image,
			Status:    domain.DeployPending,
			CreatedAt: time.Now().UTC(),
		}

		var url string
		if sched != nil {
			var (
				res scheduler.Result
				err error
			)
			if source != nil {
				res, err = sched.DeployWithSource(r.Context(), string(svcID), req.Image, source)
			} else {
				res, err = sched.Deploy(r.Context(), string(svcID), req.Image)
			}
			if err != nil {
				slog.Error("scheduler deploy failed", "service", svcID, "err", err)
			} else {
				dep.Status = domain.DeployLive
				url = res.URL
			}
		}
		store.Create(dep)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(deployResponse{Deployment: dep, URL: url})
	}
}

// listDeploymentsHandler returns a service's deployments, newest first.
func listDeploymentsHandler(store *deploystore.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svcID := domain.ServiceID(chi.URLParam(r, "serviceID"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deployments": store.ListByService(svcID),
		})
	}
}

// newID returns a short unique id like "dep_9f3a1c4e7b2d8a06".
func newID(prefix string) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return prefix + "_" + hex.EncodeToString(b[:])
}
