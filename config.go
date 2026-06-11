package main

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Endpoints []Endpoint  `yaml:"endpoints"`
	Telegram  *Telegram   `yaml:"telegram,omitempty"`
}

type Telegram struct {
	Enabled            bool      `yaml:"enabled"`
	BotToken           string    `yaml:"bot_token"`
	ChatID             string    `yaml:"chat_id"`
	MaxMessagesPerHour int       `yaml:"max_messages_per_hour"`
	GroupInterval      duration  `yaml:"group_interval"`
}

type Endpoint struct {
	Group      string            `yaml:"group"`
	Name       string            `yaml:"name"`
	URL        string            `yaml:"url"`
	Method     string            `yaml:"method"`
	Headers    map[string]string `yaml:"headers"`
	Body       string            `yaml:"body"`
	Interval   duration          `yaml:"interval"`
	Timeout    duration          `yaml:"timeout"`
	Conditions []string          `yaml:"conditions"`
}

type duration struct {
	time.Duration
}

func (d *duration) UnmarshalYAML(value *yaml.Node) error {
	dur, err := time.ParseDuration(value.Value)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", value.Value, err)
	}
	d.Duration = dur
	return nil
}

func (d duration) String() string {
	return d.Duration.String()
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}
	for i := range cfg.Endpoints {
		ep := &cfg.Endpoints[i]
		if ep.Name == "" {
			return nil, fmt.Errorf("endpoint at index %d has no name", i)
		}
		if ep.URL == "" {
			return nil, fmt.Errorf("endpoint %q has no url", ep.Name)
		}
		if ep.Method == "" {
			ep.Method = "GET"
		}
		if ep.Interval.Duration == 0 {
			ep.Interval.Duration = 60 * time.Second
		}
		if ep.Timeout.Duration == 0 {
			ep.Timeout.Duration = 10 * time.Second
		}
	}
	if cfg.Telegram != nil && cfg.Telegram.Enabled {
		if cfg.Telegram.BotToken == "" {
			return nil, fmt.Errorf("telegram enabled but bot_token is empty")
		}
		if cfg.Telegram.ChatID == "" {
			return nil, fmt.Errorf("telegram enabled but chat_id is empty")
		}
		if cfg.Telegram.MaxMessagesPerHour <= 0 {
			cfg.Telegram.MaxMessagesPerHour = 5
		}
		if cfg.Telegram.GroupInterval.Duration == 0 {
			cfg.Telegram.GroupInterval.Duration = 30 * time.Second
		}
	}
	return &cfg, nil
}

func exampleConfig() string {
	return `# perrus-cli configuration
# URL schemes: https://, http://, tcp://, dns://
# Use 'group' to categorize endpoints in the dashboard

# Optional Telegram integration for alerts
telegram:
  enabled: false
  bot_token: "YOUR_BOT_TOKEN"
  chat_id: "YOUR_CHAT_ID"
  max_messages_per_hour: 5  # per endpoint
  group_interval: 30s        # batch failures within this window

endpoints:
  - name: "httpbin-ok"
    group: "External"
    url: "https://httpbin.org/status/200"
    interval: "30s"
    conditions:
      - "[STATUS] == 200"
      - "[RESPONSE_TIME] < 2000"

  - name: "google"
    group: "External"
    url: "https://www.google.com"
    interval: "60s"
    conditions:
      - "[STATUS] == 200"

  - name: "ssh-localhost"
    group: "Infrastructure"
    url: "tcp://localhost:22"
    interval: "30s"
    conditions:
      - "[CONNECTED] == true"
`
}
