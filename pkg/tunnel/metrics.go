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
