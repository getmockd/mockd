# VHS Test: Verify Tab Bar Navigation
# This test verifies that the tab bar is rendering correctly

Output test-tabbar.gif

Set FontSize 14
Set Width 1200
Set Height 800
Set Shell bash

Type "cd /home/plex/development/repos/getmockd/mockd"
Enter
Sleep 500ms

Type "./mockd --tui"
Enter
Sleep 2s

# Take screenshot of initial view (should show tab bar)
Screenshot test-tabbar-initial.png
Sleep 1s

# Press 2 to switch to Mocks
Type "2"
Sleep 1s
Screenshot test-tabbar-mocks.png

# Press 3 to switch to Recordings
Type "3"
Sleep 1s
Screenshot test-tabbar-recordings.png

# Press 1 to go back to Dashboard
Type "1"
Sleep 1s
Screenshot test-tabbar-dashboard.png

# Press ? for help
Type "?"
Sleep 1s
Screenshot test-tabbar-help.png

# Try to exit help with ESC (testing the bug you mentioned)
Escape
Sleep 500ms
Screenshot test-tabbar-after-help-esc.png

# Try ? again to close help
Type "?"
Sleep 500ms

# Quit
Type "q"
Sleep 500ms
