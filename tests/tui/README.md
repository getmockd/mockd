# TUI Component Tests

Isolated tests for individual TUI components. Each test runs standalone to verify component behavior without dependencies.

## Available Tests

### 1. TabBar Test (`test_tabbar`)
Tests the tab bar component in isolation.

```bash
cd tests/tui
./test_tabbar
```

**What it tests:**
- Tab rendering at different widths
- Mouse click detection and bounds tracking
- Keyboard navigation (1-7)
- Resize behavior
- Underline movement

**Controls:**
- `1-7`: Switch tabs via keyboard
- `Click`: Click on tabs with mouse
- `c`: Clear debug log
- `q`: Quit

**What to verify:**
- [ ] Mouse clicks on tabs work (debug shows "✓ Clicked tab X")
- [ ] Tab bounds are correctly tracked
- [ ] Underline moves to clicked tab
- [ ] All tabs visible at 120 width
- [ ] Tabs handle resize gracefully
- [ ] No visual artifacts

### 2. Header Test (`test_header`)
Tests the header component in isolation.

```bash
cd tests/tui
./test_header
```

**What it tests:**
- Full-width rendering
- Responsive behavior at different widths
- Time updates (every second)
- Recording indicator toggle
- Port display

**Controls:**
- `r`: Toggle recording indicator
- `p`: Change port to 4290
- `q`: Quit

**What to verify:**
- [ ] Header is exactly terminal width (shows ✓)
- [ ] Time updates every second
- [ ] Recording indicator (● REC) appears/disappears
- [ ] At < 40 width: Shows "Too small"
- [ ] At 40-59 width: Logo + time only
- [ ] At 60+ width: Logo + port + time
- [ ] No wrapping or overflow

## Running All Tests

```bash
# Quick test all components
cd tests/tui
for test in test_*; do 
    [ -x "$test" ] && echo "Testing $test..." && ./$test &
    sleep 2
    pkill -f "$test"
done
```

## Creating New Component Tests

Template for a new component test:

```go
package main

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/getmockd/mockd/pkg/tui/components"
)

type model struct {
    component components.YourComponent
    width     int
    height    int
    debug     []string
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // Handle keyboard, mouse, resize
    return m, nil
}

func (m model) View() string {
    // Render component + debug info
    return ""
}

func main() {
    p := tea.NewProgram(
        initialModel(),
        tea.WithAltScreen(),
        tea.WithMouseCellMotion(), // If testing mouse
    )
    p.Run()
}
```

## VHS Automated Tests

Located in this directory with `.tape` extension.

To run:
```bash
vhs demo_tabs_compare.tape
```

## Test Checklist

Before committing component changes:

- [ ] Run `test_header` - verify full width at 60, 80, 100, 120 cols
- [ ] Run `test_tabbar` - verify mouse clicks work
- [ ] Run `test_tabbar` - verify tabs visible at 80+ cols
- [ ] Resize terminal during tests - no crashes
- [ ] Check for visual artifacts (duplicate lines, wrong colors)

## Debugging Tips

### Mouse not working?
1. Check `$TERM` supports mouse (xterm-256color, screen-256color, etc.)
2. Verify `tea.WithMouseCellMotion()` is set
3. Run test with debug output to see coordinates
4. Try different terminal emulators

### Components not sized correctly?
1. Check `Height()` method returns correct value
2. Verify `SetWidth()` is called
3. Check layout calculation in main TUI
4. Run isolated test to see actual vs expected

### Visual artifacts?
1. Check for hardcoded widths
2. Verify padding/margin calculations
3. Test at exact breakpoint sizes (79, 80, 81 columns)
4. Check lipgloss Width() vs MaxWidth()
