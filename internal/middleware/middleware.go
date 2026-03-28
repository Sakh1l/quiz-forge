package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/quizforge/quiz-forge/internal/config"
	"github.com/quizforge/quiz-forge/internal/logger"
)

type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	LoggerKey    contextKey = "logger"
)

func RequestID(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		ctx = context.WithValue(ctx, RequestIDKey, requestID)

		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

func GetRequestID(ctx context.Context) string {
	if id := ctx.Value(RequestIDKey); id != nil {
		if str, ok := id.(string); ok {
			return str
		}
	}
	return ""
}

func Logger(cfg *config.Config) func(next http.Handler) http.Handler {
	log := logger.Get()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			requestID := GetRequestID(r.Context())

			if cfg.IsDevelopment() {
				log.Debug("request started",
					"request_id", requestID,
					"method", r.Method,
					"path", r.URL.Path,
					"remote_addr", r.RemoteAddr,
				)
			}

			ww := newStatusWriter(w)
			next.ServeHTTP(ww, r)

			duration := time.Since(start)
			status := ww.Status()

			logData := []any{
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"duration_ms", duration.Milliseconds(),
				"bytes_written", ww.BytesWritten(),
			}

			if status >= 500 {
				log.Error("request completed with server error", logData...)
			} else if status >= 400 {
				log.Warn("request completed with client error", logData...)
			} else if cfg.IsDevelopment() {
				log.Info("request completed", logData...)
			} else {
				log.Debug("request completed", logData...)
			}
		})
	}
}

func Recoverer(cfg *config.Config) func(next http.Handler) http.Handler {
	log := logger.Get()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					requestID := GetRequestID(r.Context())

					logData := []any{
						"request_id", requestID,
						"method", r.Method,
						"path", r.URL.Path,
						"error", err,
					}

					if cfg.IsDevelopment() {
						log.Error("panic recovered (dev mode - stack trace available)",
							append(logData, "recover", recover())...,
						)
					} else {
						log.Error("panic recovered",
							logData...,
						)
					}

					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func RealIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rip := r.Header.Get("X-Forwarded-For"); rip != "" {
			r.Header.Set("X-Real-IP", rip)
		}
		next.ServeHTTP(w, r)
	})
}

func CORS(cfg *config.Config) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			allowed := false
			for _, allowedOrigin := range cfg.CORS.AllowedOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				if origin != "" && origin != "*" {
					w.Header().Set("Vary", "Origin")
				}
			}

			if r.Method == "OPTIONS" {
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.CORS.AllowedMethods, ","))
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.CORS.AllowedHeaders, ","))
				w.Header().Set("Access-Control-Max-Age", "86400")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RateLimit(cfg *config.Config) func(next http.Handler) http.Handler {
	if !cfg.Rate.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	log := logger.Get()

	type client struct {
		tokens    float64
		lastCheck time.Time
	}

	clients := make(map[string]*client)
	var mu sync.Mutex

	refillRate := float64(cfg.Rate.RequestsPerSecond)

	cleanup := time.NewTicker(5 * time.Minute)
	go func() {
		for range cleanup.C {
			mu.Lock()
			now := time.Now()
			for ip, c := range clients {
				if now.Sub(c.lastCheck) > 10*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
				ip = forwarded
			}

			mu.Lock()
			c, exists := clients[ip]
			if !exists {
				c = &client{
					tokens:    float64(cfg.Rate.Burst),
					lastCheck: time.Now(),
				}
				clients[ip] = c
			}

			now := time.Now()
			elapsed := now.Sub(c.lastCheck).Seconds()
			c.tokens += elapsed * refillRate
			if c.tokens > float64(cfg.Rate.Burst) {
				c.tokens = float64(cfg.Rate.Burst)
			}
			c.lastCheck = now

			if c.tokens < 1 {
				mu.Unlock()
				log.Warn("rate limit exceeded", "ip", ip)
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			c.tokens--
			mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

func generateRequestID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

type statusWriter struct {
	http.ResponseWriter
	status       int
	bytesWritten int
}

func newStatusWriter(w http.ResponseWriter) *statusWriter {
	return &statusWriter{ResponseWriter: w, status: http.StatusOK}
}

func (sw *statusWriter) WriteHeader(status int) {
	sw.status = status
	sw.ResponseWriter.WriteHeader(status)
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	sw.bytesWritten += len(b)
	return sw.ResponseWriter.Write(b)
}

func (sw *statusWriter) Status() int {
	return sw.status
}

func (sw *statusWriter) BytesWritten() int {
	return sw.bytesWritten
}
