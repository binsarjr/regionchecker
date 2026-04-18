// Package server exposes the regionchecker HTTP API.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
)

// Config bundles runtime knobs for the HTTP server.
type Config struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	RateLimit    int // requests per second per client IP
	MaxBatch     int
}

// Server is an http.Server wired with classifier, metrics, and middleware.
type Server struct {
	cfg      Config
	cls      Classifier
	dbAge    DBAgeFn
	metrics  *Metrics
	log      *slog.Logger
	httpSrv  *http.Server
	maxBatch int
}

// New constructs a Server. Pass a nil metrics to disable the Prometheus
// collectors; pass a nil dbAge to make /readyz always succeed.
func New(cfg Config, cls Classifier, dbAge DBAgeFn, metrics *Metrics, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	s := &Server{
		cfg:      cfg,
		cls:      cls,
		dbAge:    dbAge,
		metrics:  metrics,
		log:      log,
		maxBatch: cfg.MaxBatch,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/check", s.handleCheck)
	mux.HandleFunc("POST /v1/batch", s.handleBatch)
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.Handle("GET /metrics", promhttp.Handler())

	var handler http.Handler = mux
	handler = slogMiddleware(log, metrics)(handler)
	if cfg.RateLimit > 0 {
		handler = rateLimiter(rate.Limit(cfg.RateLimit))(handler)
	}
	handler = requestID(handler)

	s.httpSrv = &http.Server{
		Addr:         cfg.Addr,
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}
	return s
}

// Run starts the server and blocks until ctx is cancelled. A 15s graceful
// shutdown timeout applies on stop.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()
	select {
	case <-ctx.Done():
	case err := <-errCh:
		return err
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return s.httpSrv.Shutdown(shutdownCtx)
}

// Handler returns the fully-wrapped HTTP handler for use in httptest.
func (s *Server) Handler() http.Handler { return s.httpSrv.Handler }
