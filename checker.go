package main

import (
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type Result struct {
	EndpointName string      `json:"endpoint"`
	URL          string      `json:"url"`
	Success      bool        `json:"success"`
	Duration     int64       `json:"duration_ms"`
	StatusCode   int         `json:"status_code,omitempty"`
	Connected    bool        `json:"connected"`
	Body         string      `json:"body,omitempty"`
	Errors       []string    `json:"errors,omitempty"`
	Conditions   []CondCheck `json:"conditions"`
	Timestamp    time.Time   `json:"timestamp"`
}

type CondCheck struct {
	Condition string `json:"condition"`
	Passed    bool   `json:"passed"`
}

func checkEndpoint(ep *Endpoint) *Result {
	res := &Result{
		EndpointName: ep.Name,
		URL:          ep.URL,
		Timestamp:    time.Now(),
	}

	start := time.Now()
	switch {
	case strings.HasPrefix(ep.URL, "tcp://"):
		doTCP(ep, res)
	case strings.HasPrefix(ep.URL, "dns://"):
		doDNS(ep, res)
	default:
		doHTTP(ep, res)
	}
	res.Duration = time.Since(start).Milliseconds()

	allPassed := len(res.Errors) == 0
	for _, cond := range ep.Conditions {
		passed, err := evalCondition(cond, res)
		if err != nil {
			res.Errors = append(res.Errors, err.Error())
			passed = false
		}
		res.Conditions = append(res.Conditions, CondCheck{Condition: cond, Passed: passed})
		if !passed {
			allPassed = false
		}
	}
	res.Success = allPassed

	return res
}

func doHTTP(ep *Endpoint, res *Result) {
	client := &http.Client{
		Timeout: ep.Timeout.Duration,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil // follow redirects
		},
	}
	method := ep.Method
	if method == "" {
		method = "GET"
	}

	var bodyReader io.Reader
	if ep.Body != "" {
		bodyReader = strings.NewReader(ep.Body)
	}

	req, err := http.NewRequest(method, ep.URL, bodyReader)
	if err != nil {
		res.Errors = append(res.Errors, "failed to build request: "+err.Error())
		return
	}
	for k, v := range ep.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		res.Errors = append(res.Errors, "request failed: "+err.Error())
		return
	}
	defer resp.Body.Close()

	res.StatusCode = resp.StatusCode
	res.Connected = true

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err == nil {
		res.Body = string(body)
	}
}

func doTCP(ep *Endpoint, res *Result) {
	addr := strings.TrimPrefix(ep.URL, "tcp://")
	conn, err := net.DialTimeout("tcp", addr, ep.Timeout.Duration)
	if err != nil {
		res.Errors = append(res.Errors, "tcp connect failed: "+err.Error())
		res.Connected = false
		return
	}
	conn.Close()
	res.Connected = true
}

func doDNS(ep *Endpoint, res *Result) {
	host := strings.TrimPrefix(ep.URL, "dns://")
	addrs, err := net.LookupHost(host)
	if err != nil {
		res.Errors = append(res.Errors, "dns lookup failed: "+err.Error())
		res.Connected = false
		return
	}
	res.Connected = true
	res.Body = strings.Join(addrs, ",")
}

func checkAll(cfg *Config) []*Result {
	results := make([]*Result, len(cfg.Endpoints))
	for i := range cfg.Endpoints {
		results[i] = checkEndpoint(&cfg.Endpoints[i])
	}
	return results
}
