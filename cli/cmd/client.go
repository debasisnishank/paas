package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/viper"
)

// authToken returns the stored bearer token, if any (from ~/.antctl.yaml or
// ANTARIKSH_TOKEN).
func authToken() string { return viper.GetString("token") }

func newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, apiBase()+path, body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if t := authToken(); t != "" {
		req.Header.Set("Authorization", "Bearer "+t)
	}
	return req, nil
}

func do(req *http.Request, out any) error {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", req.Method, req.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%s %s: %s: %s", req.Method, req.URL, resp.Status, string(body))
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// apiGet issues an authenticated GET and decodes the JSON body into out.
func apiGet(ctx context.Context, path string, out any) error {
	req, err := newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	return do(req, out)
}

// apiPost issues a POST with a JSON body and decodes the JSON response into out.
func apiPost(ctx context.Context, path string, body, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	req, err := newRequest(ctx, http.MethodPost, path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return do(req, out)
}
