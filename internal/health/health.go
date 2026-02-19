package health

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"
)

type Checker struct {
	db     *sql.DB
	server *http.Server
}

func NewChecker(db *sql.DB, port string) *Checker {
	return &Checker{
		db: db,
		server: &http.Server{
			Addr: ":" + port,
		},
	}
}

func (c *Checker) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", c.handleHealth)
	mux.HandleFunc("/ready", c.handleReady)
	mux.HandleFunc("/live", c.handleLive)
	c.server.Handler = mux

	go func() {
		slog.Info("Starting health check server", "port", c.server.Addr)
		if err := c.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Health check server error", "error", err)
		}
	}()
}

func (c *Checker) Stop(ctx context.Context) error {
	return c.server.Shutdown(ctx)
}

func (c *Checker) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := c.db.PingContext(ctx); err != nil {
		slog.Warn("Health check failed - database unreachable", "error", err)
		http.Error(w, "database unreachable", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"healthy","database":"connected"}`))
}

func (c *Checker) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := c.db.PingContext(ctx); err != nil {
		slog.Warn("Readiness check failed", "error", err)
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ready"}`))
}

func (c *Checker) handleLive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"alive"}`))
}
