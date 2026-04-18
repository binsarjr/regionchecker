package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ctxKey int

const ctxRequestID ctxKey = 1

// requestID generates a 16-byte random hex identifier per request,
// stores it on ctx, and echoes it via the X-Request-Id response header.
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			var b [16]byte
			_, _ = rand.Read(b[:])
			id = hex.EncodeToString(b[:])
		}
		w.Header().Set("X-Request-Id", id)
		ctx := context.WithValue(r.Context(), ctxRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext reports the stored request id or empty string.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxRequestID).(string)
	return v
}

// slogMiddleware logs structured request/response entries.
func slogMiddleware(log *slog.Logger, m *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, code: 200}
			next.ServeHTTP(rec, r)
			dur := time.Since(start)
			if m != nil {
				m.HTTPRequests.WithLabelValues(r.URL.Path, strconv.Itoa(rec.code)).Inc()
				m.HTTPDuration.WithLabelValues(r.URL.Path).Observe(dur.Seconds())
			}
			log.Info("http",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.code),
				slog.Duration("duration", dur),
				slog.String("request_id", RequestIDFromContext(r.Context())),
				slog.String("remote", clientIP(r)))
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
}

// rateLimiter builds a per-client-IP rate limiter middleware.
// Each IP gets a token bucket with burst = limit, refill = limit/s.
func rateLimiter(limit rate.Limit) func(http.Handler) http.Handler {
	mu := sync.Mutex{}
	buckets := make(map[string]*rate.Limiter, 1024)
	limiter := func(ip string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		l, ok := buckets[ip]
		if !ok {
			l = rate.NewLimiter(limit, int(limit))
			buckets[ip] = l
		}
		return l
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter(clientIP(r)).Allow() {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
