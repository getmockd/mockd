// Package proxy provides a MITM proxy server for intercepting HTTP/HTTPS traffic.
package proxy

import (
	"log"
	"net/http"
	"sync"

	"github.com/getmockd/mockd/pkg/recording"
)

// Mode represents the proxy operating mode.
type Mode string

const (
	// ModeRecord captures traffic for mock generation.
	ModeRecord Mode = "record"
	// ModePassthrough forwards traffic without storing.
	ModePassthrough Mode = "passthrough"
)

// Options configures proxy behavior.
type Options struct {
	// Mode is the initial operating mode
	Mode Mode
	// Filter is the traffic filter configuration
	Filter *FilterConfig
	// Store is the recording store for captured traffic
	Store *recording.Store
	// CAManager handles certificate generation for HTTPS
	CAManager *CAManager
	// Logger for traffic logging (nil = no logging)
	Logger *log.Logger
}

// Proxy is an HTTP/HTTPS MITM proxy server.
type Proxy struct {
	mu      sync.RWMutex
	mode    Mode
	filter  *FilterConfig
	store   *recording.Store
	ca      *CAManager
	logger  *log.Logger
	running bool
}

// New creates a new Proxy with the given options.
func New(opts Options) *Proxy {
	mode := opts.Mode
	if mode == "" {
		mode = ModeRecord
	}

	filter := opts.Filter
	if filter == nil {
		filter = NewFilterConfig()
	}

	store := opts.Store
	if store == nil {
		store = recording.NewStore()
	}

	return &Proxy{
		mode:   mode,
		filter: filter,
		store:  store,
		ca:     opts.CAManager,
		logger: opts.Logger,
	}
}

// Mode returns the current proxy mode.
func (p *Proxy) Mode() Mode {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mode
}

// SetMode changes the proxy operating mode at runtime.
func (p *Proxy) SetMode(mode Mode) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mode = mode
	if p.logger != nil {
		p.logger.Printf("Proxy mode changed to: %s", mode)
	}
}

// Filter returns the current filter configuration.
func (p *Proxy) Filter() *FilterConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.filter
}

// SetFilter updates the filter configuration.
func (p *Proxy) SetFilter(filter *FilterConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.filter = filter
}

// Store returns the recording store.
func (p *Proxy) Store() *recording.Store {
	return p.store
}

// CAManager returns the CA manager.
func (p *Proxy) CAManager() *CAManager {
	return p.ca
}

// ServeHTTP implements http.Handler for the proxy.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
	} else {
		p.handleHTTP(w, r)
	}
}

// handleConnect is implemented in https.go

// log logs a message if a logger is configured.
func (p *Proxy) log(format string, args ...interface{}) {
	if p.logger != nil {
		p.logger.Printf(format, args...)
	}
}
