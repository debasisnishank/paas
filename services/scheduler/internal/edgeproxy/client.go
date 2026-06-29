// Package edgeproxy is a client for the edge-proxy admin API, used by the
// scheduler to register/remove `host → backend` routes as deploys come and go.
package edgeproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	base string
	http *http.Client
}

// New builds a client for the admin API base URL, e.g. "http://127.0.0.1:9901".
func New(adminBase string) *Client {
	return &Client{base: adminBase, http: &http.Client{Timeout: 10 * time.Second}}
}

type routeRequest struct {
	Host     string   `json:"host"`
	Replicas []string `json:"replicas"`
}

// RegisterRoute upserts a host → replicas route.
func (c *Client) RegisterRoute(ctx context.Context, host string, replicas []string) error {
	body, err := json.Marshal(routeRequest{Host: host, Replicas: replicas})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/routes", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req)
}

// RemoveRoute deletes the route for host.
func (c *Client) RemoveRoute(ctx context.Context, host string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.base+"/routes/"+host, nil)
	if err != nil {
		return err
	}
	return c.do(req)
}

func (c *Client) do(req *http.Request) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", req.Method, req.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("%s %s: %s: %s", req.Method, req.URL, resp.Status, string(b))
	}
	return nil
}
