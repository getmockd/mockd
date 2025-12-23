#!/bin/bash
# Test script for TUI workflows
# This script starts the mockd server and provides instructions for manual testing

set -e

echo "=== MockD TUI Workflow Test ==="
echo ""
echo "This script will help you test all TUI workflows:"
echo ""
echo "1. CREATE MOCK (Press 'n')"
echo "   - Press '2' to go to Mocks view"
echo "   - Press 'n' for new mock"
echo "   - Fill in the form:"
echo "     * Name: Test API"
echo "     * Method: GET (or press Tab to keep default)"
echo "     * Path: /test"
echo "     * Status: 200 (or press Tab to keep default)"
echo "     * Headers: {} (or press Tab to keep default)"
echo "     * Body: {\"message\": \"Hello World\"}"
echo "     * Delay: 0 (or press Tab to keep default)"
echo "   - Press Ctrl+S to submit"
echo "   - VERIFY: Mock appears in the list"
echo ""
echo "2. EDIT MOCK (Press 'e')"
echo "   - Use arrow keys to select the mock you created"
echo "   - Press 'e' to edit"
echo "   - Change Name to: Test API Updated"
echo "   - Press Ctrl+S to submit"
echo "   - VERIFY: Mock name is updated in the list"
echo ""
echo "3. TOGGLE MOCK (Press Enter)"
echo "   - Select the mock with arrow keys"
echo "   - Press Enter to toggle enabled/disabled"
echo "   - VERIFY: Checkmark (✓) changes to (✗) or vice versa"
echo "   - Press Enter again to toggle back"
echo ""
echo "4. DELETE MOCK (Press 'd')"
echo "   - Select the mock with arrow keys"
echo "   - Press 'd' to delete"
echo "   - Confirm deletion by pressing 'y' or Enter"
echo "   - VERIFY: Mock disappears from the list"
echo ""
echo "Press Enter to start the TUI..."
read

# Start mockd in TUI mode
./mockd --tui
