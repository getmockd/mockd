package tunnel

import (
	"fmt"
	"time"
)

// FormatStats returns a human-readable string of tunnel statistics.
func FormatStats(stats *TunnelStats) string {
	if stats == nil {
		return "No stats available"
	}

	status := "Disconnected"
	if stats.IsConnected {
		status = "Connected"
	}

	return fmt.Sprintf(`Tunnel Status: %s
Requests Served: %d
Bytes In: %s
Bytes Out: %s
Uptime: %s
Reconnects: %d`,
		status,
		stats.RequestsServed,
		formatBytes(stats.BytesIn),
		formatBytes(stats.BytesOut),
		formatDuration(stats.Uptime()),
		stats.Reconnects,
	)
}

// FormatDetailedStats returns a detailed human-readable string of tunnel statistics.
func FormatDetailedStats(stats *TunnelStats) string {
	if stats == nil {
		return "No stats available"
	}

	status := "Disconnected"
	if stats.IsConnected {
		status = "Connected"
	}

	latencySection := ""
	if stats.RequestsServed > 0 {
		latencySection = fmt.Sprintf(`
Latency:
  Average: %.2f ms
  Minimum: %.2f ms
  Maximum: %.2f ms`,
			stats.AvgLatencyMs(),
			stats.MinLatencyMs(),
			stats.MaxLatencyMs(),
		)
	}

	throughputSection := ""
	if stats.Uptime() > 0 {
		reqsPerSec := float64(stats.RequestsServed) / stats.Uptime().Seconds()
		bytesPerSec := float64(stats.BytesIn+stats.BytesOut) / stats.Uptime().Seconds()
		throughputSection = fmt.Sprintf(`
Throughput:
  Requests/sec: %.2f
  Bytes/sec: %s`,
			reqsPerSec,
			formatBytes(int64(bytesPerSec)),
		)
	}

	return fmt.Sprintf(`Tunnel Status: %s

Connection:
  Uptime: %s
  Reconnects: %d

Traffic:
  Requests Served: %d
  Bytes In: %s
  Bytes Out: %s
  Total: %s%s%s`,
		status,
		formatDuration(stats.Uptime()),
		stats.Reconnects,
		stats.RequestsServed,
		formatBytes(stats.BytesIn),
		formatBytes(stats.BytesOut),
		formatBytes(stats.BytesIn+stats.BytesOut),
		latencySection,
		throughputSection,
	)
}

// formatBytes formats bytes in human-readable format.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// formatDuration formats a duration in human-readable format.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", mins, secs)
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, mins)
}
