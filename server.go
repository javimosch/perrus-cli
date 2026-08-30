package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

//go:embed dashboard.html
var dashboardHTML []byte

func runServer(port int, cfg *Config) {
	store := NewStore(cfg)
	mon := NewMonitor(cfg, store)
	mon.Start()

	ingester := NewAlertIngester(mon.telegram)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", handleHealth)
	mux.HandleFunc("/api/v1/ingest", ingester.Handle)
	mux.HandleFunc("/api/v1/endpoints/statuses", func(w http.ResponseWriter, r *http.Request) {
		all := store.GetStatuses(cfg)
		// ?group=X filters to one group (case-insensitive) — powers the /g/<group> pages
		if g := r.URL.Query().Get("group"); g != "" {
			filtered := make([]EndpointStatus, 0, len(all))
			for _, ep := range all {
				if strings.EqualFold(ep.Group, g) {
					filtered = append(filtered, ep)
				}
			}
			all = filtered
		}
		writeJSON(w, all)
	})
	// /g/<group> — a public status page scoped to a single group (same dashboard,
	// pre-filtered), so a product can link to a status page for its services only.
	mux.HandleFunc("/g/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(dashboardHTML)
	})
	mux.HandleFunc("/api/v1/endpoints/", func(w http.ResponseWriter, r *http.Request) {
		// /api/v1/endpoints/{name}/statuses
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/endpoints/")
		name := strings.TrimSuffix(path, "/statuses")
		st := store.GetStatus(name, cfg)
		if st == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"endpoint not found"}`))
			return
		}
		writeJSON(w, st)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(dashboardHTML)
	})

	srv := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		mon.Stop()
		srv.Close()
	}()

	log.Printf("perrus-cli v%s — dashboard: http://localhost:%d", Version, port)
	log.Printf("monitoring %d endpoints — Ctrl+C to stop", len(cfg.Endpoints))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "healthy", "version": Version})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}
