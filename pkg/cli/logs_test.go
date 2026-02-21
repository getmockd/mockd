package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func RunLogs(args []string) error {
	logsCmd.SetArgs(args)
	return logsCmd.Execute()
}

func TestRunLogs_HelpFlag(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RunLogs panicked with --help: %v", r)
		}
	}()

	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	err := RunLogs([]string{"--help"})

	w.Close()
	os.Stderr = oldStderr

	if err != nil {
		t.Errorf("RunLogs --help returned error: %v", err)
	}
}

func TestDefaultLogsPath(t *testing.T) {
	path := defaultLogsPath()
	if path == "" {
		t.Error("defaultLogsPath returned empty string")
	}

	// Should contain .mockd/logs
	if !containsPath(path, ".mockd") || !containsPath(path, "logs") {
		// Unless home dir failed, then it's /tmp
		if filepath.Dir(filepath.Dir(path)) != "/tmp" {
			t.Errorf("unexpected path: %s", path)
		}
	}
}

func containsPath(fullPath, segment string) bool {
	for {
		base := filepath.Base(fullPath)
		if base == segment {
			return true
		}
		parent := filepath.Dir(fullPath)
		if parent == fullPath {
			break
		}
		fullPath = parent
	}
	return false
}

func TestFindLogFiles_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	files, err := findLogFiles(tmpDir, "")
	if err != nil {
		t.Fatalf("findLogFiles failed: %v", err)
	}

	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
}

func TestFindLogFiles_WithLogFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some log files
	logFiles := []string{"mockd.log", "admin-local.log", "engine-default.log"}
	for _, name := range logFiles {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte("test log content\n"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	// Create a non-log file (should be ignored)
	nonLogFile := filepath.Join(tmpDir, "other.txt")
	if err := os.WriteFile(nonLogFile, []byte("not a log\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	files, err := findLogFiles(tmpDir, "")
	if err != nil {
		t.Fatalf("findLogFiles failed: %v", err)
	}

	if len(files) != 3 {
		t.Errorf("expected 3 files, got %d", len(files))
	}
}

func TestFindLogFiles_WithServiceFilter(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some log files
	logFiles := []string{"mockd.log", "admin-local.log", "engine-default.log"}
	for _, name := range logFiles {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte("test log content\n"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	// Filter by admin/local (should match admin-local.log and mockd.log)
	files, err := findLogFiles(tmpDir, "admin/local")
	if err != nil {
		t.Fatalf("findLogFiles failed: %v", err)
	}

	// Should find admin-local.log and mockd.log (combined log)
	if len(files) != 2 {
		t.Errorf("expected 2 files for 'admin/local', got %d: %v", len(files), files)
	}

	// Filter by engine/default
	files, err = findLogFiles(tmpDir, "engine/default")
	if err != nil {
		t.Fatalf("findLogFiles failed: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("expected 2 files for 'engine/default', got %d: %v", len(files), files)
	}
}

func TestReadLastLines(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Create a log file with 10 lines
	content := ""
	for i := 1; i <= 10; i++ {
		content += "line " + string(rune('0'+i)) + "\n"
	}
	// Fix: use proper string formatting
	content = "line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10\n"

	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Read last 5 lines
	lines, err := readLastLines(logPath, 5)
	if err != nil {
		t.Fatalf("readLastLines failed: %v", err)
	}

	if len(lines) != 5 {
		t.Errorf("expected 5 lines, got %d", len(lines))
	}

	// Verify it's the last 5 lines
	expected := []string{"line 6", "line 7", "line 8", "line 9", "line 10"}
	for i, line := range lines {
		if line != expected[i] {
			t.Errorf("line %d: expected %q, got %q", i, expected[i], line)
		}
	}
}

func TestReadLastLines_FewerLines(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Create a log file with 3 lines
	content := "line 1\nline 2\nline 3\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Read last 10 lines (more than available)
	lines, err := readLastLines(logPath, 10)
	if err != nil {
		t.Fatalf("readLastLines failed: %v", err)
	}

	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
}

func TestParseLogTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantYear int
	}{
		{
			name:     "RFC3339",
			line:     "2024-01-15T10:30:00Z Starting mockd...",
			wantYear: 2024,
		},
		{
			name:     "RFC3339 with brackets",
			line:     "[2024-01-15T10:30:00Z] Starting mockd...",
			wantYear: 2024,
		},
		{
			name:     "Common format",
			line:     "2024-01-15 10:30:00 Starting mockd...",
			wantYear: 2024,
		},
		{
			name:     "No timestamp",
			line:     "Some log message without timestamp",
			wantYear: 1, // time.Time{}.Year() == 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := parseLogTimestamp(tt.line)
			if ts.Year() != tt.wantYear {
				t.Errorf("parseLogTimestamp(%q) year = %d, want %d", tt.line, ts.Year(), tt.wantYear)
			}
		})
	}
}

func TestRunDaemonLogs_NoLogDir(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentDir := filepath.Join(tmpDir, "nonexistent")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runDaemonLogs(runDaemonLogsOptions{
		logDir:      nonExistentDir,
		serviceName: "",
		lines:       100,
		follow:      false,
		jsonOutput:  false,
	})

	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if err != nil {
		t.Errorf("runDaemonLogs returned error: %v", err)
	}

	if !logsContainsString(output, "No logs found") {
		t.Errorf("expected 'No logs found' message, got: %s", output)
	}
}

func TestRunDaemonLogs_EmptyLogDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runDaemonLogs(runDaemonLogsOptions{
		logDir:      tmpDir,
		serviceName: "",
		lines:       100,
		follow:      false,
		jsonOutput:  false,
	})

	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if err != nil {
		t.Errorf("runDaemonLogs returned error: %v", err)
	}

	if !logsContainsString(output, "No log files found") {
		t.Errorf("expected 'No log files found' message, got: %s", output)
	}
}

func TestRunDaemonLogs_WithLogs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a log file with some content
	logPath := filepath.Join(tmpDir, "mockd.log")
	content := "2024-01-15T10:30:00Z Starting mockd...\n2024-01-15T10:30:01Z Server ready\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runDaemonLogs(runDaemonLogsOptions{
		logDir:      tmpDir,
		serviceName: "",
		lines:       100,
		follow:      false,
		jsonOutput:  false,
	})

	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if err != nil {
		t.Errorf("runDaemonLogs returned error: %v", err)
	}

	if !logsContainsString(output, "Starting mockd") {
		t.Errorf("expected log content in output, got: %s", output)
	}
}

func TestRunDaemonLogs_JSONOutput(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a log file with some content
	logPath := filepath.Join(tmpDir, "mockd.log")
	content := "2024-01-15T10:30:00Z Starting mockd...\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runDaemonLogs(runDaemonLogsOptions{
		logDir:      tmpDir,
		serviceName: "",
		lines:       100,
		follow:      false,
		jsonOutput:  true,
	})

	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if err != nil {
		t.Errorf("runDaemonLogs returned error: %v", err)
	}

	// Should contain JSON structure
	if !logsContainsString(output, "timestamp") || !logsContainsString(output, "message") {
		t.Errorf("expected JSON output with timestamp and message, got: %s", output)
	}
}

func TestReadNewLines(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Create initial log file
	content := "line 1\nline 2\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Get initial size
	info, _ := os.Stat(logPath)
	initialOffset := info.Size()

	// Read from start (should get all lines)
	lines, newOffset, err := readNewLines(logPath, 0)
	if err != nil {
		t.Fatalf("readNewLines failed: %v", err)
	}
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
	if newOffset != initialOffset {
		t.Errorf("expected offset %d, got %d", initialOffset, newOffset)
	}

	// Read from end (should get no new lines)
	lines, _, err = readNewLines(logPath, initialOffset)
	if err != nil {
		t.Fatalf("readNewLines failed: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("expected 0 lines, got %d", len(lines))
	}

	// Append new content
	f, _ := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString("line 3\nline 4\n")
	f.Close()

	// Read new lines
	lines, _, err = readNewLines(logPath, initialOffset)
	if err != nil {
		t.Fatalf("readNewLines failed: %v", err)
	}
	if len(lines) != 2 {
		t.Errorf("expected 2 new lines, got %d", len(lines))
	}
}

func TestReadNewLines_FileTruncated(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Create initial log file
	content := "line 1\nline 2\nline 3\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	info, _ := os.Stat(logPath)
	initialOffset := info.Size()

	// Truncate file (simulate log rotation)
	if err := os.WriteFile(logPath, []byte("new line 1\n"), 0644); err != nil {
		t.Fatalf("failed to truncate file: %v", err)
	}

	// Read should detect truncation and read from beginning
	lines, _, err := readNewLines(logPath, initialOffset)
	if err != nil {
		t.Fatalf("readNewLines failed: %v", err)
	}
	if len(lines) != 1 {
		t.Errorf("expected 1 line after truncation, got %d", len(lines))
	}
}

func TestDisplayLogs_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two log files with overlapping timestamps
	log1 := filepath.Join(tmpDir, "admin-local.log")
	log2 := filepath.Join(tmpDir, "engine-default.log")

	content1 := "2024-01-15T10:30:00Z Admin starting\n2024-01-15T10:30:02Z Admin ready\n"
	content2 := "2024-01-15T10:30:01Z Engine starting\n2024-01-15T10:30:03Z Engine ready\n"

	os.WriteFile(log1, []byte(content1), 0644)
	// Wait a bit to ensure different mod times
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(log2, []byte(content2), 0644)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	files := []string{log1, log2}
	err := displayLogs(files, 100, false)

	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	buf := make([]byte, 2048)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if err != nil {
		t.Errorf("displayLogs returned error: %v", err)
	}

	// Should contain all 4 lines
	if !logsContainsString(output, "Admin starting") ||
		!logsContainsString(output, "Engine starting") ||
		!logsContainsString(output, "Admin ready") ||
		!logsContainsString(output, "Engine ready") {
		t.Errorf("expected all log lines, got: %s", output)
	}
}

func logsContainsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
