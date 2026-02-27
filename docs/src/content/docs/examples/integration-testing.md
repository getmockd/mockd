---
title: Integration Testing Example
description: Learn how to use mockd in integration tests for various languages and frameworks.
---

This guide shows how to use mockd in integration tests for various languages and frameworks.

## Overview

mockd is ideal for integration testing because:

- **Isolation**: Tests don't depend on external services
- **Speed**: No network latency to real APIs
- **Predictability**: Responses are always consistent
- **Control**: Easy to simulate errors and edge cases

## Test Setup Pattern

### 1. Start mockd Before Tests

```bash
# Start in background
mockd start --config test-mocks.json &
MOCKD_PID=$!

# Run tests
npm test

# Cleanup
kill $MOCKD_PID
```

### 2. Reset State Between Tests

```bash
# Reset a specific resource to its seed data
curl -X POST http://localhost:4290/state/resources/users/reset

# Or clear a resource (remove all items, no seed data restored)
curl -X DELETE http://localhost:4290/state/resources/users
```

### 3. Point Application to mockd

```bash
API_BASE_URL=http://localhost:4280 npm test
```

## JavaScript / Node.js

### Jest Setup

`jest.setup.js`:

```javascript
const { spawn } = require('child_process');

let mockdProcess;

beforeAll(async () => {
  // Start mockd
  mockdProcess = spawn('mockd', ['start', '--config', 'test-mocks.json'], {
    stdio: 'pipe'
  });

  // Wait for server to be ready
  await waitForServer('http://localhost:4280/health');
});

afterAll(() => {
  if (mockdProcess) {
    mockdProcess.kill();
  }
});

beforeEach(async () => {
  // Reset stateful resources to seed data
  await fetch('http://localhost:4290/state/resources/users/reset', { method: 'POST' });
});

async function waitForServer(url, timeout = 5000) {
  const start = Date.now();
  while (Date.now() - start < timeout) {
    try {
      await fetch(url);
      return;
    } catch {
      await new Promise(r => setTimeout(r, 100));
    }
  }
  throw new Error('Server did not start');
}
```

### Example Tests

```javascript
const API = process.env.API_BASE_URL || 'http://localhost:4280';

describe('User Service', () => {
  test('fetches user by ID', async () => {
    const response = await fetch(`${API}/api/users/1`);
    const user = await response.json();

    expect(response.status).toBe(200);
    expect(user).toEqual({
      id: 1,
      name: 'Alice',
      email: 'alice@example.com'
    });
  });

  test('handles user not found', async () => {
    const response = await fetch(`${API}/api/users/999`);

    expect(response.status).toBe(404);
  });

  test('creates new user', async () => {
    const response = await fetch(`${API}/api/users`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: 'Charlie',
        email: 'charlie@example.com'
      })
    });

    expect(response.status).toBe(201);

    const user = await response.json();
    expect(user.id).toBeDefined();
    expect(user.name).toBe('Charlie');
  });
});
```

### Testing Error Scenarios

```javascript
describe('Error Handling', () => {
  test('handles server errors gracefully', async () => {
    // Add temporary mock for error
    await fetch('http://localhost:4290/mocks', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        type: 'http',
        http: {
          matcher: { method: 'GET', path: '/api/flaky' },
          response: { statusCode: 500, body: '{"error": "Internal error"}' }
        }
      })
    });

    const response = await fetch(`${API}/api/flaky`);

    expect(response.status).toBe(500);
    // Test your app's error handling
  });

  test('handles timeout', async () => {
    await fetch('http://localhost:4290/mocks', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        type: 'http',
        http: {
          matcher: { method: 'GET', path: '/api/slow' },
          response: { statusCode: 200, delayMs: 10000, body: '{}' }
        }
      })
    });

    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 1000);

    await expect(
      fetch(`${API}/api/slow`, { signal: controller.signal })
    ).rejects.toThrow();

    clearTimeout(timeout);
  });
});
```

## Python

### pytest Setup

`conftest.py`:

```python
import subprocess
import time
import requests
import pytest

@pytest.fixture(scope="session")
def mockd_server():
    """Start mockd server for the test session."""
    proc = subprocess.Popen(
        ["mockd", "start", "--config", "test-mocks.json"],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE
    )

    # Wait for server
    for _ in range(50):
        try:
            requests.get("http://localhost:4280/health")
            break
        except requests.ConnectionError:
            time.sleep(0.1)
    else:
        raise RuntimeError("mockd failed to start")

    yield "http://localhost:4280"

    proc.terminate()
    proc.wait()

@pytest.fixture(autouse=True)
def reset_state():
    """Reset mockd stateful resources before each test."""
    requests.post("http://localhost:4290/state/resources/users/reset")
    requests.post("http://localhost:4290/state/resources/tasks/reset")
```

### Example Tests

```python
import requests
import pytest

def test_get_users(mockd_server):
    response = requests.get(f"{mockd_server}/api/users")

    assert response.status_code == 200
    users = response.json()
    assert len(users) >= 1

def test_create_user(mockd_server):
    response = requests.post(
        f"{mockd_server}/api/users",
        json={"name": "Test User", "email": "test@example.com"}
    )

    assert response.status_code == 201
    user = response.json()
    assert "id" in user
    assert user["name"] == "Test User"

def test_user_not_found(mockd_server):
    response = requests.get(f"{mockd_server}/api/users/99999")

    assert response.status_code == 404

class TestStatefulOperations:
    def test_crud_workflow(self, mockd_server):
        # Create
        create_resp = requests.post(
            f"{mockd_server}/api/tasks",
            json={"title": "Test task", "status": "todo"}
        )
        assert create_resp.status_code == 201
        task_id = create_resp.json()["id"]

        # Read
        get_resp = requests.get(f"{mockd_server}/api/tasks/{task_id}")
        assert get_resp.json()["title"] == "Test task"

        # Update
        requests.patch(
            f"{mockd_server}/api/tasks/{task_id}",
            json={"status": "done"}
        )
        get_resp = requests.get(f"{mockd_server}/api/tasks/{task_id}")
        assert get_resp.json()["status"] == "done"

        # Delete
        delete_resp = requests.delete(f"{mockd_server}/api/tasks/{task_id}")
        assert delete_resp.status_code == 204
```

## Go

### Testing Setup

```go
package integration_test

import (
    "encoding/json"
    "net/http"
    "os"
    "os/exec"
    "strings"
    "testing"
    "time"
)

var baseURL = "http://localhost:4280"
var adminURL = "http://localhost:4290"

func TestMain(m *testing.M) {
    // Start mockd
    cmd := exec.Command("mockd", "start", "--config", "test-mocks.json")
    if err := cmd.Start(); err != nil {
        panic(err)
    }

    // Wait for ready
    waitForServer(baseURL + "/health")

    // Run tests
    code := m.Run()

    // Cleanup
    cmd.Process.Kill()
    os.Exit(code)
}

func waitForServer(url string) {
    for i := 0; i < 50; i++ {
        if _, err := http.Get(url); err == nil {
            return
        }
        time.Sleep(100 * time.Millisecond)
    }
    panic("server did not start")
}

func resetState(t *testing.T) {
    t.Helper()
    req, _ := http.NewRequest("POST", adminURL+"/state/resources/users/reset", nil)
    http.DefaultClient.Do(req)
    req, _ = http.NewRequest("POST", adminURL+"/state/resources/tasks/reset", nil)
    http.DefaultClient.Do(req)
}
```

### Example Tests

```go
func TestGetUsers(t *testing.T) {
    resetState(t)

    resp, err := http.Get(baseURL + "/api/users")
    if err != nil {
        t.Fatal(err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        t.Errorf("expected 200, got %d", resp.StatusCode)
    }

    var users []map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&users)

    if len(users) == 0 {
        t.Error("expected users")
    }
}

func TestCreateTask(t *testing.T) {
    resetState(t)

    body := strings.NewReader(`{"title": "Test", "status": "todo"}`)
    resp, err := http.Post(baseURL+"/api/tasks", "application/json", body)
    if err != nil {
        t.Fatal(err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 201 {
        t.Errorf("expected 201, got %d", resp.StatusCode)
    }

    var task map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&task)

    if _, ok := task["id"]; !ok {
        t.Error("expected id field")
    }
}
```

## Docker Compose

For CI/CD environments:

```yaml
version: '3.8'

services:
  mockd:
    image: ghcr.io/getmockd/mockd:latest
    ports:
      - "4280:4280"
      - "4290:4290"
    volumes:
      - ./test-mocks.json:/mocks/config.json
    command: start --config /mocks/config.json

  app-tests:
    build: .
    depends_on:
      - mockd
    environment:
      - API_BASE_URL=http://mockd:4280
    command: npm test
```

## GitHub Actions

`.github/workflows/test.yml`:

```yaml
name: Integration Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v4

      - name: Install mockd
        run: |
          curl -sSL https://github.com/getmockd/mockd/releases/latest/download/mockd-linux-amd64 -o mockd
          chmod +x mockd
          sudo mv mockd /usr/local/bin/

      - name: Start mockd
        run: |
          mockd start --config test-mocks.json &
          sleep 2

      - name: Run tests
        run: npm test
        env:
          API_BASE_URL: http://localhost:4280
```

## Tips

### 1. Seed Data for Tests

```json
{
  "statefulResources": [
    {
      "name": "users",
      "basePath": "/api/users",
      "idField": "id",
      "seedData": [
        {"id": "1", "name": "Test User", "email": "test@example.com"}
      ]
    }
  ]
}
```

### 2. Test Different Scenarios

Create multiple config files:

- `mocks-success.json` - Happy path
- `mocks-errors.json` - Error scenarios
- `mocks-slow.json` - Timeout testing

### 3. Parallel Test Safety

Reset state in each test to ensure isolation:

```javascript
beforeEach(async () => {
  await fetch('http://localhost:4290/state/resources/users/reset', { method: 'POST' });
});
```

### 4. Dynamic Mocks for Edge Cases

Add mocks at runtime for specific test scenarios:

```javascript
test('handles rate limiting', async () => {
  await fetch('http://localhost:4290/mocks', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      type: 'http',
      http: {
        matcher: { method: 'GET', path: '/api/limited' },
        response: {
          statusCode: 429,
          headers: { 'Retry-After': '60' },
          body: '{"error": "Rate limited"}'
        }
      }
    })
  });

  // Test your rate limit handling
});
```

## Mock Verification

After running your tests, verify that your code made the expected API calls. mockd tracks every request matched to a mock, so you can assert call counts and inspect invocation details.

### Assert Call Counts

```javascript
test('payment endpoint is called exactly once', async () => {
  // Create mock
  const res = await fetch('http://localhost:4290/mocks', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      type: 'http',
      http: {
        matcher: { method: 'POST', path: '/api/payments' },
        response: { statusCode: 201, body: '{"id": "pay_123"}' }
      }
    })
  });
  const { id: mockId } = await res.json();

  // Run your application code...
  await myApp.processOrder({ amount: 49.99 });

  // Verify: payment endpoint called exactly once
  const verify = await fetch(`http://localhost:4290/mocks/${mockId}/verify`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ exactly: 1 })
  });
  const result = await verify.json();
  expect(result.passed).toBe(true);
});
```

### Inspect Invocations

```bash
# View every request that hit a specific mock
curl http://localhost:4290/mocks/http_a1b2c3d4/invocations
```

Returns timestamps, request headers, bodies, and matched response details for each call.

### Reset Between Tests

```javascript
beforeEach(async () => {
  // Reset all verification data (call counts + invocation history)
  await fetch('http://localhost:4290/verify', { method: 'DELETE' });
});
```

### Verification API Reference

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/mocks/{id}/verify` | GET | Get call count and last-called timestamp |
| `/mocks/{id}/verify` | POST | Assert call count (`exactly`, `atLeast`, `atMost`, `never`) |
| `/mocks/{id}/invocations` | GET | List all request/response pairs |
| `/mocks/{id}/invocations` | DELETE | Reset invocations for one mock |
| `/verify` | DELETE | Reset all verification data |

For full details, see the [Mock Verification guide](/guides/mock-verification/).

## Next Steps

- [Basic Mocks](/examples/basic-mocks) - Simple mock examples
- [CRUD API](/examples/crud-api) - Stateful API example
- [Mock Verification](/guides/mock-verification/) - Full verification guide
- [Admin API](/reference/admin-api) - Runtime management
