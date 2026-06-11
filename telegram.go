package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type TelegramNotifier struct {
	cfg            *Telegram
	client         *http.Client
	mu             sync.Mutex
	messageLog     map[string][]time.Time // endpoint -> message timestamps
	pendingAlerts  map[string]*Result     // endpoint -> latest failure
	lastSent       map[string]time.Time   // endpoint -> last sent timestamp
	groupTimer     *time.Timer
	sending        bool                   // flag to prevent concurrent sends
	chatID         string
}

type TelegramMessage struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

func NewTelegramNotifier(cfg *Telegram) *TelegramNotifier {
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	return &TelegramNotifier{
		cfg:           cfg,
		client:        &http.Client{Timeout: 10 * time.Second},
		messageLog:    make(map[string][]time.Time),
		pendingAlerts: make(map[string]*Result),
		lastSent:      make(map[string]time.Time),
		chatID:        cfg.ChatID,
	}
}

func (tn *TelegramNotifier) Notify(result *Result) {
	if tn == nil {
		return
	}

	tn.mu.Lock()
	defer tn.mu.Unlock()

	// Only notify on failures
	if result.Success {
		// Clear pending alert if endpoint recovers
		delete(tn.pendingAlerts, result.EndpointName)
		return
	}

	// Check if endpoint is already in pending alerts (deduplication)
	if _, exists := tn.pendingAlerts[result.EndpointName]; exists {
		return // Already pending, skip
	}

	// Check if we recently sent an alert for this endpoint (cooldown: 10 minutes)
	if lastSent, ok := tn.lastSent[result.EndpointName]; ok {
		if time.Since(lastSent) < 10*time.Minute {
			return // Skip, cooldown period not over
		}
	}

	// Add to pending alerts
	tn.pendingAlerts[result.EndpointName] = result

	// Start group timer only if not already running
	if tn.groupTimer == nil {
		tn.groupTimer = time.AfterFunc(tn.cfg.GroupInterval.Duration, func() {
			tn.sendGroupedAlerts()
		})
	}
}

func (tn *TelegramNotifier) checkRateLimit(endpoint string) bool {
	now := time.Now()
	hourAgo := now.Add(-time.Hour)

	// Clean old messages
	timestamps := tn.messageLog[endpoint]
	var recent []time.Time
	for _, ts := range timestamps {
		if ts.After(hourAgo) {
			recent = append(recent, ts)
		}
	}
	tn.messageLog[endpoint] = recent

	// Check limit
	if len(recent) >= tn.cfg.MaxMessagesPerHour {
		return false
	}

	// Add current message
	tn.messageLog[endpoint] = append(recent, now)
	return true
}

func (tn *TelegramNotifier) sendGroupedAlerts() {
	tn.mu.Lock()
	
	// Prevent concurrent sends
	if tn.sending {
		tn.groupTimer = nil
		tn.mu.Unlock()
		return
	}
	
	if len(tn.pendingAlerts) == 0 {
		tn.groupTimer = nil
		tn.mu.Unlock()
		return
	}

	// Mark as sending
	tn.sending = true
	
	// Copy pending alerts to avoid holding lock during send
	pending := make(map[string]*Result)
	for k, v := range tn.pendingAlerts {
		pending[k] = v
	}
	
	tn.groupTimer = nil
	tn.mu.Unlock()

	// Check rate limit for each endpoint
	for endpoint := range pending {
		if !tn.checkRateLimit(endpoint) {
			log.Printf("Telegram rate limit exceeded for %s, skipping notification", endpoint)
			delete(pending, endpoint)
		}
	}

	// If all endpoints were rate limited, don't send anything
	if len(pending) == 0 {
		tn.mu.Lock()
		tn.sending = false
		tn.mu.Unlock()
		return
	}

	var message string
	if len(pending) == 1 {
		// Single endpoint failure
		for _, res := range pending {
			message = tn.formatSingleAlert(res)
		}
	} else {
		// Multiple endpoint failures
		message = tn.formatGroupedAlert()
	}

	if err := tn.sendMessage(message); err != nil {
		log.Printf("Failed to send Telegram message: %v", err)
	} else {
		log.Printf("Sent Telegram alert for %d failing endpoints", len(pending))
		// Record last sent time for each endpoint
		now := time.Now()
		for endpoint := range pending {
			tn.mu.Lock()
			tn.lastSent[endpoint] = now
			delete(tn.pendingAlerts, endpoint)
			tn.mu.Unlock()
		}
	}

	tn.mu.Lock()
	tn.sending = false
	tn.mu.Unlock()
}

func (tn *TelegramNotifier) formatSingleAlert(res *Result) string {
	var statusInfo string
	if res.StatusCode != 0 {
		statusInfo = fmt.Sprintf("Status: %d\n", res.StatusCode)
	}
	
	var errors string
	if len(res.Errors) > 0 {
		errors = "\nErrors:\n"
		for _, e := range res.Errors {
			errors += fmt.Sprintf("  • %s\n", e)
		}
	}

	return fmt.Sprintf("🚨 *Alert: %s is DOWN*\n\n"+
		"URL: %s\n"+
		"%s"+
		"Response Time: %dms\n"+
		"Time: %s%s",
		res.EndpointName,
		res.URL,
		statusInfo,
		res.Duration,
		res.Timestamp.Format(time.RFC3339),
		errors)
}

func (tn *TelegramNotifier) formatGroupedAlert() string {
	message := fmt.Sprintf("🚨 *Multiple Services DOWN*\n\n")
	
	for _, res := range tn.pendingAlerts {
		var statusInfo string
		if res.StatusCode != 0 {
			statusInfo = fmt.Sprintf(" (Status: %d)", res.StatusCode)
		}
		message += fmt.Sprintf("• %s%s — %dms\n", res.EndpointName, statusInfo, res.Duration)
	}
	
	message += fmt.Sprintf("\nTime: %s", time.Now().Format(time.RFC3339))
	return message
}

func (tn *TelegramNotifier) sendMessage(text string) error {
	msg := TelegramMessage{
		ChatID:    tn.chatID,
		Text:      text,
		ParseMode: "Markdown",
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", tn.cfg.BotToken), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := tn.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	return nil
}

func (tn *TelegramNotifier) Stop() {
	if tn != nil && tn.groupTimer != nil {
		tn.groupTimer.Stop()
	}
}
