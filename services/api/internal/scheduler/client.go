// Package scheduler is the API's client for the scheduler service, used to turn
// a recorded deploy intent into a running microVM + routed URL.
package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

type Client struct {
	base string
	http *http.Client
	// build is a long-timeout client for source deploys (docker build + boot).
	build *http.Client
}

// New builds a client for the scheduler base URL, e.g. "http://127.0.0.1:7070".
func New(base string) *Client {
	return &Client{
		base:  base,
		http:  &http.Client{Timeout: 60 * time.Second},
		build: &http.Client{Timeout: 15 * time.Minute},
	}
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

// DeployWithSource asks the scheduler to build a rootfs from the uploaded source
// tarball (gzip+tar) and boot it. The tarball is streamed (no buffering) as a
// multipart body, so source size is bounded only by the scheduler's cap.
func (c *Client) DeployWithSource(ctx context.Context, service, image string, source io.Reader) (Result, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		var werr error
		defer func() { _ = pw.CloseWithError(werr) }()
		if werr = mw.WriteField("service", service); werr != nil {
			return
		}
		if werr = mw.WriteField("image", image); werr != nil {
			return
		}
		fw, err := mw.CreateFormFile("source", "source.tar.gz")
		if err != nil {
			werr = err
			return
		}
		if _, werr = io.Copy(fw, source); werr != nil {
			return
		}
		werr = mw.Close()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/internal/deploy", pr)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := c.build.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("POST %s/internal/deploy (source): %w", c.base, err)
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
