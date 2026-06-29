package edgeproxy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterRoutePostsBody(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody routeRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL)
	if err := c.RegisterRoute(context.Background(), "app.local", []string{"172.16.0.2:80"}); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotPath != "/routes" {
		t.Errorf("got %s %s", gotMethod, gotPath)
	}
	if gotBody.Host != "app.local" || len(gotBody.Replicas) != 1 || gotBody.Replicas[0] != "172.16.0.2:80" {
		t.Errorf("body = %+v", gotBody)
	}
}

func TestRemoveRouteDeletes(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := New(srv.URL).RemoveRoute(context.Background(), "app.local"); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/routes/app.local" {
		t.Errorf("got %s %s", gotMethod, gotPath)
	}
}

func TestNon2xxIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	if err := New(srv.URL).RegisterRoute(context.Background(), "h", []string{"x"}); err == nil {
		t.Fatal("expected error on 400")
	}
}
