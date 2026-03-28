//go:build dev || debug
// +build dev debug

package handler

import (
	"net/http"
	"runtime"
	"time"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) RegisterDebugRoutes(r *chi.Mux) {
	r.Get("/debug/info", h.DebugInfo)
	r.Get("/debug/config", h.DebugConfig)
}

func (h *Handler) DebugInfo(w http.ResponseWriter, r *http.Request) {
	info := map[string]interface{}{
		"version":       "dev",
		"go_version":    runtime.Version(),
		"go_os":         runtime.GOOS,
		"go_arch":       runtime.GOARCH,
		"num_cpu":       runtime.NumCPU(),
		"num_goroutine": runtime.NumGoroutine(),
		"timestamp":     time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"type":"debug_info","data":`))
	writeJSON(w, info)
	w.Write([]byte(`}`))
}

func (h *Handler) DebugConfig(w http.ResponseWriter, r *http.Request) {
	cfg := map[string]interface{}{
		"env":       h.cfg.Env,
		"port":      h.cfg.Port,
		"host":      h.cfg.Host,
		"log_level": h.cfg.LogLevel,
		"dev_mode":  h.cfg.IsDevelopment(),
		"server":    h.cfg.Server,
		"quiz":      h.cfg.Quiz,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"type":"debug_config","data":`))
	writeJSON(w, cfg)
	w.Write([]byte(`}`))
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	switch val := v.(type) {
	case string:
		w.Write([]byte(`"` + val + `"`))
	case int:
		w.Write([]byte(string(rune('0' + val%10))))
	case map[string]interface{}:
		w.Write([]byte("{"))
		first := true
		for k, v := range val {
			if !first {
				w.Write([]byte(","))
			}
			w.Write([]byte(`"` + k + `":`))
			writeJSON(w, v)
			first = false
		}
		w.Write([]byte("}"))
	default:
		w.Write([]byte("null"))
	}
}
