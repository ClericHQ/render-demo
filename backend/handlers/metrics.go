package handlers

import (
	"fmt"
	"sync/atomic"
)

// Metrics holds application metrics using atomic counters
type Metrics struct {
	promptsCreated        atomic.Int64
	promptVersionsCreated atomic.Int64
	httpRequests          atomic.Int64
	httpErrors            atomic.Int64
}

// NewMetrics creates a new Metrics instance
func NewMetrics() *Metrics {
	return &Metrics{}
}

// IncrementPromptsCreated increments the prompts created counter
func (m *Metrics) IncrementPromptsCreated() {
	m.promptsCreated.Add(1)
}

// IncrementPromptVersionsCreated increments the prompt versions created counter
func (m *Metrics) IncrementPromptVersionsCreated() {
	m.promptVersionsCreated.Add(1)
}

// IncrementHTTPRequests increments the HTTP requests counter
func (m *Metrics) IncrementHTTPRequests() {
	m.httpRequests.Add(1)
}

// IncrementHTTPErrors increments the HTTP errors counter
func (m *Metrics) IncrementHTTPErrors() {
	m.httpErrors.Add(1)
}

// ExportPrometheus returns metrics in Prometheus text format
func (m *Metrics) ExportPrometheus() string {
	return fmt.Sprintf(`# HELP prompts_created_total Total number of prompts created
# TYPE prompts_created_total counter
prompts_created_total %d

# HELP prompt_versions_created_total Total number of prompt versions created
# TYPE prompt_versions_created_total counter
prompt_versions_created_total %d

# HELP http_requests_total Total number of HTTP requests
# TYPE http_requests_total counter
http_requests_total %d

# HELP http_errors_total Total number of HTTP errors
# TYPE http_errors_total counter
http_errors_total %d
`,
		m.promptsCreated.Load(),
		m.promptVersionsCreated.Load(),
		m.httpRequests.Load(),
		m.httpErrors.Load(),
	)
}
