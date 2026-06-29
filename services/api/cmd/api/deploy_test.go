package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/threemates/antariksh/services/api/internal/deploystore"
	"github.com/threemates/antariksh/services/api/internal/domain"
)

func deployRouter(store *deploystore.Store) *chi.Mux {
	r := chi.NewRouter()
	base := "/v1/orgs/{orgSlug}/projects/{projectID}/services/{serviceID}"
	r.Post(base+"/deploy", deployHandler(store, nil))
	r.Get(base+"/deployments", listDeploymentsHandler(store))
	return r
}

func TestDeployCreatesPendingDeployment(t *testing.T) {
	store := deploystore.New()
	r := deployRouter(store)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost,
		"/v1/orgs/acme/projects/backend/services/web/deploy",
		strings.NewReader(`{"region":"in-mum-1","builder":"nixpacks"}`))
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (%s)", rec.Code, rec.Body)
	}
	var dep domain.Deployment
	if err := json.NewDecoder(rec.Body).Decode(&dep); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dep.ID == "" {
		t.Error("expected a deployment id")
	}
	if dep.ServiceID != "web" {
		t.Errorf("service id = %q, want web", dep.ServiceID)
	}
	if dep.Status != domain.DeployPending {
		t.Errorf("status = %q, want pending", dep.Status)
	}
	if dep.EnvID != "production" {
		t.Errorf("env = %q, want production (default)", dep.EnvID)
	}
}

func TestListDeploymentsReturnsCreated(t *testing.T) {
	store := deploystore.New()
	r := deployRouter(store)

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost,
			"/v1/orgs/acme/projects/backend/services/web/deploy",
			strings.NewReader(`{}`))
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("deploy %d: status %d", i, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/v1/orgs/acme/projects/backend/services/web/deployments", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	var resp struct {
		Deployments []domain.Deployment `json:"deployments"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Deployments) != 2 {
		t.Fatalf("got %d deployments, want 2", len(resp.Deployments))
	}
}
