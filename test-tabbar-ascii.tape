# VHS Test: Verify Tab Bar with ASCII output
Output test-tabbar-debug.ascii

Set FontSize 14
Set Width 1200
Set Height 800
Set Shell bash

Type "cd /home/plex/development/repos/getmockd/mockd"
Enter
Sleep 500ms

Type "./mockd --tui --serve"
Enter
Sleep 3s

# Capture initial state
Sleep 2s
