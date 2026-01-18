# CRUD API Example

This example demonstrates mockd's stateful mocking feature to simulate a complete CRUD (Create, Read, Update, Delete) API.

## Overview

We'll create a mock API for a task management system with:

- Tasks (main resource)
- Users (for assignment)
- Persistent state across requests

## Configuration

Create `tasks-api.json`:

```json
{
  "server": {
    "port": 4280
  },
  "stateful": {
    "resources": {
      "users": {
        "collection": "/api/users",
        "item": "/api/users/{id}",
        "idField": "id",
        "autoId": true,
        "seed": [
          {"id": 1, "name": "Alice", "email": "alice@example.com"},
          {"id": 2, "name": "Bob", "email": "bob@example.com"}
        ]
      },
      "tasks": {
        "collection": "/api/tasks",
        "item": "/api/tasks/{id}",
        "idField": "id",
        "autoId": true,
        "filtering": true,
        "seed": [
          {
            "id": 1,
            "title": "Setup project",
            "description": "Initialize the project structure",
            "status": "done",
            "assigneeId": 1,
            "createdAt": "2024-01-10T09:00:00Z"
          },
          {
            "id": 2,
            "title": "Write documentation",
            "description": "Create user documentation",
            "status": "in_progress",
            "assigneeId": 2,
            "createdAt": "2024-01-11T10:00:00Z"
          },
          {
            "id": 3,
            "title": "Add tests",
            "description": "Write unit tests",
            "status": "todo",
            "assigneeId": null,
            "createdAt": "2024-01-12T11:00:00Z"
          }
        ]
      }
    }
  },
  "mocks": [
    {
      "name": "Health check",
      "request": {
        "method": "GET",
        "path": "/health"
      },
      "response": {
        "status": 200,
        "body": {"status": "ok", "timestamp": "{{now}}"}
      }
    }
  ]
}
```

## Start the Server

```bash
mockd start --config tasks-api.json
```

## API Operations

### List All Tasks

```bash
curl http://localhost:4280/api/tasks
```

Response:

```json
[
  {
    "id": 1,
    "title": "Setup project",
    "status": "done",
    "assigneeId": 1,
    "createdAt": "2024-01-10T09:00:00Z"
  },
  {
    "id": 2,
    "title": "Write documentation",
    "status": "in_progress",
    "assigneeId": 2,
    "createdAt": "2024-01-11T10:00:00Z"
  },
  {
    "id": 3,
    "title": "Add tests",
    "status": "todo",
    "assigneeId": null,
    "createdAt": "2024-01-12T11:00:00Z"
  }
]
```

### Filter Tasks

```bash
# By status
curl "http://localhost:4280/api/tasks?status=todo"

# By assignee
curl "http://localhost:4280/api/tasks?assigneeId=1"
```

### Get Single Task

```bash
curl http://localhost:4280/api/tasks/2
```

Response:

```json
{
  "id": 2,
  "title": "Write documentation",
  "description": "Create user documentation",
  "status": "in_progress",
  "assigneeId": 2,
  "createdAt": "2024-01-11T10:00:00Z"
}
```

### Create Task

```bash
curl -X POST http://localhost:4280/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Review PR",
    "description": "Review pull request #42",
    "status": "todo",
    "assigneeId": 1
  }'
```

Response:

```json
{
  "id": 4,
  "title": "Review PR",
  "description": "Review pull request #42",
  "status": "todo",
  "assigneeId": 1
}
```

### Update Task

```bash
curl -X PUT http://localhost:4280/api/tasks/4 \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Review PR",
    "description": "Review pull request #42",
    "status": "in_progress",
    "assigneeId": 1
  }'
```

### Partial Update (PATCH)

```bash
curl -X PATCH http://localhost:4280/api/tasks/4 \
  -H "Content-Type: application/json" \
  -d '{"status": "done"}'
```

### Delete Task

```bash
curl -X DELETE http://localhost:4280/api/tasks/4
```

Response: `204 No Content`

Verify deletion:

```bash
curl http://localhost:4280/api/tasks/4
```

Response: `404 Not Found`

## User Operations

### List Users

```bash
curl http://localhost:4280/api/users
```

### Create User

```bash
curl -X POST http://localhost:4280/api/users \
  -H "Content-Type: application/json" \
  -d '{"name": "Charlie", "email": "charlie@example.com"}'
```

## State Management

### View Current State

```bash
curl http://localhost:4290/state
```

### Reset State

```bash
# Reset all state
curl -X DELETE http://localhost:4290/state

# Reset specific resource
curl -X DELETE http://localhost:4290/state/tasks
```

### Import State

```bash
curl -X POST http://localhost:4290/state \
  -H "Content-Type: application/json" \
  -d '{
    "tasks": [
      {"id": 1, "title": "Fresh task", "status": "todo"}
    ]
  }'
```

## With Persistence

Enable file persistence so state survives restarts:

```json
{
  "stateful": {
    "resources": { ... },
    "persistence": {
      "enabled": true,
      "file": "./state.json",
      "saveInterval": "10s"
    }
  }
}
```

## Workflow Example

Simulate a complete workflow:

```bash
# 1. Create a new task
TASK=$(curl -s -X POST http://localhost:4280/api/tasks \
  -H "Content-Type: application/json" \
  -d '{"title": "New feature", "status": "todo"}')
TASK_ID=$(echo $TASK | jq -r '.id')

# 2. Assign to user
curl -X PATCH http://localhost:4280/api/tasks/$TASK_ID \
  -H "Content-Type: application/json" \
  -d '{"assigneeId": 1}'

# 3. Start work
curl -X PATCH http://localhost:4280/api/tasks/$TASK_ID \
  -H "Content-Type: application/json" \
  -d '{"status": "in_progress"}'

# 4. Complete task
curl -X PATCH http://localhost:4280/api/tasks/$TASK_ID \
  -H "Content-Type: application/json" \
  -d '{"status": "done"}'

# 5. Verify
curl http://localhost:4280/api/tasks/$TASK_ID
```

## Integration with Tests

### JavaScript/Node.js

```javascript
const API = 'http://localhost:4280/api';

describe('Tasks API', () => {
  beforeEach(async () => {
    // Reset state before each test
    await fetch('http://localhost:4290/state', {
      method: 'DELETE'
    });
  });

  test('create and list tasks', async () => {
    // Create task
    const createRes = await fetch(`${API}/tasks`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: 'Test task', status: 'todo' })
    });
    const task = await createRes.json();
    expect(task.id).toBeDefined();

    // List tasks
    const listRes = await fetch(`${API}/tasks`);
    const tasks = await listRes.json();
    expect(tasks).toHaveLength(1);
    expect(tasks[0].title).toBe('Test task');
  });

  test('update task status', async () => {
    // Create
    const createRes = await fetch(`${API}/tasks`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: 'Task', status: 'todo' })
    });
    const { id } = await createRes.json();

    // Update
    await fetch(`${API}/tasks/${id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ status: 'done' })
    });

    // Verify
    const getRes = await fetch(`${API}/tasks/${id}`);
    const task = await getRes.json();
    expect(task.status).toBe('done');
  });
});
```

### Python

```python
import requests

API = 'http://localhost:4280/api'
ADMIN = 'http://localhost:4290'

def test_task_crud():
    # Reset state
    requests.delete(f'{ADMIN}/state')

    # Create
    task = requests.post(f'{API}/tasks', json={
        'title': 'Test task',
        'status': 'todo'
    }).json()
    assert 'id' in task

    # Read
    fetched = requests.get(f'{API}/tasks/{task["id"]}').json()
    assert fetched['title'] == 'Test task'

    # Update
    requests.patch(f'{API}/tasks/{task["id"]}', json={
        'status': 'done'
    })
    updated = requests.get(f'{API}/tasks/{task["id"]}').json()
    assert updated['status'] == 'done'

    # Delete
    resp = requests.delete(f'{API}/tasks/{task["id"]}')
    assert resp.status_code == 204

    # Verify deleted
    resp = requests.get(f'{API}/tasks/{task["id"]}')
    assert resp.status_code == 404
```

## Next Steps

- [Integration Testing](integration-testing.md) - More testing patterns
- [Stateful Mocking Guide](../guides/stateful-mocking.md) - Full reference
- [Admin API](../reference/admin-api.md) - State management API
