Output tests/tui/screenshots/all_tabs_test.gif

Set FontSize 12
Set Width 100
Set Height 30
Set Padding 10

Type "./mockd --tui --serve"
Enter
Sleep 2s

# Tab 1: Dashboard
Screenshot tests/tui/screenshots/tab1_dashboard.png
Sleep 1s

# Tab 2: Mocks
Type "2"
Sleep 1s
Screenshot tests/tui/screenshots/tab2_mocks.png
Sleep 1s

# Tab 3: Recordings/Proxy
Type "3"
Sleep 1s
Screenshot tests/tui/screenshots/tab3_recordings.png
Sleep 1s

# Tab 4: Streams
Type "4"
Sleep 1s
Screenshot tests/tui/screenshots/tab4_streams.png
Sleep 1s

# Tab 5: Traffic (THE PROBLEMATIC ONE)
Type "5"
Sleep 1s
Screenshot tests/tui/screenshots/tab5_traffic.png
Sleep 1s

# Tab 6: Connections
Type "6"
Sleep 1s
Screenshot tests/tui/screenshots/tab6_connections.png
Sleep 1s

# Tab 7: Logs
Type "7"
Sleep 1s
Screenshot tests/tui/screenshots/tab7_logs.png
Sleep 1s

# Back to Dashboard
Type "1"
Sleep 1s

# Quit
Type "q"
Sleep 500ms
