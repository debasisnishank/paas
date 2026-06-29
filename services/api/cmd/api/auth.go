package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/threemates/antariksh/services/api/internal/auth"
)

type ctxKey string

const claimsCtxKey ctxKey = "claims"

// tokenTTL is how long an issued bearer token stays valid.
const tokenTTL = 24 * time.Hour

type loginRequest struct {
	Email  string `json:"email"`
	APIKey string `json:"api_key"`
}

type loginResponse struct {
	Token     string `json:"token"`
	Email     string `json:"email"`
	ExpiresAt int64  `json:"expires_at"`
}

// loginHandler exchanges an email + API key for a signed bearer token.
func loginHandler(secret []byte, apiKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Email == "" || req.APIKey == "" {
			writeJSONError(w, http.StatusBadRequest, "email and api_key are required")
			return
		}
		// constant-time compare so a wrong key can't be timed out character by character
		if subtle.ConstantTimeCompare([]byte(req.APIKey), []byte(apiKey)) != 1 {
			writeJSONError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		now := time.Now()
		tok, err := auth.Issue(secret, req.Email, tokenTTL, now)
		if err != nil {
			slog.Error("issue token", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "could not issue token")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(loginResponse{
			Token:     tok,
			Email:     req.Email,
			ExpiresAt: now.Add(tokenTTL).Unix(),
		})
	}
}

// authMiddleware rejects requests without a valid `Authorization: Bearer` token
// and stashes the verified claims in the request context.
func authMiddleware(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || token == "" {
				writeJSONError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}
			claims, err := auth.Verify(secret, token, time.Now())
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}
			ctx := context.WithValue(r.Context(), claimsCtxKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
