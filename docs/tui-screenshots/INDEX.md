# mockd TUI Screenshots Index

All screenshots captured on December 22, 2025 from branch `019-tui-cli`

## Screenshot Files

### Core Views

#### 01-dashboard-initial.txt
- **View:** Dashboard (startup)
- **Description:** Initial state showing server running on :9090, 0 mocks, uptime
- **Size:** 24KB
- **Notable:** Clean status display, quick stats section

#### 02-mocks-empty.txt
- **View:** Mocks List (empty)
- **Description:** Empty mocks table with column headers
- **Size:** 24KB
- **Notable:** "Press / to filter" hint, proper table structure

#### 03-mock-form-initial.txt
- **View:** Mock Creation Form
- **Description:** Huh form with all fields visible
- **Size:** 24KB
- **Notable:** Pre-filled defaults (My API Mock, GET, /api/users, 200)
- **Fields:** Name, Method, Path, Status Code, Headers, Body, Delay

#### 04-mocks-with-one.txt
- **View:** Mocks List (1 mock)
- **Description:** Single mock listed after creation
- **Size:** 24KB
- **Notable:** Success message "Mock created successfully"
- **Mock:** GET /api/users 200 (enabled)

#### 05-mocks-with-two.txt
- **View:** Mocks List (2 mocks)
- **Description:** Shows duplication bug - both mocks have same data
- **Size:** 24KB
- **BUG:** Second mock should be POST /api/posts 201 but shows GET /api/users 200
- **Critical Issue:** Mock creation not saving correct data

#### 06-recordings.txt
- **View:** Recordings
- **Description:** Stuck on loading state
- **Size:** 24KB
- **Issue:** "⣾ Loading proxy status..." with no resolution

#### 07-streams.txt
- **View:** Streams (labeled as view 4)
- **Description:** Shows "Traffic Log LIVE • 0 requests"
- **Size:** 6.4KB
- **Issue:** Appears identical to Traffic view (confusion)

#### 08-traffic.txt
- **View:** Traffic
- **Description:** Traffic log table with headers
- **Size:** 24KB
- **Notable:** LIVE indicator, columns for Time/Method/Path/Status/Duration/Mock

#### 09-connections.txt
- **View:** Connections
- **Description:** Empty connections view with tab navigation
- **Size:** 26KB
- **Notable:** Tabs for [1] All [2] WebSocket [3] SSE

#### 10-logs.txt
- **View:** Application Logs
- **Description:** Log viewer with 1 entry
- **Size:** 26KB
- **Notable:** Shows "14:43:37.724 INFO [engine] Request received: GET /api/users"
- **Features:** Level filters [1] Debug [2] Info [3] Warn [4] Error

#### 11-help.txt
- **View:** Help Overlay
- **Description:** Comprehensive keyboard shortcuts modal
- **Size:** 6.7KB
- **Notable:** Well-organized sections for Navigation, Global Actions, View-Specific

#### 12-mock-disabled.txt
- **View:** Help Overlay (unintended)
- **Description:** Shows bug where Enter key opened help instead of toggling mock
- **Size:** 6.7KB
- **BUG:** Enter key behavior broken in Mocks view

#### 13-mocks-after-restart.txt
- **View:** Mocks List (after restart)
- **Description:** Shows mocks are not persisted
- **Size:** 24KB
- **Finding:** Mocks count is 0 - confirms in-memory only storage

## Quick Access by Category

### Working Features
- `01-dashboard-initial.txt` - Dashboard works perfectly
- `03-mock-form-initial.txt` - Form renders beautifully
- `10-logs.txt` - Logs capture requests correctly
- `11-help.txt` - Help system is comprehensive

### Known Bugs
- `05-mocks-with-two.txt` - Mock duplication bug
- `12-mock-disabled.txt` - Enter key behavior bug
- `06-recordings.txt` - Loading stuck
- `07-streams.txt` vs `08-traffic.txt` - View confusion

### Empty/Initial States
- `02-mocks-empty.txt` - No mocks
- `09-connections.txt` - No connections
- `13-mocks-after-restart.txt` - Mocks cleared

## File Format

All screenshots are plain text ASCII files captured using:
```bash
tmux capture-pane -t 2 -p > filename.txt
```

Terminal size: 317x72 (width x height)

Each file contains 73 lines representing the full terminal output including:
- Unicode box-drawing characters
- ANSI color codes (may not be visible in plain text)
- Status bar at line 71-72
- Blank line at 73

## Usage

To view a screenshot:
```bash
cat docs/tui-screenshots/01-dashboard-initial.txt
```

To compare two screenshots:
```bash
diff docs/tui-screenshots/04-mocks-with-one.txt docs/tui-screenshots/05-mocks-with-two.txt
```

## Related Documents

- `../../TUI_AUDIT_REPORT.md` - Full comprehensive audit report
- `../../AUDIT_SUMMARY.md` - Quick summary of findings
