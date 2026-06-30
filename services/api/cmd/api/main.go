package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/threemates/antariksh/services/api/internal/deploystore"
	"github.com/threemates/antariksh/services/api/internal/scheduler"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	port := envOr("PORT", "8080")

	authSecret := []byte(envOr("API_AUTH_SECRET", "dev-insecure-secret-change-me"))
	apiKey := envOr("API_KEY", "dev-api-key")
	if string(authSecret) == "dev-insecure-secret-change-me" || apiKey == "dev-api-key" {
		log.Warn("using insecure default auth credentials — set API_AUTH_SECRET and API_KEY in production")
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	// No global request timeout: the deploy route builds + boots a microVM,
	// which can take minutes. It carries its own (longer) timeout; see
	// registerV1Routes. Fast routes return immediately anyway.

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	deploys := deploystore.New()
	var sched *scheduler.Client
	if u := envOr("SCHEDULER_URL", ""); u != "" {
		sched = scheduler.New(u)
		log.Info("scheduler wired", "url", u)
	}
	r.Route("/v1", v1Router(authSecret, apiKey, deploys, sched))

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: r,
		// No Read/Write timeouts: deploy streams a source upload and then waits
		// on a build+boot (minutes). ReadHeaderTimeout still guards slowloris;
		// per-route timeouts bound the rest.
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Info("api listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("shutdown error", "err", err)
	}
	log.Info("api stopped")
}

// v1Router wires the /v1 API: a public login route plus a token-protected group
// holding every resource endpoint.
func v1Router(authSecret []byte, apiKey string, deploys *deploystore.Store, sched *scheduler.Client) func(chi.Router) {
	return func(r chi.Router) {
		// Public: exchange credentials for a bearer token.
		r.Post("/auth/login", loginHandler(authSecret, apiKey))

		// Everything else requires a valid bearer token.
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware(authSecret))
			registerV1Routes(r, deploys, sched)
		})
	}
}

// deployTimeout bounds a build-from-source deploy (docker build + microVM boot).
const deployTimeout = 20 * time.Minute

func registerV1Routes(r chi.Router, deploys *deploystore.Store, sched *scheduler.Client) {
	r.Get("/regions", regionsHandler)

	r.Route("/orgs/{orgSlug}", func(r chi.Router) {
		r.Route("/projects", func(r chi.Router) {
			r.Get("/", stubHandler("list-projects"))
			r.Post("/", stubHandler("create-project"))
			r.Route("/{projectID}", func(r chi.Router) {
				r.Get("/", stubHandler("get-project"))
				r.Route("/services", func(r chi.Router) {
					r.Get("/", stubHandler("list-services"))
					r.Post("/", stubHandler("create-service"))
					r.Route("/{serviceID}", func(r chi.Router) {
						r.Get("/", stubHandler("get-service"))
						// Deploy is long-running (build + boot); give it a
						// generous timeout instead of the default request budget.
						r.With(middleware.Timeout(deployTimeout)).
							Post("/deploy", deployHandler(deploys, sched))
						r.Get("/deployments", listDeploymentsHandler(deploys))
						r.Get("/logs", stubHandler("stream-logs"))
					})
				})
				r.Route("/envs", func(r chi.Router) {
					r.Get("/", stubHandler("list-envs"))
					r.Post("/", stubHandler("create-env"))
				})
				r.Route("/databases", func(r chi.Router) {
					r.Get("/", stubHandler("list-dbs"))
					r.Post("/", stubHandler("create-db"))
					r.Route("/{dbID}", func(r chi.Router) {
						r.Get("/", stubHandler("get-db"))
						r.Post("/branches", stubHandler("create-branch"))
						r.Get("/branches", stubHandler("list-branches"))
						r.Post("/restore", stubHandler("restore-pitr"))
					})
				})
			})
		})
	})
}

func stubHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = fmt.Fprintf(w, `{"endpoint":%q,"status":"not_implemented"}`+"\n", name)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
