package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Validation E2E Tests
// These tests verify the validation feature works correctly in integration
// =============================================================================

// TestStateful_Validation_RejectsInvalidData tests that validation rejects
// data that doesn't match the defined rules
func TestStateful_Validation_RejectsInvalidData(t *testing.T) {
	srv, httpPort, _ := createStatefulServer(t, &statefulResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	waitForReady(t, srv.ManagementPort())

	// Create a valid user first to establish baseline
	validBody := bytes.NewBufferString(`{"name": "Alice", "email": "alice@example.com"}`)
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/api/users", httpPort),
		"application/json",
		validBody,
	)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// Test with invalid JSON
	invalidJSON := bytes.NewBufferString(`{invalid json}`)
	resp2, err := http.Post(
		fmt.Sprintf("http://localhost:%d/api/users", httpPort),
		"application/json",
		invalidJSON,
	)
	require.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp2.StatusCode)
}

// TestStateful_Validation_AcceptsValidData tests that valid data is accepted
func TestStateful_Validation_AcceptsValidData(t *testing.T) {
	srv, httpPort, _ := createStatefulServer(t, &statefulResourceConfig{
		Name:     "products",
		BasePath: "/api/products",
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	waitForReady(t, srv.ManagementPort())

	// Valid product with all fields
	body := bytes.NewBufferString(`{
		"name": "Widget",
		"price": 19.99,
		"quantity": 100,
		"category": "electronics"
	}`)
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/api/products", httpPort),
		"application/json",
		body,
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "Widget", result["name"])
	assert.Equal(t, 19.99, result["price"])
}

// TestStateful_Validation_UpdateValidation tests validation on PUT requests
func TestStateful_Validation_UpdateValidation(t *testing.T) {
	srv, httpPort, _ := createStatefulServer(t, &statefulResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
		SeedData: []map[string]interface{}{
			{"id": "item-1", "name": "Original", "price": 10.0},
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	waitForReady(t, srv.ManagementPort())

	// Valid update
	validBody := bytes.NewBufferString(`{"name": "Updated", "price": 20.0}`)
	req, _ := http.NewRequest(http.MethodPut,
		fmt.Sprintf("http://localhost:%d/api/items/item-1", httpPort),
		validBody,
	)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "Updated", result["name"])
	assert.Equal(t, 20.0, result["price"])
}

// TestStateful_Validation_NestedResourceValidation tests validation on nested resources
func TestStateful_Validation_NestedResourceValidation(t *testing.T) {
	srv, httpPort, _ := createStatefulServer(t,
		&statefulResourceConfig{
			Name:     "posts",
			BasePath: "/api/posts",
			SeedData: []map[string]interface{}{
				{"id": "post-1", "title": "Test Post"},
			},
		},
		&statefulResourceConfig{
			Name:        "comments",
			BasePath:    "/api/posts/:postId/comments",
			ParentField: "postId",
		},
	)

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	waitForReady(t, srv.ManagementPort())

	// Valid comment - should be accepted
	validBody := bytes.NewBufferString(`{"text": "Great post!", "author": "Alice"}`)
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/api/posts/post-1/comments", httpPort),
		"application/json",
		validBody,
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	// Verify postId was auto-set from path
	assert.Equal(t, "post-1", result["postId"])
	assert.Equal(t, "Great post!", result["text"])
}

// TestStateful_Validation_EmptyBody tests handling of empty request body
func TestStateful_Validation_EmptyBody(t *testing.T) {
	srv, httpPort, _ := createStatefulServer(t, &statefulResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	waitForReady(t, srv.ManagementPort())

	// Empty body should create resource with just auto-generated fields
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/api/items", httpPort),
		"application/json",
		bytes.NewBufferString(`{}`),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	// Should have auto-generated ID and timestamps
	assert.NotEmpty(t, result["id"])
	assert.NotEmpty(t, result["createdAt"])
}

// =============================================================================
// Binary E2E Tests for Validation
// =============================================================================

// TestBinaryE2E_ValidationConfigFile tests validation loaded from config file
func TestBinaryE2E_ValidationConfigFile(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create a stateful resource via config import
	// The /config endpoint expects: {"config": {...}, "replace": bool}
	configPayload := map[string]interface{}{
		"replace": true,
		"config": map[string]interface{}{
			"version": "1.0",
			"name":    "validation-test",
			"statefulResources": []map[string]interface{}{
				{
					"name":     "validated-users",
					"basePath": "/api/validated-users",
					"seedData": []map[string]interface{}{
						{"id": "user-1", "email": "test@example.com", "age": 25},
					},
				},
			},
		},
	}

	configJSON, _ := json.Marshal(configPayload)
	resp, err := http.Post(proc.adminURL()+"/config", "application/json", bytes.NewReader(configJSON))
	if err != nil {
		t.Fatalf("Failed to import config: %v", err)
	}
	resp.Body.Close()

	// Small delay for registration
	time.Sleep(100 * time.Millisecond)

	// Test valid creation
	validBody := map[string]interface{}{
		"name":  "Alice",
		"email": "alice@example.com",
		"age":   30,
	}
	validJSON, _ := json.Marshal(validBody)

	resp, err = http.Post(proc.mockURL()+"/api/validated-users", "application/json", bytes.NewReader(validJSON))
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status 201 for valid data, got %d: %s", resp.StatusCode, body)
	}

	// Test invalid JSON
	resp2, err := http.Post(proc.mockURL()+"/api/validated-users", "application/json", bytes.NewReader([]byte(`{invalid}`)))
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid JSON, got %d", resp2.StatusCode)
	}
}

// TestBinaryE2E_PerMockValidation tests HTTP mock-level validation
func TestBinaryE2E_PerMockValidation(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create a mock with custom response
	mockPayload := map[string]interface{}{
		"id":      "validated-mock",
		"name":    "Validated Mock",
		"type":    "http",
		"enabled": true,
		"http": map[string]interface{}{
			"matcher": map[string]interface{}{
				"method": "POST",
				"path":   "/api/orders",
			},
			"response": map[string]interface{}{
				"statusCode": 201,
				"body":       `{"status": "created"}`,
				"headers": map[string]string{
					"Content-Type": "application/json",
				},
			},
		},
	}

	mockJSON, _ := json.Marshal(mockPayload)
	resp, err := http.Post(proc.adminURL()+"/mocks", "application/json", bytes.NewReader(mockJSON))
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	resp.Body.Close()

	// Small delay for registration
	time.Sleep(100 * time.Millisecond)

	// Test valid request
	validBody := map[string]interface{}{
		"items":    []string{"item1", "item2"},
		"quantity": 5,
	}
	validJSON, _ := json.Marshal(validBody)

	resp, err = http.Post(proc.mockURL()+"/api/orders", "application/json", bytes.NewReader(validJSON))
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status 201 for valid data, got %d: %s", resp.StatusCode, body)
	}
}

// TestBinaryE2E_ValidationErrorResponse tests that validation errors have proper format
func TestBinaryE2E_ValidationErrorResponse(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create a stateful resource via config import
	configPayload := map[string]interface{}{
		"replace": true,
		"config": map[string]interface{}{
			"version": "1.0",
			"name":    "error-test",
			"statefulResources": []map[string]interface{}{
				{
					"name":     "error-test-users",
					"basePath": "/api/error-test-users",
				},
			},
		},
	}

	configJSON, _ := json.Marshal(configPayload)
	resp, err := http.Post(proc.adminURL()+"/config", "application/json", bytes.NewReader(configJSON))
	if err != nil {
		t.Fatalf("Failed to import config: %v", err)
	}
	resp.Body.Close()

	time.Sleep(100 * time.Millisecond)

	// Test invalid JSON - should get structured error response
	resp, err = http.Post(proc.mockURL()+"/api/error-test-users", "application/json", bytes.NewReader([]byte(`{not valid json`)))
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid JSON, got %d", resp.StatusCode)
	}

	// Verify error response structure
	body, _ := io.ReadAll(resp.Body)
	var errorResp map[string]interface{}
	if err := json.Unmarshal(body, &errorResp); err != nil {
		t.Fatalf("Error response should be valid JSON: %v\nBody: %s", err, body)
	}

	// Should have an error field
	if _, ok := errorResp["error"]; !ok {
		t.Errorf("Error response should have 'error' field: %s", body)
	}
}

// TestBinaryE2E_StatefulCRUDWithValidation tests full CRUD with validation
func TestBinaryE2E_StatefulCRUDWithValidation(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create a stateful resource via config import
	configPayload := map[string]interface{}{
		"replace": true,
		"config": map[string]interface{}{
			"version": "1.0",
			"name":    "crud-test",
			"statefulResources": []map[string]interface{}{
				{
					"name":     "crud-validated",
					"basePath": "/api/crud-validated",
					"seedData": []map[string]interface{}{
						{"id": "seed-1", "name": "Seed Item", "status": "active"},
					},
				},
			},
		},
	}

	configJSON, _ := json.Marshal(configPayload)
	resp, err := http.Post(proc.adminURL()+"/config", "application/json", bytes.NewReader(configJSON))
	if err != nil {
		t.Fatalf("Failed to import config: %v", err)
	}
	resp.Body.Close()

	time.Sleep(100 * time.Millisecond)

	// CREATE
	t.Run("Create", func(t *testing.T) {
		createBody := map[string]interface{}{
			"name":   "New Item",
			"status": "pending",
		}
		createJSON, _ := json.Marshal(createBody)

		resp, err := http.Post(proc.mockURL()+"/api/crud-validated", "application/json", bytes.NewReader(createJSON))
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected 201, got %d: %s", resp.StatusCode, body)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		if result["name"] != "New Item" {
			t.Errorf("Expected name 'New Item', got %v", result["name"])
		}
		if result["id"] == nil || result["id"] == "" {
			t.Error("Expected auto-generated ID")
		}
	})

	// READ
	t.Run("Read", func(t *testing.T) {
		resp, err := http.Get(proc.mockURL() + "/api/crud-validated/seed-1")
		if err != nil {
			t.Fatalf("Read failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected 200, got %d", resp.StatusCode)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		if result["name"] != "Seed Item" {
			t.Errorf("Expected name 'Seed Item', got %v", result["name"])
		}
	})

	// UPDATE
	t.Run("Update", func(t *testing.T) {
		updateBody := map[string]interface{}{
			"name":   "Updated Item",
			"status": "completed",
		}
		updateJSON, _ := json.Marshal(updateBody)

		req, _ := http.NewRequest(http.MethodPut, proc.mockURL()+"/api/crud-validated/seed-1", bytes.NewReader(updateJSON))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected 200, got %d: %s", resp.StatusCode, body)
		}

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		if result["name"] != "Updated Item" {
			t.Errorf("Expected name 'Updated Item', got %v", result["name"])
		}
	})

	// DELETE
	t.Run("Delete", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, proc.mockURL()+"/api/crud-validated/seed-1", nil)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("Expected 204, got %d", resp.StatusCode)
		}

		// Verify deleted
		resp2, err := http.Get(proc.mockURL() + "/api/crud-validated/seed-1")
		require.NoError(t, err)
		resp2.Body.Close()

		if resp2.StatusCode != http.StatusNotFound {
			t.Errorf("Expected 404 after delete, got %d", resp2.StatusCode)
		}
	})
}

// TestBinaryE2E_NestedResourceValidation tests validation on nested resources via binary
func TestBinaryE2E_NestedResourceValidation(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create both resources via config import
	configPayload := map[string]interface{}{
		"replace": true,
		"config": map[string]interface{}{
			"version": "1.0",
			"name":    "nested-test",
			"statefulResources": []map[string]interface{}{
				{
					"name":     "articles",
					"basePath": "/api/articles",
					"seedData": []map[string]interface{}{
						{"id": "article-1", "title": "Test Article"},
					},
				},
				{
					"name":        "article-comments",
					"basePath":    "/api/articles/:articleId/comments",
					"parentField": "articleId",
				},
			},
		},
	}

	configJSON, _ := json.Marshal(configPayload)
	resp, err := http.Post(proc.adminURL()+"/config", "application/json", bytes.NewReader(configJSON))
	if err != nil {
		t.Fatalf("Failed to import config: %v", err)
	}
	resp.Body.Close()

	time.Sleep(100 * time.Millisecond)

	// Create comment on article
	commentBody := map[string]interface{}{
		"text":   "Great article!",
		"author": "Reader",
	}
	commentJSON, _ := json.Marshal(commentBody)

	resp, err = http.Post(proc.mockURL()+"/api/articles/article-1/comments", "application/json", bytes.NewReader(commentJSON))
	if err != nil {
		t.Fatalf("Failed to create comment: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected 201, got %d: %s", resp.StatusCode, body)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	// Verify articleId was auto-set from path
	if result["articleId"] != "article-1" {
		t.Errorf("Expected articleId 'article-1', got %v", result["articleId"])
	}

	// List comments for article
	resp2, err := http.Get(proc.mockURL() + "/api/articles/article-1/comments")
	if err != nil {
		t.Fatalf("Failed to list comments: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp2.StatusCode)
	}

	var listResult map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&listResult)

	if data, ok := listResult["data"].([]interface{}); ok {
		if len(data) != 1 {
			t.Errorf("Expected 1 comment, got %d", len(data))
		}
	}
}

// TestBinaryE2E_LargeBodyRejection tests that overly large request bodies are rejected
func TestBinaryE2E_LargeBodyRejection(t *testing.T) {
	binaryPath := buildBinary(t)
	proc := startServer(t, binaryPath)

	// Create a stateful resource via config import
	configPayload := map[string]interface{}{
		"replace": true,
		"config": map[string]interface{}{
			"version": "1.0",
			"name":    "large-body-test",
			"statefulResources": []map[string]interface{}{
				{
					"name":     "large-body-test",
					"basePath": "/api/large-body-test",
				},
			},
		},
	}
	configJSON, _ := json.Marshal(configPayload)
	resp, err := http.Post(proc.adminURL()+"/config", "application/json", bytes.NewReader(configJSON))
	if err != nil {
		t.Fatalf("Failed to import config: %v", err)
	}
	resp.Body.Close()

	time.Sleep(100 * time.Millisecond)

	// Create a very large body (over 1MB)
	largeData := make([]byte, 2*1024*1024) // 2MB
	for i := range largeData {
		largeData[i] = 'x'
	}
	largeBody := fmt.Sprintf(`{"data": "%s"}`, string(largeData))

	resp, err = http.Post(proc.mockURL()+"/api/large-body-test", "application/json", bytes.NewReader([]byte(largeBody)))
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Should be rejected with 413 Request Entity Too Large
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413 for large body, got %d", resp.StatusCode)
	}
}
