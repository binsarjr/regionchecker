package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/binsarjr/regionchecker/internal/classifier"
	"github.com/binsarjr/regionchecker/internal/server"
)

type stubClassifier struct {
	mu      sync.Mutex
	results map[string]*classifier.Result
	errs    map[string]error
}

func (s *stubClassifier) Classify(_ context.Context, input string) (*classifier.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err, ok := s.errs[input]; ok {
		return nil, err
	}
	if r, ok := s.results[input]; ok {
		return r, nil
	}
	return nil, classifier.ErrInvalidInput
}

func newSrv(t *testing.T, age time.Duration, ageErr error) (*httptest.Server, *stubClassifier) {
	t.Helper()
	cls := &stubClassifier{
		results: map[string]*classifier.Result{
			"8.8.8.8": {Input: "8.8.8.8", FinalCountry: "US", Confidence: "ip-only"},
		},
		errs: map[string]error{
			"10.0.0.1":   classifier.ErrBogon,
			"nope.local": classifier.ErrUnresolvable,
		},
	}
	dbAge := func() (time.Duration, error) { return age, ageErr }
	srv := server.New(server.Config{
		MaxBatch: 1000,
	}, cls, dbAge, nil, nil)
	return httptest.NewServer(srv.Handler()), cls
}

func TestCheck_OK(t *testing.T) {
	srv, _ := newSrv(t, time.Hour, nil)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/v1/check?host=8.8.8.8")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var got classifier.Result
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.FinalCountry != "US" {
		t.Errorf("FinalCountry = %q, want US", got.FinalCountry)
	}
	if resp.Header.Get("X-Request-Id") == "" {
		t.Error("missing X-Request-Id header")
	}
}

func TestCheck_MissingHost(t *testing.T) {
	srv, _ := newSrv(t, time.Hour, nil)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/v1/check")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCheck_Bogon422(t *testing.T) {
	srv, _ := newSrv(t, time.Hour, nil)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/v1/check?host=10.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", resp.StatusCode)
	}
}

func TestCheck_Unresolvable404(t *testing.T) {
	srv, _ := newSrv(t, time.Hour, nil)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/v1/check?host=nope.local")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestBatch_OK(t *testing.T) {
	srv, _ := newSrv(t, time.Hour, nil)
	defer srv.Close()
	body := `{"hosts":["8.8.8.8","10.0.0.1"]}`
	resp, err := http.Post(srv.URL+"/v1/batch", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var out struct {
		Results []classifier.Result `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Results) != 2 {
		t.Errorf("results = %d, want 2", len(out.Results))
	}
}

func TestBatch_TooLarge(t *testing.T) {
	srv, _ := newSrv(t, time.Hour, nil)
	defer srv.Close()
	hosts := make([]string, 1001)
	for i := range hosts {
		hosts[i] = "x"
	}
	body, _ := json.Marshal(map[string]any{"hosts": hosts})
	resp, err := http.Post(srv.URL+"/v1/batch", "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", resp.StatusCode)
	}
}

func TestHealthz(t *testing.T) {
	srv, _ := newSrv(t, time.Hour, nil)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestReadyz_Fresh(t *testing.T) {
	srv, _ := newSrv(t, time.Hour, nil)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestReadyz_Stale(t *testing.T) {
	srv, _ := newSrv(t, 72*time.Hour, nil)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
}

func TestReadyz_AgeErr(t *testing.T) {
	srv, _ := newSrv(t, 0, errors.New("no meta"))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
}
