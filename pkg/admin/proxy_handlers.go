package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/getmockd/mockd/pkg/logging"
	"github.com/getmockd/mockd/pkg/proxy"
	"github.com/getmockd/mockd/pkg/recording"
)

// ProxyManager manages the proxy server lifecycle for the Admin API.
type ProxyManager struct {
	mu        sync.RWMutex
	log       *slog.Logger
	proxy     *proxy.Proxy
	store     *recording.Store
	ca        *proxy.CAManager
	server    *http.Server
	listener  net.Listener
	running   bool
	port      int
	startTime time.Time
	sessionID string
}

// NewProxyManager creates a new ProxyManager.
func NewProxyManager() *ProxyManager {
	return &ProxyManager{
		log: logging.Nop(),
	}
}

// SetLogger sets the logger under the manager's own lock.
func (pm *ProxyManager) SetLogger(log *slog.Logger) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if log != nil {
		pm.log = log
	} else {
		pm.log = logging.Nop()
	}
}

// ProxyStartRequest represents a request to start the proxy.
type ProxyStartRequest struct {
	Port        int                 `json:"port"`
	Mode        string              `json:"mode"`
	SessionName string              `json:"sessionName"`
	CAPath      string              `json:"caPath"`
	Filters     *FilterConfigUpdate `json:"filters,omitempty"`
}

// FilterConfigUpdate represents filter configuration for updates.
type FilterConfigUpdate struct {
	IncludePaths []string `json:"includePaths,omitempty"`
	ExcludePaths []string `json:"excludePaths,omitempty"`
	IncludeHosts []string `json:"includeHosts,omitempty"`
	ExcludeHosts []string `json:"excludeHosts,omitempty"`
}

// ProxyStatusResponse represents proxy status.
type ProxyStatusResponse struct {
	Running        bool   `json:"running"`
	Port           int    `json:"port,omitempty"`
	Mode           string `json:"mode,omitempty"`
	SessionID      string `json:"sessionId,omitempty"`
	RecordingCount int    `json:"recordingCount,omitempty"`
	Uptime         int    `json:"uptime,omitempty"`
}

// ModeRequest represents a mode change request.
type ModeRequest struct {
	Mode string `json:"mode"`
}

// CAInfoResponse represents CA certificate info.
type CAInfoResponse struct {
	Exists       bool   `json:"exists"`
	Path         string `json:"path,omitempty"`
	Fingerprint  string `json:"fingerprint,omitempty"`
	ExpiresAt    string `json:"expiresAt,omitempty"`
	Organization string `json:"organization,omitempty"`
}

// handleProxyStart handles POST /proxy/start.
func (pm *ProxyManager) handleProxyStart(w http.ResponseWriter, r *http.Request) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.running {
		writeError(w, http.StatusConflict, "already_running", "Proxy is already running")
		return
	}

	var req ProxyStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONDecodeError(w, err, pm.log)
		return
	}

	// Defaults
	if req.Port == 0 {
		req.Port = 8888
	}
	if req.Mode == "" {
		req.Mode = "record"
	}

	// Parse mode
	var mode proxy.Mode
	switch req.Mode {
	case "record":
		mode = proxy.ModeRecord
	case "passthrough":
		mode = proxy.ModePassthrough
	default:
		writeError(w, http.StatusBadRequest, "invalid_mode", "Mode must be 'record' or 'passthrough'")
		return
	}

	// Create store and session
	store := recording.NewStore()
	sessionName := req.SessionName
	if sessionName == "" {
		sessionName = "default"
	}
	session := store.CreateSession(sessionName, nil)
	pm.sessionID = session.ID

	// Create CA manager if path provided
	var ca *proxy.CAManager
	if req.CAPath != "" {
		// Validate CA path to prevent path traversal
		if strings.Contains(req.CAPath, "..") {
			writeError(w, http.StatusBadRequest, "invalid_path", "CA path cannot contain path traversal sequences")
			return
		}
		if filepath.IsAbs(req.CAPath) {
			writeError(w, http.StatusBadRequest, "invalid_path", "CA path must be a relative path")
			return
		}
		cleanPath := filepath.Clean(req.CAPath)
		if strings.HasPrefix(cleanPath, "..") {
			writeError(w, http.StatusBadRequest, "invalid_path", "CA path cannot escape the working directory")
			return
		}

		ca = proxy.NewCAManager(cleanPath+"/ca.crt", cleanPath+"/ca.key")
		if err := ca.EnsureCA(); err != nil {
			pm.log.Error("failed to initialize CA", "error", err)
			writeError(w, http.StatusInternalServerError, "ca_error", "Failed to initialize CA certificate")
			return
		}
	}

	// Create filter config
	filter := proxy.NewFilterConfig()
	if req.Filters != nil {
		if len(req.Filters.IncludePaths) > 0 {
			filter.IncludePaths = req.Filters.IncludePaths
		}
		if len(req.Filters.ExcludePaths) > 0 {
			filter.ExcludePaths = req.Filters.ExcludePaths
		}
		if len(req.Filters.IncludeHosts) > 0 {
			filter.IncludeHosts = req.Filters.IncludeHosts
		}
		if len(req.Filters.ExcludeHosts) > 0 {
			filter.ExcludeHosts = req.Filters.ExcludeHosts
		}
	}

	// Create proxy with logger
	logger := slog.NewLogLogger(pm.log.Handler(), slog.LevelInfo)
	p := proxy.New(proxy.Options{
		Mode:      mode,
		Store:     store,
		Filter:    filter,
		CAManager: ca,
		Logger:    logger,
	})

	// Start HTTP server
	addr := fmt.Sprintf(":%d", req.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "port_error", fmt.Sprintf("Failed to listen on port %d", req.Port))
		return
	}

	server := &http.Server{
		Handler:      p,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second, // Longer write timeout for proxied responses
		IdleTimeout:  120 * time.Second,
	}

	// Store state
	pm.proxy = p
	pm.store = store
	pm.ca = ca
	pm.server = server
	pm.listener = listener
	pm.port = req.Port
	pm.startTime = time.Now()
	pm.running = true

	// Start server in goroutine
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			pm.log.Error("proxy server error", "error", err)
		}
	}()

	writeJSON(w, http.StatusOK, pm.getStatus())
}

// handleProxyStop handles POST /proxy/stop.
func (pm *ProxyManager) handleProxyStop(w http.ResponseWriter, r *http.Request) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if !pm.running {
		writeError(w, http.StatusNotFound, "not_running", "Proxy is not running")
		return
	}

	if pm.server != nil {
		if err := pm.server.Close(); err != nil {
			pm.log.Error("failed to stop proxy", "error", err)
			writeError(w, http.StatusInternalServerError, "stop_error", "Failed to stop proxy server")
			return
		}
	}

	pm.running = false
	writeJSON(w, http.StatusOK, pm.getStatus())
}

// handleProxyStatus handles GET /proxy/status.
func (pm *ProxyManager) handleProxyStatus(w http.ResponseWriter, r *http.Request) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	writeJSON(w, http.StatusOK, pm.getStatus())
}

// handleProxyMode handles PUT /proxy/mode.
func (pm *ProxyManager) handleProxyMode(w http.ResponseWriter, r *http.Request) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if !pm.running {
		writeError(w, http.StatusNotFound, "not_running", "Proxy is not running")
		return
	}

	var req ModeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONDecodeError(w, err, pm.log)
		return
	}

	switch req.Mode {
	case "record":
		pm.proxy.SetMode(proxy.ModeRecord)
	case "passthrough":
		pm.proxy.SetMode(proxy.ModePassthrough)
	default:
		writeError(w, http.StatusBadRequest, "invalid_mode", "Mode must be 'record' or 'passthrough'")
		return
	}

	writeJSON(w, http.StatusOK, pm.getStatus())
}

// handleGetFilters handles GET /proxy/filters.
func (pm *ProxyManager) handleGetFilters(w http.ResponseWriter, r *http.Request) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if !pm.running || pm.proxy == nil {
		writeJSON(w, http.StatusOK, FilterConfigUpdate{})
		return
	}

	filter := pm.proxy.Filter()
	writeJSON(w, http.StatusOK, FilterConfigUpdate{
		IncludePaths: filter.IncludePaths,
		ExcludePaths: filter.ExcludePaths,
		IncludeHosts: filter.IncludeHosts,
		ExcludeHosts: filter.ExcludeHosts,
	})
}

// handleSetFilters handles PUT /proxy/filters.
func (pm *ProxyManager) handleSetFilters(w http.ResponseWriter, r *http.Request) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if !pm.running {
		writeError(w, http.StatusNotFound, "not_running", "Proxy is not running")
		return
	}

	var req FilterConfigUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONDecodeError(w, err, pm.log)
		return
	}

	filter := proxy.NewFilterConfig()
	filter.IncludePaths = req.IncludePaths
	filter.ExcludePaths = req.ExcludePaths
	filter.IncludeHosts = req.IncludeHosts
	filter.ExcludeHosts = req.ExcludeHosts

	pm.proxy.SetFilter(filter)

	writeJSON(w, http.StatusOK, req)
}

// handleGetCA handles GET /proxy/ca.
func (pm *ProxyManager) handleGetCA(w http.ResponseWriter, r *http.Request) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.ca == nil || !pm.ca.Exists() {
		writeJSON(w, http.StatusOK, CAInfoResponse{Exists: false})
		return
	}

	info := CAInfoResponse{
		Exists: true,
		Path:   pm.ca.CertPath(),
	}

	// Get certificate info
	certInfo, err := pm.ca.CertInfo()
	if err == nil {
		info.Fingerprint = certInfo.Fingerprint
		info.ExpiresAt = certInfo.NotAfter.Format(time.RFC3339)
		info.Organization = certInfo.Organization
	}

	writeJSON(w, http.StatusOK, info)
}

// handleGenerateCA handles POST /proxy/ca.
func (pm *ProxyManager) handleGenerateCA(w http.ResponseWriter, r *http.Request) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Parse optional ca-path from request body
	var req struct {
		CAPath string `json:"caPath"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	if req.CAPath == "" {
		writeError(w, http.StatusBadRequest, "missing_path", "caPath is required")
		return
	}

	// Validate CA path to prevent path traversal
	if strings.Contains(req.CAPath, "..") {
		writeError(w, http.StatusBadRequest, "invalid_path", "CA path cannot contain path traversal sequences")
		return
	}
	if filepath.IsAbs(req.CAPath) {
		writeError(w, http.StatusBadRequest, "invalid_path", "CA path must be a relative path")
		return
	}
	cleanPath := filepath.Clean(req.CAPath)
	if strings.HasPrefix(cleanPath, "..") {
		writeError(w, http.StatusBadRequest, "invalid_path", "CA path cannot escape the working directory")
		return
	}

	ca := proxy.NewCAManager(cleanPath+"/ca.crt", cleanPath+"/ca.key")
	if ca.Exists() {
		writeError(w, http.StatusConflict, "ca_exists", "CA certificate already exists")
		return
	}

	if err := ca.Generate(); err != nil {
		pm.log.Error("failed to generate CA", "error", err)
		writeError(w, http.StatusInternalServerError, "generate_error", "Failed to generate CA certificate")
		return
	}

	pm.ca = ca

	info := CAInfoResponse{
		Exists: true,
		Path:   ca.CertPath(),
	}

	certInfo, err := ca.CertInfo()
	if err == nil {
		info.Fingerprint = certInfo.Fingerprint
		info.ExpiresAt = certInfo.NotAfter.Format(time.RFC3339)
		info.Organization = certInfo.Organization
	}

	writeJSON(w, http.StatusOK, info)
}

// handleDownloadCA handles GET /proxy/ca/download.
func (pm *ProxyManager) handleDownloadCA(w http.ResponseWriter, r *http.Request) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if pm.ca == nil || !pm.ca.Exists() {
		writeError(w, http.StatusNotFound, "no_ca", "CA certificate not found")
		return
	}

	certPEM, err := pm.ca.CACertPEM()
	if err != nil {
		pm.log.Error("failed to read CA certificate", "error", err)
		writeError(w, http.StatusInternalServerError, "read_error", "Failed to read CA certificate")
		return
	}

	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", "attachment; filename=mockd-ca.crt")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(certPEM)
}

// getStatus returns the current proxy status (must be called with lock held).
func (pm *ProxyManager) getStatus() ProxyStatusResponse {
	status := ProxyStatusResponse{
		Running: pm.running,
	}

	if pm.running {
		status.Port = pm.port
		status.Mode = string(pm.proxy.Mode())
		status.SessionID = pm.sessionID
		status.Uptime = int(time.Since(pm.startTime).Seconds())

		if pm.store != nil {
			_, total := pm.store.ListRecordings(recording.RecordingFilter{})
			status.RecordingCount = total
		}
	}

	return status
}
