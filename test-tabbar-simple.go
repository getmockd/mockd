package main

import (
	"fmt"
	"github.com/getmockd/mockd/pkg/tui/components"
)

func main() {
	// Test tab bar rendering standalone
	tabBar := components.NewTabBar()
	tabBar.SetWidth(120)
	tabBar.SetActive(1) // Mocks

	output := tabBar.View()
	fmt.Println("Tab bar output:")
	fmt.Println(output)
	fmt.Printf("\nLength: %d characters\n", len(output))
	fmt.Printf("Lines: %d\n", len(output) > 0)
}
