// Package proxy provides a MITM proxy server for intercepting HTTP/HTTPS traffic.
package proxy

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

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
	// Store is the recording store for captured traffic (in-memory, used by admin API)
	Store *recording.Store
	// DiskDir is the directory to write recordings to disk as they are captured.
	// When set, each recording is written as a JSON file organized by host.
	// This is the primary persistence mechanism for CLI proxy usage.
	DiskDir string
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
	diskDir string
	ca      *CAManager
	logger  *log.Logger
	client  *http.Client // Shared HTTP client for connection pooling
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
		mode:    mode,
		filter:  filter,
		store:   store,
		diskDir: opts.DiskDir,
		ca:      opts.CAManager,
		logger:  opts.Logger,
		client: &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // Don't follow redirects
			},
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
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

// persistToDisk writes a recording to disk organized by host.
// This is a best-effort operation — errors are logged but don't affect proxying.
func (p *Proxy) persistToDisk(rec *recording.Recording) {
	if p.diskDir == "" {
		return
	}

	host := rec.Request.Host
	if host == "" {
		host = "_unknown"
	}

	hostDir := filepath.Join(p.diskDir, host)
	if err := os.MkdirAll(hostDir, 0700); err != nil {
		p.log("Error creating host directory %s: %v", hostDir, err)
		return
	}

	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		p.log("Error marshaling recording: %v", err)
		return
	}

	filename := filepath.Join(hostDir, "rec_"+rec.ID+".json")
	if err := os.WriteFile(filename, data, 0600); err != nil {
		p.log("Error writing recording to disk: %v", err)
		return
	}
}

// log logs a message if a logger is configured.
func (p *Proxy) log(format string, args ...interface{}) {
	if p.logger != nil {
		p.logger.Printf(format, args...)
	}
}
