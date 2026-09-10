package main

import "testing"

// The group limiter was hardcoded to 1/hour, so whichever source alerted first
// each hour silenced every other alert until the window rolled. On dk1 that was
// one repeating offload-watch alert dropping 142 notifications, rbm21 being
// unreachable for ten hours among them.
func TestGroupRateLimitHonoursConfig(t *testing.T) {
	tn := &TelegramNotifier{cfg: &Telegram{MaxGroupMessagesPerHour: 3}}
	for i := 1; i <= 3; i++ {
		if !tn.checkGroupRateLimit() {
			t.Fatalf("message %d of 3 should have been allowed", i)
		}
	}
	if tn.checkGroupRateLimit() {
		t.Error("the 4th message in the hour should be dropped")
	}
}

// A config that does not mention the group limit must not fall back to 1.
func TestGroupRateLimitDefaultsToMaxMessagesPerHour(t *testing.T) {
	cfg := &Config{Telegram: &Telegram{
		Enabled: true, BotToken: "t", ChatID: "c", MaxMessagesPerHour: 10,
	}}
	applyTelegramDefaults(cfg)
	if got := cfg.Telegram.MaxGroupMessagesPerHour; got != 10 {
		t.Errorf("MaxGroupMessagesPerHour = %d, want it to inherit 10", got)
	}
}
