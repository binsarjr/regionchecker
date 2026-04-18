package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/binsarjr/regionchecker/internal/classifier"
)

// Classifier is the subset of classifier.Classifier used by the HTTP handlers.
type Classifier interface {
	Classify(ctx context.Context, input string) (*classifier.Result, error)
}

// DBAgeFn reports the age of the parsed RIR snapshot.
type DBAgeFn func() (time.Duration, error)

const (
	defaultMaxBatch = 1000
	maxAgeForReady  = 48 * time.Hour
)

func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	if host == "" {
		http.Error(w, "missing host param", http.StatusBadRequest)
		return
	}
	res, err := s.cls.Classify(r.Context(), host)
	if err != nil {
		httpErrorForClassifier(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

type batchRequest struct {
	Hosts      []string `json:"hosts"`
	Country    string   `json:"country,omitempty"`
	Confidence string   `json:"confidence,omitempty"`
}

func (s *Server) handleBatch(w http.ResponseWriter, r *http.Request) {
	var req batchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json: "+err.Error(), http.StatusBadRequest)
		return
	}
	max := s.maxBatch
	if max == 0 {
		max = defaultMaxBatch
	}
	if len(req.Hosts) > max {
		http.Error(w, "batch too large", http.StatusRequestEntityTooLarge)
		return
	}
	results := make([]*classifier.Result, 0, len(req.Hosts))
	for _, h := range req.Hosts {
		res, err := s.cls.Classify(r.Context(), h)
		if err != nil {
			results = append(results, &classifier.Result{Input: h, Reason: err.Error()})
			continue
		}
		if req.Country != "" && res.FinalCountry != req.Country {
			continue
		}
		results = append(results, res)
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	if s.dbAge == nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	age, err := s.dbAge()
	if err != nil {
		http.Error(w, "db age unknown", http.StatusServiceUnavailable)
		return
	}
	if s.metrics != nil {
		s.metrics.DBAgeSeconds.Set(age.Seconds())
	}
	if age > maxAgeForReady {
		http.Error(w, "db stale", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func httpErrorForClassifier(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, classifier.ErrInvalidInput):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, classifier.ErrBogon):
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, classifier.ErrUnresolvable):
		http.Error(w, err.Error(), http.StatusNotFound)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
