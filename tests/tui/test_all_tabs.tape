Output tests/tui/output/all_tabs_test.gif
Set FontSize 14
Set Width 120
Set Height 30

# Start the TUI (can't actually start mockd in VHS, but document the flow)
Type "# Test plan: Click through all 7 tabs and check layouts"
Enter
Sleep 1s

# Document what we need to test manually
Type "# Tabs to test:"
Enter
Type "# 1. Dashboard - initial view"
Enter
Type "# 2. Mocks - list view"
Enter
Type "# 3. Recordings - proxy recordings"
Enter  
Type "# 4. Streams - SSE streams"
Enter
Type "# 5. Traffic - HTTP traffic log"
Enter
Type "# 6. Connections - active connections"
Enter
Type "# 7. Logs - server logs"
Enter
Sleep 2s

# Screenshot the test plan
Screenshot tests/tui/screenshots/test_plan.png
