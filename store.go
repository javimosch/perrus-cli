package main

import "sync"

const historySize = 20

type Store struct {
	mu      sync.RWMutex
	history map[string][]*Result
	order   []string
}

type EndpointStatus struct {
	Name    string    `json:"name"`
	Group   string    `json:"group"`
	URL     string    `json:"url"`
	Results []*Result `json:"results"`
}

func NewStore(cfg *Config) *Store {
	s := &Store{history: make(map[string][]*Result)}
	for _, ep := range cfg.Endpoints {
		s.history[ep.Name] = nil
		s.order = append(s.order, ep.Name)
	}
	return s
}

func (s *Store) Add(name string, res *Result) {
	s.mu.Lock()
	defer s.mu.Unlock()
	h := append(s.history[name], res)
	if len(h) > historySize {
		h = h[len(h)-historySize:]
	}
	s.history[name] = h
}

func (s *Store) GetStatuses(cfg *Config) []EndpointStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]EndpointStatus, 0, len(cfg.Endpoints))
	for _, ep := range cfg.Endpoints {
		h := s.history[ep.Name]
		cp := make([]*Result, len(h))
		copy(cp, h)
		out = append(out, EndpointStatus{Name: ep.Name, Group: ep.Group, URL: ep.URL, Results: cp})
	}
	return out
}

func (s *Store) GetStatus(name string, cfg *Config) *EndpointStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h, ok := s.history[name]
	if !ok {
		return nil
	}
	cp := make([]*Result, len(h))
	copy(cp, h)
	url := ""
	for _, ep := range cfg.Endpoints {
		if ep.Name == name {
			url = ep.URL
			break
		}
	}
	group := ""
	for _, ep := range cfg.Endpoints {
		if ep.Name == name {
			group = ep.Group
			break
		}
	}
	return &EndpointStatus{Name: name, Group: group, URL: url, Results: cp}
}
