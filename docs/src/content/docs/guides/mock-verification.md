---
title: Mock Verification
description: Verify that your mocks were called correctly during tests with call counts, invocation history, and expectations.
---

Mock verification lets you assert that your mocks were called the expected number of times and with the expected parameters. This is essential for integration testing where you need to verify that your code is making the correct API calls.

## Overview

mockd tracks every request that matches a mock:

- **Call count** - How many times the mock was called
- **Invocation history** - Details of each call (method, path, headers, body)
- **Timestamps** - When the mock was first and last called

## Quick Start

```bash
# 1. Start mockd with a mock
mockd serve --config mocks.json

# 2. Run your tests (which call the mock)
npm test

# 3. Verify the mock was called
curl http://localhost:4290/mocks/my-mock-id/verify

# 4. Reset for the next test
curl -X DELETE http://localhost:4290/verify
```

## Verification Endpoints

### Check Call Count

```bash
GET /mocks/{id}/verify
```

**Response:**

```json
{
  "mockId": "get-users",
  "callCount": 5,
  "lastCalledAt": "2024-01-15T10:35:00Z"
}
```

### Assert Call Count

```bash
POST /mocks/{id}/verify
Content-Type: application/json

{
  "atLeast": 1,
  "atMost": 10
}
```

| Field | Description |
|-------|-------------|
| `atLeast` | Minimum expected calls |
| `atMost` | Maximum expected calls |
| `exactly` | Exact expected calls |

**Success Response (200):**

```json
{
  "passed": true,
  "actual": 5,
  "expected": "at least 1 time(s)",
  "message": "Mock was called 5 time(s), matching expectations"
}
```

**Failure Response (409):**

```json
{
  "passed": false,
  "actual": 0,
  "expected": "at least 1 time(s)",
  "message": "Mock was called 0 time(s), not matching expectations"
}
```

### Get Invocation History

```bash
GET /mocks/{id}/invocations
```

**Response:**

```json
{
  "invocations": [
    {
      "id": "req-1",
      "timestamp": "2024-01-15T10:30:00Z",
      "method": "GET",
      "path": "/api/users",
      "headers": {
        "Authorization": "Bearer token123",
        "User-Agent": "my-app/1.0"
      },
      "body": ""
    },
    {
      "id": "req-2",
      "timestamp": "2024-01-15T10:31:00Z",
      "method": "POST",
      "path": "/api/users",
      "headers": {
        "Authorization": "Bearer token123",
        "Content-Type": "application/json"
      },
      "body": "{\"name\": \"Alice\"}"
    }
  ],
  "count": 2,
  "total": 2
}
```

### Reset Verification Data

```bash
# Reset specific mock
DELETE /mocks/{id}/invocations

# Reset all mocks
DELETE /verify
```

## Testing Patterns

### Before Each Test

Reset verification state before each test to ensure isolation:

```javascript
beforeEach(async () => {
  await fetch('http://localhost:4290/verify', { method: 'DELETE' });
});
```

### Verify After Test

```javascript
test('fetches users on load', async () => {
  // Run your code
  await loadUsers();
  
  // Verify the mock was called
  const res = await fetch('http://localhost:4290/mocks/get-users/verify');
  const data = await res.json();
  
  expect(data.callCount).toBe(1);
});
```

### Verify Call Count Assertion

```javascript
test('retries on failure', async () => {
  // Run code that should retry 3 times
  await fetchWithRetry('/api/users');
  
  // Assert exactly 3 calls
  const res = await fetch('http://localhost:4290/mocks/get-users/verify', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ exactly: 3 })
  });
  
  const data = await res.json();
  expect(data.passed).toBe(true);
});
```

### Verify Request Details

```javascript
test('sends correct authorization header', async () => {
  await fetchUsers('my-token');
  
  const res = await fetch('http://localhost:4290/mocks/get-users/invocations');
  const data = await res.json();
  
  expect(data.invocations[0].headers['Authorization']).toBe('Bearer my-token');
});
```

### Verify Request Body

```javascript
test('sends correct payload', async () => {
  await createUser({ name: 'Alice', email: 'alice@example.com' });
  
  const res = await fetch('http://localhost:4290/mocks/create-user/invocations');
  const data = await res.json();
  
  const body = JSON.parse(data.invocations[0].body);
  expect(body.name).toBe('Alice');
  expect(body.email).toBe('alice@example.com');
});
```

## Framework Examples

### Jest (JavaScript)

```javascript
const ADMIN_URL = 'http://localhost:4290';

describe('User API', () => {
  beforeEach(async () => {
    await fetch(`${ADMIN_URL}/verify`, { method: 'DELETE' });
  });

  test('fetches users with pagination', async () => {
    const users = await fetchUsers({ page: 2, limit: 20 });
    
    // Verify mock was called
    const verify = await fetch(`${ADMIN_URL}/mocks/get-users/verify`);
    const { callCount } = await verify.json();
    expect(callCount).toBe(1);
    
    // Verify query parameters
    const invocations = await fetch(`${ADMIN_URL}/mocks/get-users/invocations`);
    const { invocations: calls } = await invocations.json();
    expect(calls[0].query.page).toBe('2');
    expect(calls[0].query.limit).toBe('20');
  });
});
```

### pytest (Python)

```python
import requests
import pytest

ADMIN_URL = 'http://localhost:4290'

@pytest.fixture(autouse=True)
def reset_verification():
    requests.delete(f'{ADMIN_URL}/verify')
    yield

def test_fetches_users_with_auth():
    # Run code under test
    fetch_users(token='secret123')
    
    # Verify mock was called
    res = requests.get(f'{ADMIN_URL}/mocks/get-users/verify')
    assert res.json()['callCount'] == 1
    
    # Verify authorization header
    res = requests.get(f'{ADMIN_URL}/mocks/get-users/invocations')
    invocations = res.json()['invocations']
    assert invocations[0]['headers']['Authorization'] == 'Bearer secret123'

def test_creates_user_with_correct_payload():
    # Run code under test
    create_user(name='Bob', email='bob@example.com')
    
    # Verify request body
    res = requests.get(f'{ADMIN_URL}/mocks/create-user/invocations')
    body = json.loads(res.json()['invocations'][0]['body'])
    assert body['name'] == 'Bob'
    assert body['email'] == 'bob@example.com'
```

### Go

```go
func TestFetchUsers(t *testing.T) {
    adminURL := "http://localhost:4290"
    
    // Reset before test
    req, _ := http.NewRequest("DELETE", adminURL+"/verify", nil)
    http.DefaultClient.Do(req)
    
    // Run code under test
    FetchUsers()
    
    // Verify
    resp, _ := http.Get(adminURL + "/mocks/get-users/verify")
    var result struct {
        CallCount int `json:"callCount"`
    }
    json.NewDecoder(resp.Body).Decode(&result)
    
    if result.CallCount != 1 {
        t.Errorf("expected 1 call, got %d", result.CallCount)
    }
}
```

## Best Practices

### 1. Always Reset Before Tests

```javascript
beforeEach(() => fetch('http://localhost:4290/verify', { method: 'DELETE' }));
```

### 2. Use Descriptive Mock IDs

```yaml
mocks:
  - id: get-users-paginated  # Not just "mock-1"
    name: Get Users with Pagination
```

### 3. Verify Both Count and Content

```javascript
// Verify it was called
expect(callCount).toBe(1);

// Verify it was called correctly
expect(invocations[0].headers['Authorization']).toBeDefined();
```

### 4. Use `exactly` for Strict Tests

```javascript
// Strict: fail if called more or less
{ exactly: 1 }

// Flexible: just ensure it was called
{ atLeast: 1 }
```

### 5. Consider Test Parallelization

If running tests in parallel, use unique mock IDs or separate mockd instances to avoid verification conflicts.

## Comparison with Other Tools

| Feature | mockd | WireMock | Mockoon |
|---------|-------|----------|---------|
| Call count tracking | ✅ Free | ✅ Free | ❌ |
| Invocation history | ✅ Free | ✅ Free | ❌ |
| Assertion API | ✅ Free | ✅ Free | ❌ |
| Request body capture | ✅ Free | ✅ Paid | ❌ |

## See Also

- [Admin API Reference](/reference/admin-api#mock-verification) - Full API details
- [Integration Testing](/examples/integration-testing) - Testing patterns
- [Stateful Mocking](/guides/stateful-mocking) - State management
