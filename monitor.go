package main

import (
	"sync"
	"time"
)

type Monitor struct {
	cfg   *Config
	store *Store
	done  chan struct{}
	wg    sync.WaitGroup
}

func NewMonitor(cfg *Config, store *Store) *Monitor {
	return &Monitor{cfg: cfg, store: store, done: make(chan struct{})}
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
}

func (m *Monitor) watch(ep *Endpoint) {
	defer m.wg.Done()
	// First check immediately
	m.store.Add(ep.Name, checkEndpoint(ep))

	ticker := time.NewTicker(ep.Interval.Duration)
	defer ticker.Stop()
	for {
		select {
		case <-m.done:
			return
		case <-ticker.C:
			m.store.Add(ep.Name, checkEndpoint(ep))
		}
	}
}
