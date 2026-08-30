package main

// Alert ingestion endpoint — lets external scripts (e.g. offload-watch on dk2)
// push alerts through perrus-cli's Telegram notifier instead of each script
// needing its own Telegram bot config. POST JSON to /api/v1/ingest.
//
// Accepted JSON fields (at least one of text/message/level required):
//   {"source": "dk2/offload-watch", "level": "ALERTE", "text": "...", "detail": {...}}
//
// The handler formats a Telegram message and sends it immediately, bypassing
// the grouped-alert timer (external alerts are urgent and pre-filtered by the
// source script). A per-source cooldown prevents flooding.

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type IngestAlert struct {
	Source string          `json:"source"`
	Level  string          `json:"level"`
	Text   string          `json:"text"`
	Detail json.RawMessage `json:"detail,omitempty"`
}

type AlertIngester struct {
	telegram *TelegramNotifier
	mu       sync.Mutex
	lastSent map[string]time.Time // source -> last alert time
	cooldown time.Duration
}

func NewAlertIngester(tn *TelegramNotifier) *AlertIngester {
	return &AlertIngester{
		telegram: tn,
		lastSent: make(map[string]time.Time),
		cooldown: 10 * time.Minute,
	}
}

func (ai *AlertIngester) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var alert IngestAlert
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		http.Error(w, `{"error":"read body"}`, http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(body, &alert); err != nil {
		http.Error(w, `{"error":"invalid json: `+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	msg := strings.TrimSpace(alert.Text)
	if msg == "" {
		msg = strings.TrimSpace(alert.Level)
	}
	if msg == "" {
		http.Error(w, `{"error":"no text or level in alert"}`, http.StatusBadRequest)
		return
	}

	source := alert.Source
	if source == "" {
		source = "external"
	}

	// Per-source cooldown to prevent flooding
	ai.mu.Lock()
	if last, ok := ai.lastSent[source]; ok && time.Since(last) < ai.cooldown {
		ai.mu.Unlock()
		writeJSON(w, map[string]any{"status": "cooldown", "source": source})
		return
	}
	ai.lastSent[source] = time.Now()
	ai.mu.Unlock()

	// Format and send via Telegram
	level := alert.Level
	if level == "" {
		level = "ALERT"
	}
	text := fmt.Sprintf("📢 *%s* — %s\n\n%s", source, level, msg)

	if ai.telegram == nil {
		log.Printf("[ingest] no telegram configured — dropped alert from %s: %s", source, msg)
		writeJSON(w, map[string]any{"status": "no-telegram", "source": source})
		return
	}

	if err := ai.telegram.sendMessage(text); err != nil {
		log.Printf("[ingest] telegram send failed for %s: %v", source, err)
		http.Error(w, `{"error":"telegram send failed"}`, http.StatusServiceUnavailable)
		return
	}

	log.Printf("[ingest] alert from %s forwarded to telegram: %s", source, msg[:min(len(msg), 80)])
	writeJSON(w, map[string]any{"status": "sent", "source": source})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
