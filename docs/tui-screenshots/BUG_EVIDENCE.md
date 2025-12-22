# Critical Bug Evidence - Mock Duplication

## Bug Description
When creating a second mock with different parameters, the TUI displays the same values as the first mock instead of the new values.

## Steps to Reproduce

1. Start TUI: `./mockd --tui --serve`
2. Navigate to Mocks view (key: 2)
3. Create first mock (key: n):
   - Name: "My API Mock"
   - Method: GET
   - Path: /api/users
   - Status: 200
4. Create second mock (key: n):
   - Name: "API Posts"
   - Method: POST
   - Path: /api/posts
   - Status: 201
5. View mocks list

## Expected Result

```
Mocks (2)

 Enabled   Method    Path                            Status    Name
─────────────────────────────────────────────────────────────────────────────────────
 ✓         GET       /api/users                      200       My API Mock
 ✓         POST      /api/posts                      201       API Posts
```

## Actual Result (from 05-mocks-with-two.txt)

```
Mocks (2)

 Enabled   Method    Path                            Status    Name
─────────────────────────────────────────────────────────────────────────────────────
 ✓         GET       /api/users                      200
 ✓         GET       /api/users                      200
```

## Evidence Files

- `05-mocks-with-two.txt` - Lines 6-9 show both mocks with identical data
- `03-mock-form-initial.txt` - Shows form was pre-filled with first mock's defaults

## Analysis

Possible causes:
1. Form state not being cleared between creations
2. API endpoint receiving wrong data from form submission
3. Form values being cached incorrectly
4. Default values overriding user input

## Related Code Locations

Based on diagnostics:
- `/home/plex/development/repos/getmockd/mockd/pkg/tui/components/form.go`
  - Line 80: if statement modernization hint
  - Line 230: Mouse event deprecation warnings
  
- `/home/plex/development/repos/getmockd/mockd/pkg/tui/messages.go`
  - Line 88: `mockCreatedMsg` type (unused warning - might be the issue!)
  - Line 93: `mockUpdatedMsg` type (unused warning)

## Severity

**CRITICAL** - This makes it impossible to create multiple different mocks via the TUI, which is a core use case.

## Priority

**P0** - Must be fixed before TUI can be considered functional for production use.
