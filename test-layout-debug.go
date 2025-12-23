package main

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/getmockd/mockd/pkg/tui/components"
)

func main() {
	// Test all components individually
	fmt.Println("=== Testing Header ===")
	header := components.NewHeader()
	header.SetWidth(120)
	headerView := header.View()
	fmt.Printf("Header output (%d chars, %d lines):\n", len(headerView), lipgloss.Height(headerView))
	fmt.Println(headerView)
	fmt.Println()

	fmt.Println("=== Testing Tab Bar ===")
	tabBar := components.NewTabBar()
	tabBar.SetWidth(120)
	tabBar.SetActive(1)
	tabBarView := tabBar.View()
	fmt.Printf("Tab bar output (%d chars, %d lines):\n", len(tabBarView), lipgloss.Height(tabBarView))
	fmt.Println(tabBarView)
	fmt.Println()

	fmt.Println("=== Testing Status Bar ===")
	statusBar := components.NewStatusBar()
	statusBar.SetWidth(120)
	statusBarView := statusBar.View()
	fmt.Printf("Status bar output (%d chars, %d lines):\n", len(statusBarView), lipgloss.Height(statusBarView))
	fmt.Println(statusBarView)
	fmt.Println()

	fmt.Println("=== Testing Joined Layout ===")
	joined := lipgloss.JoinVertical(
		lipgloss.Left,
		headerView,
		tabBarView,
		"[CONTENT AREA]",
		statusBarView,
	)
	fmt.Printf("Joined output (%d chars, %d lines):\n", len(joined), lipgloss.Height(joined))
	fmt.Println(joined)
}
