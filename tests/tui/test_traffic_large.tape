Output tests/tui/screenshots/traffic_120x40.png

Set FontSize 12
Set Width 120
Set Height 40
Set Padding 10

Type "./mockd --tui --serve"
Enter
Sleep 2s

# Click on Traffic tab (tab 5)
Type "5"
Sleep 1s

Screenshot tests/tui/screenshots/traffic_120x40.png
Sleep 500ms

# Quit
Type "q"
Sleep 500ms
