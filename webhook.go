package main

// WebhookNotifier POSTs a JSON alert to configured URLs when an endpoint goes
// DOWN — built so an automation (e.g. a roam-hub remediation agent) can react
// to the outage, not just a human reading Telegram. Fires on the down
// TRANSITION with a per-endpoint cooldown; recovery clears the state so the
// next outage fires again. No grouping: remediation wants one event per
// endpoint, immediately.

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

type Webhooks struct {
	Enabled  bool     `yaml:"enabled"`
	URLs     []string `yaml:"urls"`
	Cooldown duration `yaml:"cooldown"` // default 10m
}

type WebhookNotifier struct {
	cfg      *Webhooks
	client   *http.Client
	mu       sync.Mutex
	down     map[string]bool      // endpoint -> currently in down state
	lastSent map[string]time.Time // endpoint -> last webhook fired
	groups   map[string]string    // endpoint name -> group
}

type WebhookAlert struct {
	Source     string    `json:"source"`
	Event      string    `json:"event"`
	Endpoint   string    `json:"endpoint"`
	Group      string    `json:"group,omitempty"`
	URL        string    `json:"url"`
	StatusCode int       `json:"status_code,omitempty"`
	DurationMS int64     `json:"duration_ms"`
	Errors     []string  `json:"errors,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

func NewWebhookNotifier(cfg *Config) *WebhookNotifier {
	if cfg.Webhooks == nil || !cfg.Webhooks.Enabled || len(cfg.Webhooks.URLs) == 0 {
		return nil
	}
	groups := make(map[string]string)
	for i := range cfg.Endpoints {
		groups[cfg.Endpoints[i].Name] = cfg.Endpoints[i].Group
	}
	return &WebhookNotifier{
		cfg:      cfg.Webhooks,
		client:   &http.Client{Timeout: 10 * time.Second},
		down:     make(map[string]bool),
		lastSent: make(map[string]time.Time),
		groups:   groups,
	}
}

func (wn *WebhookNotifier) Notify(res *Result) {
	if wn == nil {
		return
	}
	wn.mu.Lock()
	if res.Success {
		delete(wn.down, res.EndpointName)
		wn.mu.Unlock()
		return
	}
	if wn.down[res.EndpointName] {
		wn.mu.Unlock()
		return // still down, already fired for this outage
	}
	cooldown := wn.cfg.Cooldown.Duration
	if cooldown == 0 {
		cooldown = 10 * time.Minute
	}
	if last, ok := wn.lastSent[res.EndpointName]; ok && time.Since(last) < cooldown {
		wn.mu.Unlock()
		return
	}
	wn.down[res.EndpointName] = true
	wn.lastSent[res.EndpointName] = time.Now()
	group := wn.groups[res.EndpointName]
	wn.mu.Unlock()

	alert := WebhookAlert{
		Source:     "perrus",
		Event:      "down",
		Endpoint:   res.EndpointName,
		Group:      group,
		URL:        res.URL,
		StatusCode: res.StatusCode,
		DurationMS: res.Duration,
		Errors:     res.Errors,
		Timestamp:  res.Timestamp,
	}
	body, err := json.Marshal(alert)
	if err != nil {
		log.Printf("webhook: marshal alert: %v", err)
		return
	}
	for _, url := range wn.cfg.URLs {
		go func(u string) {
			req, err := http.NewRequest("POST", u, bytes.NewReader(body))
			if err != nil {
				log.Printf("webhook: build request for %s: %v", u, err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := wn.client.Do(req)
			if err != nil {
				log.Printf("webhook: POST %s: %v", u, err)
				return
			}
			resp.Body.Close()
			log.Printf("webhook: alert %s (down) -> %s [%d]", alert.Endpoint, u, resp.StatusCode)
		}(url)
	}
}
