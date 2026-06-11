package main

import (
	"sync"
	"time"
)

type Monitor struct {
	cfg       *Config
	store     *Store
	telegram  *TelegramNotifier
	done      chan struct{}
	wg        sync.WaitGroup
}

func NewMonitor(cfg *Config, store *Store) *Monitor {
	return &Monitor{
		cfg:      cfg,
		store:    store,
		telegram: NewTelegramNotifier(cfg.Telegram),
		done:     make(chan struct{}),
	}
}

func (m *Monitor) Start() {
	for i := range m.cfg.Endpoints {
		ep := &m.cfg.Endpoints[i]
		m.wg.Add(1)
		go m.watch(ep)
	}
}

func (m *Monitor) Stop() {
	close(m.done)
	m.wg.Wait()
	m.telegram.Stop()
}

func (m *Monitor) watch(ep *Endpoint) {
	defer m.wg.Done()
	// First check immediately
	res := checkEndpoint(ep)
	m.store.Add(ep.Name, res)
	m.telegram.Notify(res)

	ticker := time.NewTicker(ep.Interval.Duration)
	defer ticker.Stop()
	for {
		select {
		case <-m.done:
			return
		case <-ticker.C:
			res := checkEndpoint(ep)
			m.store.Add(ep.Name, res)
			m.telegram.Notify(res)
		}
	}
}
