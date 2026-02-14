// Package recording provides utilities for loading recordings from disk.
package recording

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/getmockd/mockd/pkg/store"
)

// DiskSessionMeta is the metadata from a session's meta.json file.
type DiskSessionMeta struct {
	Name           string   `json:"name"`
	StartTime      string   `json:"startTime"`
	EndTime        string   `json:"endTime,omitempty"`
	Port           int      `json:"port"`
	Mode           string   `json:"mode"`
	RecordingCount int      `json:"recordingCount"`
	Hosts          []string `json:"hosts,omitempty"`
	Filters        *struct {
		IncludePaths []string `json:"includePaths,omitempty"`
		ExcludePaths []string `json:"excludePaths,omitempty"`
		IncludeHosts []string `json:"includeHosts,omitempty"`
		ExcludeHosts []string `json:"excludeHosts,omitempty"`
	} `json:"filters,omitempty"`
}

// DiskSession represents a session directory on disk.
type DiskSession struct {
	DirName string          `json:"dirName"`
	Path    string          `json:"path"`
	Meta    DiskSessionMeta `json:"meta"`
}

// DefaultRecordingsBaseDir returns the default base directory for recordings.
func DefaultRecordingsBaseDir() string {
	return store.DefaultRecordingsDir()
}

// LoadFromDir loads all recordings from a directory, walking subdirectories recursively.
// The directory can be a session directory (containing host subdirectories)
// or a host directory (containing rec_*.json files).
func LoadFromDir(dir string) ([]*Recording, error) {
	var recordings []*Recording

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil //nolint:nilerr // intentionally skip inaccessible entries
		}
		if !strings.HasPrefix(d.Name(), "rec_") || !strings.HasSuffix(d.Name(), ".json") {
			return nil // skip non-recording files
		}

		rec, loadErr := LoadFromFile(path)
		if loadErr != nil {
			return nil //nolint:nilerr // intentionally skip corrupted files
		}
		recordings = append(recordings, rec...)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", dir, err)
	}

	// Sort by timestamp (oldest first)
	sort.Slice(recordings, func(i, j int) bool {
		return recordings[i].Timestamp.Before(recordings[j].Timestamp)
	})

	return recordings, nil
}

// LoadFromFile loads recordings from a single JSON file.
// Supports both a single Recording object and an array of Recordings.
func LoadFromFile(path string) ([]*Recording, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	// Try as single recording first
	var rec Recording
	if err := json.Unmarshal(data, &rec); err == nil && rec.ID != "" {
		return []*Recording{&rec}, nil
	}

	// Try as array
	var recordings []*Recording
	if err := json.Unmarshal(data, &recordings); err == nil {
		return recordings, nil
	}

	return nil, fmt.Errorf("file %s does not contain valid recording(s)", path)
}

// ResolveSessionDir resolves a session name or "latest" to an absolute directory path.
// It searches the given base directory for matching session directories.
func ResolveSessionDir(baseDir, sessionName string) (string, error) {
	if baseDir == "" {
		baseDir = DefaultRecordingsBaseDir()
	}

	// "latest" resolves via the symlink or latest file
	if sessionName == "latest" || sessionName == "" {
		return resolveLatest(baseDir)
	}

	// Check if sessionName is already an absolute/relative path to a directory
	if info, err := os.Stat(sessionName); err == nil && info.IsDir() {
		return sessionName, nil
	}

	// Exact match first
	candidate := filepath.Join(baseDir, sessionName)
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate, nil
	}

	// Prefix match (session name without timestamp)
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to read recordings directory %s: %w", baseDir, err)
	}

	var matches []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), sessionName+"-") || e.Name() == sessionName {
			matches = append(matches, e.Name())
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no session found matching %q in %s", sessionName, baseDir)
	}

	// Return the most recent match (sorted lexicographically, timestamps sort correctly)
	sort.Strings(matches)
	return filepath.Join(baseDir, matches[len(matches)-1]), nil
}

// resolveLatest resolves the "latest" session from a base directory.
func resolveLatest(baseDir string) (string, error) {
	latestLink := filepath.Join(baseDir, "latest")

	// Try symlink first
	target, err := os.Readlink(latestLink)
	if err == nil {
		// Resolve relative to baseDir
		if !filepath.IsAbs(target) {
			target = filepath.Join(baseDir, target)
		}
		if info, err := os.Stat(target); err == nil && info.IsDir() {
			return target, nil
		}
	}

	// Try reading as a plain file containing the session dir name
	data, err := os.ReadFile(latestLink)
	if err == nil {
		name := strings.TrimSpace(string(data))
		candidate := filepath.Join(baseDir, name)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
	}

	// Fall back to most recent directory by modification time
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return "", fmt.Errorf("no recordings found in %s", baseDir)
	}

	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}

	if len(dirs) == 0 {
		return "", fmt.Errorf("no recording sessions found in %s", baseDir)
	}

	// Sort lexicographically (timestamps in dir names sort correctly)
	sort.Strings(dirs)
	return filepath.Join(baseDir, dirs[len(dirs)-1]), nil
}

// ListSessions lists all recording sessions in the base directory.
func ListSessions(baseDir string) ([]DiskSession, error) {
	if baseDir == "" {
		baseDir = DefaultRecordingsBaseDir()
	}

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read recordings directory: %w", err)
	}

	var sessions []DiskSession
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		sessionPath := filepath.Join(baseDir, e.Name())
		meta := loadSessionMeta(sessionPath)
		if meta == nil {
			// No meta.json — try to infer from directory contents
			meta = &DiskSessionMeta{Name: e.Name()}
		}

		sessions = append(sessions, DiskSession{
			DirName: e.Name(),
			Path:    sessionPath,
			Meta:    *meta,
		})
	}

	// Sort by start time (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339, sessions[i].Meta.StartTime)
		tj, _ := time.Parse(time.RFC3339, sessions[j].Meta.StartTime)
		return ti.After(tj)
	})

	return sessions, nil
}

// loadSessionMeta loads meta.json from a session directory.
func loadSessionMeta(sessionDir string) *DiskSessionMeta {
	data, err := os.ReadFile(filepath.Join(sessionDir, "meta.json"))
	if err != nil {
		return nil
	}

	var meta DiskSessionMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil
	}
	return &meta
}

// CountRecordingsInDir counts recording files in a directory (recursive).
func CountRecordingsInDir(dir string) int {
	count := 0
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil //nolint:nilerr // intentionally skip inaccessible entries
		}
		if strings.HasPrefix(d.Name(), "rec_") && strings.HasSuffix(d.Name(), ".json") {
			count++
		}
		return nil
	})
	return count
}

// ClearSession removes all recording files from a session directory.
// Keeps the meta.json and directory structure.
func ClearSession(sessionDir string) (int, error) {
	count := 0
	err := filepath.WalkDir(sessionDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil //nolint:nilerr // intentionally skip inaccessible entries
		}
		if strings.HasPrefix(d.Name(), "rec_") && strings.HasSuffix(d.Name(), ".json") {
			if removeErr := os.Remove(path); removeErr == nil {
				count++
			}
		}
		return nil
	})
	return count, err
}

// DeleteSession removes an entire session directory.
func DeleteSession(sessionDir string) error {
	return os.RemoveAll(sessionDir)
}
