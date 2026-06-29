// Package scheduler is the API's client for the scheduler service, used to turn
// a recorded deploy intent into a running microVM + routed URL.
package scheduler

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

// New builds a client for the scheduler base URL, e.g. "http://127.0.0.1:7070".
func New(base string) *Client {
	return &Client{base: base, http: &http.Client{Timeout: 60 * time.Second}}
}

type deployReq struct {
	Service string `json:"service"`
	Image   string `json:"image"`
}

// Result mirrors the scheduler's deploy response.
type Result struct {
	Host    string `json:"host"`
	URL     string `json:"url"`
	GuestIP string `json:"guest_ip"`
	TapDev  string `json:"tap_dev"`
}

// Deploy asks the scheduler to boot a microVM for service and route a URL to it.
func (c *Client) Deploy(ctx context.Context, service, image string) (Result, error) {
	body, err := json.Marshal(deployReq{Service: service, Image: image})
	if err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/internal/deploy", bytes.NewReader(body))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("POST %s/internal/deploy: %w", c.base, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return Result{}, fmt.Errorf("scheduler deploy: %s: %s", resp.Status, string(b))
	}
	var res Result
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return Result{}, fmt.Errorf("decode scheduler response: %w", err)
	}
	return res, nil
}
