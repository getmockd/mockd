package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// PIDFile contains process information for a running mockd instance.
type PIDFile struct {
	PID        int            `json:"pid"`
	StartTime  time.Time      `json:"startTime"`
	Version    string         `json:"version"`
	Commit     string         `json:"commit,omitempty"`
	Components ComponentsInfo `json:"components"`
	Config     ConfigInfo     `json:"config,omitempty"`
}

// ComponentsInfo contains status for each component.
type ComponentsInfo struct {
	Admin  ComponentStatus `json:"admin"`
	Engine ComponentStatus `json:"engine"`
}

// ComponentStatus contains the status of a single component.
type ComponentStatus struct {
	Enabled   bool   `json:"enabled"`
	Port      int    `json:"port"`
	Host      string `json:"host"`
	HTTPSPort int    `json:"httpsPort,omitempty"` // Engine only
}

// ConfigInfo contains configuration metadata.
type ConfigInfo struct {
	File        string `json:"file,omitempty"`
	MocksLoaded int    `json:"mocksLoaded"`
}

// DefaultPIDPath returns the default PID file location (~/.mockd/mockd.pid).
func DefaultPIDPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory
		return ".mockd/mockd.pid"
	}
	return filepath.Join(home, ".mockd", "mockd.pid")
}

// WritePIDFile writes the PID file to the specified path.
// It creates the parent directory if it doesn't exist.
func WritePIDFile(path string, info *PIDFile) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create PID file directory: %w", err)
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal PID file: %w", err)
	}

	// Write atomically by writing to temp file first
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Rename to final path
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename PID file: %w", err)
	}

	return nil
}

// ReadPIDFile reads and parses the PID file from the specified path.
func ReadPIDFile(path string) (*PIDFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("PID file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to read PID file: %w", err)
	}

	var info PIDFile
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("failed to parse PID file: %w", err)
	}

	return &info, nil
}

// RemovePIDFile removes the PID file at the specified path.
func RemovePIDFile(path string) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove PID file: %w", err)
	}
	return nil
}

// IsRunning checks if the process with the stored PID is still running.
func (p *PIDFile) IsRunning() bool {
	if p.PID <= 0 {
		return false
	}

	// Find the process
	process, err := os.FindProcess(p.PID)
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds. We need to send signal 0 to check
	// if the process actually exists.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// Uptime returns the duration since the process started.
func (p *PIDFile) Uptime() time.Duration {
	if p.StartTime.IsZero() {
		return 0
	}
	return time.Since(p.StartTime)
}

// FormatUptime returns a human-readable uptime string.
func (p *PIDFile) FormatUptime() string {
	d := p.Uptime()
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if hours >= 24 {
		days := hours / 24
		hours = hours % 24
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	return fmt.Sprintf("%dh %dm", hours, mins)
}

// AdminURL returns the full URL for the admin API.
func (p *PIDFile) AdminURL() string {
	if !p.Components.Admin.Enabled {
		return ""
	}
	host := p.Components.Admin.Host
	if host == "" {
		host = "localhost"
	}
	return fmt.Sprintf("http://%s:%d", host, p.Components.Admin.Port)
}

// EngineURL returns the full URL for the mock engine.
func (p *PIDFile) EngineURL() string {
	if !p.Components.Engine.Enabled {
		return ""
	}
	host := p.Components.Engine.Host
	if host == "" {
		host = "localhost"
	}
	return fmt.Sprintf("http://%s:%d", host, p.Components.Engine.Port)
}

// EngineHTTPSURL returns the full HTTPS URL for the mock engine.
func (p *PIDFile) EngineHTTPSURL() string {
	if !p.Components.Engine.Enabled || p.Components.Engine.HTTPSPort == 0 {
		return ""
	}
	host := p.Components.Engine.Host
	if host == "" {
		host = "localhost"
	}
	return fmt.Sprintf("https://%s:%d", host, p.Components.Engine.HTTPSPort)
}
