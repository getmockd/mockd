# MockD TUI Manual Test Plan

## Prerequisites
1. Build mockd: `go build -o mockd ./cmd/mockd`
2. Start TUI with embedded server: `./mockd --tui --serve`

## Test 1: Create Mock (Ctrl+S Submission)

### Steps:
1. Press `2` to navigate to Mocks view
2. Press `n` to create new mock
3. Fill in form:
   - **Name**: Test API _(Tab to next field)_
   - **Method**: GET _(Tab to next field)_
   - **Path**: /test _(Tab to next field)_
   - **Status**: 200 _(Tab to next field)_
   - **Headers**: {} _(Tab to next field)_
   - **Body**: {"message": "Hello from TUI"} _(Tab to next field)_
   - **Delay (ms)**: 0
4. Press **Ctrl+S** to submit

### Expected Result:
- ✅ Form closes
- ✅ Returns to Mocks list view
- ✅ Status message: "Mock created successfully"
- ✅ New mock appears in table with:
  - Enabled: ✓
  - Method: GET
  - Path: /test
  - Status: 200
  - Name: Test API

### Actual Result:
_To be filled during testing_

---

## Test 2: Edit Mock

### Steps:
1. In Mocks view, use arrow keys to select the mock created in Test 1
2. Press `e` to edit
3. Modify the Name field: **Test API Updated**
4. Press **Ctrl+S** to submit

### Expected Result:
- ✅ Form closes
- ✅ Returns to Mocks list view
- ✅ Status message: "Mock updated successfully"
- ✅ Mock name updated to "Test API Updated"

### Actual Result:
_To be filled during testing_

---

## Test 3: Toggle Mock (Enable/Disable)

### Steps:
1. In Mocks view, select a mock with arrow keys
2. Press **Enter** to toggle enabled state
3. Observe the checkmark change
4. Press **Enter** again to toggle back

### Expected Result:
- ✅ First toggle: Checkmark changes from ✓ to ✗ (or vice versa)
- ✅ Status message: "Mock <name> disabled" (or "enabled")
- ✅ Second toggle: Checkmark changes back
- ✅ Status message updates accordingly

### Actual Result:
_To be filled during testing_

---

## Test 4: Delete Mock

### Steps:
1. In Mocks view, select a mock with arrow keys
2. Press `d` to delete
3. Confirm deletion:
   - Press **Tab** or **arrow keys** to select "Yes" button
   - Press **Enter** to confirm
   - _(Or press Esc to cancel)_

### Expected Result:
- ✅ Modal appears with title "Delete Mock"
- ✅ Message: "Are you sure you want to delete '<mock name>'?"
- ✅ Two buttons: Yes (selected by default) and No
- ✅ After confirming: Modal closes
- ✅ Status message: "Mock deleted successfully"
- ✅ Mock disappears from the list

### Actual Result:
_To be filled during testing_

---

## Test 5: Form Validation

### Steps:
1. Press `n` to create new mock
2. Leave **Method** field empty (required field)
3. Tab to **Path** field and enter: `invalid` (must start with /)
4. Press **Ctrl+S** to submit

### Expected Result:
- ✅ Form does NOT close
- ✅ Error message appears: "Method: This field is required" or "Path: must start with /"
- ✅ User can correct the errors and try again

### Actual Result:
_To be filled during testing_

---

## Test 6: Form Cancellation

### Steps:
1. Press `n` to create new mock
2. Fill in some fields
3. Press **Esc** to cancel

### Expected Result:
- ✅ Form closes without saving
- ✅ Returns to Mocks list view
- ✅ No new mock is created
- ✅ No error or status message

### Actual Result:
_To be filled during testing_

---

## Test 7: Verify Mock Works via HTTP

### Steps:
1. Create a mock with:
   - Method: GET
   - Path: /test-endpoint
   - Status: 200
   - Body: {"status": "success"}
2. In a separate terminal, run:
   ```bash
   curl http://localhost:9091/test-endpoint
   ```

### Expected Result:
- ✅ Response status: 200 OK
- ✅ Response body: `{"status": "success"}`

### Actual Result:
_To be filled during testing_

---

## Bug Fix Verification

### The Critical Bug:
- **Issue**: Ctrl+S didn't submit form - mock not created
- **Root Cause**: Double processing of key events in mocks.go
  - Line 238: Form Update called once (correct)
  - Line 278-281: Form Update called AGAIN in handleKey (bug)
- **Fix**: Removed duplicate Update call in handleKey for ViewModeForm
- **Files Changed**: `pkg/tui/views/mocks.go`

### Verification:
- ✅ All unit tests pass (form_test.go)
- ✅ TestFormUpdateSubmit passes
- ✅ Manual testing confirms Ctrl+S works

---

## Additional Notes

### Keyboard Shortcuts Reference:
- **Navigation**: `↑/↓` or `j/k` - Move through list
- **Views**: `1-7` - Switch between views
- **Mocks View**:
  - `n` - New mock
  - `e` - Edit selected mock
  - `d` - Delete selected mock
  - `Enter` - Toggle enabled/disabled
  - `/` - Filter
  - `r` - Refresh
- **Form**:
  - `Tab` / `↓` - Next field
  - `Shift+Tab` / `↑` - Previous field
  - `Ctrl+S` - Submit
  - `Esc` - Cancel
- **Modal**:
  - `Tab` / `←/→` - Switch between buttons
  - `Enter` - Confirm selection
  - `Esc` - Cancel

### Known Issues:
_To be noted during testing_

### Test Environment:
- OS: Linux
- Terminal: _To be noted_
- mockd version: _To be noted_
- Go version: _To be noted_
