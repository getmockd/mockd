# TUI Form Input Bug Fix - Test Plan

## Build
```bash
cd /home/plex/development/repos/getmockd/mockd
go build -o mockd ./cmd/mockd
```

## Test Cases

### Test 1: Form Shows on 'n' Press
1. Run: `./mockd --tui`
2. Press '2' to go to Mocks view
3. Press 'n' to create new mock
4. **Expected**: Form should appear with title "Create Mock"
5. **Expected**: Status bar should show: "tab: next field • ctrl+s: save • esc: cancel"

### Test 2: Form Fields Accept Input
1. Continue from Test 1
2. Type "test" in the Name field
3. **Expected**: Should see "test" appear in the field
4. **Expected**: Cursor should blink in the Name field

### Test 3: 'q' Does Not Quit When in Form
1. Continue from Test 2
2. Press 'q'
3. **Expected**: Should type 'q' in the field (now showing "testq")
4. **Expected**: App should NOT quit

### Test 4: Tab Moves Between Fields
1. Continue from Test 3
2. Clear the field (backspace)
3. Type "My Test Mock"
4. Press Tab
5. **Expected**: Should move to Method field
6. **Expected**: Method field should be focused (cursor blinking)

### Test 5: Esc Cancels Form
1. Continue from Test 4
2. Press Esc
3. **Expected**: Should return to Mocks list view
4. **Expected**: Status bar should show: "enter: toggle • n: new • e: edit • d: delete • /: filter • ?: help • q: quit"

### Test 6: Form Submit
1. Press 'n' again
2. Fill in fields:
   - Name: "Test API"
   - Tab
   - Method: "GET" (default)
   - Tab
   - Path: "/test"
   - Tab to Status, Body, etc.
3. Press Ctrl+S
4. **Expected**: Form should submit
5. **Expected**: Should return to list view
6. **Expected**: Should see new mock in the list (if backend is running)

### Test 7: 'q' Quits When in List View
1. In Mocks list view
2. Press 'q'
3. **Expected**: App should quit

## Success Criteria
- All form fields accept typing
- 'q' only quits in list view, not in form
- Tab navigates between fields
- Status bar shows correct hints for each mode
- Form can be cancelled with Esc
- Form can be submitted with Ctrl+S
