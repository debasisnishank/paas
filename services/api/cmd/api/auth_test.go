package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoginIssuesToken(t *testing.T) {
	secret := []byte("test-secret")
	h := loginHandler(secret, "good-key")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login",
		strings.NewReader(`{"email":"a@b.com","api_key":"good-key"}`))
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body)
	}
	var resp loginResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Token == "" {
		t.Error("expected a token")
	}
	if resp.Email != "a@b.com" {
		t.Errorf("email = %q", resp.Email)
	}
}

func TestLoginRejectsBadKey(t *testing.T) {
	h := loginHandler([]byte("s"), "good-key")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login",
		strings.NewReader(`{"email":"a@b.com","api_key":"wrong"}`))
	h(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestAuthMiddleware(t *testing.T) {
	secret := []byte("test-secret")
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	guarded := authMiddleware(secret)(ok)

	// mint a real token via the login handler
	loginRec := httptest.NewRecorder()
	loginHandler(secret, "k")(loginRec, httptest.NewRequest(http.MethodPost, "/v1/auth/login",
		strings.NewReader(`{"email":"a@b.com","api_key":"k"}`)))
	var lr loginResponse
	_ = json.NewDecoder(loginRec.Body).Decode(&lr)

	t.Run("valid token passes", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/regions", nil)
		req.Header.Set("Authorization", "Bearer "+lr.Token)
		guarded.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
	})

	t.Run("missing token is 401", func(t *testing.T) {
		rec := httptest.NewRecorder()
		guarded.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/regions", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("garbage token is 401", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/regions", nil)
		req.Header.Set("Authorization", "Bearer not.a.jwt")
		guarded.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})
}
