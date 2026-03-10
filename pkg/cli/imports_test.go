package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

// ---------------------------------------------------------------------------
// importRef
// ---------------------------------------------------------------------------

func TestImportRef(t *testing.T) {
	tests := []struct {
		name string
		imp  *config.ImportEntry
		want string
	}{
		{
			name: "path takes precedence",
			imp:  &config.ImportEntry{Path: "specs/api.yaml", URL: "https://example.com/api.yaml"},
			want: "specs/api.yaml",
		},
		{
			name: "url returned when no path",
			imp:  &config.ImportEntry{URL: "https://example.com/api.yaml"},
			want: "https://example.com/api.yaml",
		},
		{
			name: "empty entry returns empty string",
			imp:  &config.ImportEntry{},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := importRef(tt.imp)
			if got != tt.want {
				t.Errorf("importRef() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// applyNamespace
// ---------------------------------------------------------------------------

func TestApplyNamespace(t *testing.T) {
	tests := []struct {
		name      string
		mocks     []*mock.Mock
		namespace string
		wantIDs   []string // expected operationIds after apply
	}{
		{
			name: "prefixes operationId with namespace",
			mocks: []*mock.Mock{
				{OperationID: "GetUsers"},
				{OperationID: "PostCustomers"},
			},
			namespace: "stripe",
			wantIDs:   []string{"stripe.GetUsers", "stripe.PostCustomers"},
		},
		{
			name: "skips mocks without operationId",
			mocks: []*mock.Mock{
				{OperationID: "GetUsers"},
				{OperationID: ""}, // no operationId
			},
			namespace: "api",
			wantIDs:   []string{"api.GetUsers", ""},
		},
		{
			name:      "empty mocks list is a no-op",
			mocks:     []*mock.Mock{},
			namespace: "ns",
			wantIDs:   []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			col := &config.MockCollection{Mocks: tt.mocks}
			applyNamespace(col, tt.namespace)

			if len(col.Mocks) != len(tt.wantIDs) {
				t.Fatalf("mock count = %d, want %d", len(col.Mocks), len(tt.wantIDs))
			}
			for i, m := range col.Mocks {
				if m.OperationID != tt.wantIDs[i] {
					t.Errorf("mock[%d].OperationID = %q, want %q", i, m.OperationID, tt.wantIDs[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// readImportSource
// ---------------------------------------------------------------------------

func TestReadImportSource(t *testing.T) {
	t.Run("reads local file with absolute path", func(t *testing.T) {
		dir := t.TempDir()
		content := []byte(`mocks: []`)
		fpath := filepath.Join(dir, "spec.yaml")
		if err := os.WriteFile(fpath, content, 0644); err != nil {
			t.Fatal(err)
		}

		imp := &config.ImportEntry{Path: fpath}
		data, filename, err := readImportSource(imp, "/unused")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != string(content) {
			t.Errorf("data = %q, want %q", data, content)
		}
		if filename != "spec.yaml" {
			t.Errorf("filename = %q, want %q", filename, "spec.yaml")
		}
	})

	t.Run("reads local file with relative path resolved via configDir", func(t *testing.T) {
		dir := t.TempDir()
		content := []byte(`version: "1.0"`)
		fpath := filepath.Join(dir, "sub", "api.yaml")
		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fpath, content, 0644); err != nil {
			t.Fatal(err)
		}

		imp := &config.ImportEntry{Path: "sub/api.yaml"}
		data, filename, err := readImportSource(imp, dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != string(content) {
			t.Errorf("data = %q, want %q", data, content)
		}
		if filename != "api.yaml" {
			t.Errorf("filename = %q, want %q", filename, "api.yaml")
		}
	})

	t.Run("file not found returns error with path", func(t *testing.T) {
		imp := &config.ImportEntry{Path: "/does/not/exist/spec.yaml"}
		_, _, err := readImportSource(imp, "/tmp")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
		if !strings.Contains(err.Error(), "/does/not/exist/spec.yaml") {
			t.Errorf("error should contain file path, got: %v", err)
		}
	})

	t.Run("reads from URL", func(t *testing.T) {
		want := `{"mocks":[]}`
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(want))
		}))
		defer srv.Close()

		imp := &config.ImportEntry{URL: srv.URL + "/api.json"}
		data, filename, err := readImportSource(imp, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != want {
			t.Errorf("data = %q, want %q", data, want)
		}
		if filename != "api.json" {
			t.Errorf("filename = %q, want %q", filename, "api.json")
		}
	})

	t.Run("URL 404 returns error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(404)
		}))
		defer srv.Close()

		imp := &config.ImportEntry{URL: srv.URL + "/missing.json"}
		_, _, err := readImportSource(imp, "")
		if err == nil {
			t.Fatal("expected error for 404 URL")
		}
	})

	t.Run("neither path nor url returns error", func(t *testing.T) {
		imp := &config.ImportEntry{}
		_, _, err := readImportSource(imp, "/tmp")
		if err == nil {
			t.Fatal("expected error when both path and url are empty")
		}
		if !strings.Contains(err.Error(), "path") || !strings.Contains(err.Error(), "url") {
			t.Errorf("error should mention path/url, got: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// processImports
// ---------------------------------------------------------------------------

// validMockdYAML is the simplest valid mockd config for import testing.
const validMockdYAML = `
version: "1.0"
mocks:
  - id: get-users
    type: http
    operationId: GetUsers
    http:
      matcher:
        method: GET
        path: /api/users
      response:
        statusCode: 200
        body: "[]"
  - id: create-user
    type: http
    operationId: CreateUser
    http:
      matcher:
        method: POST
        path: /api/users
      response:
        statusCode: 201
        body: "{}"
`

func TestProcessImports(t *testing.T) {
	t.Run("empty imports list is a no-op", func(t *testing.T) {
		col := &config.MockCollection{Imports: nil}
		if err := processImports(col, "/tmp"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(col.Mocks) != 0 {
			t.Errorf("mocks should be empty, got %d", len(col.Mocks))
		}
	})

	t.Run("empty imports slice is a no-op", func(t *testing.T) {
		col := &config.MockCollection{Imports: []*config.ImportEntry{}}
		if err := processImports(col, "/tmp"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("single file import loads mocks", func(t *testing.T) {
		dir := t.TempDir()
		fpath := filepath.Join(dir, "api.yaml")
		if err := os.WriteFile(fpath, []byte(validMockdYAML), 0644); err != nil {
			t.Fatal(err)
		}

		col := &config.MockCollection{
			Imports: []*config.ImportEntry{
				{Path: "api.yaml"},
			},
		}
		if err := processImports(col, dir); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(col.Mocks) != 2 {
			t.Fatalf("expected 2 mocks, got %d", len(col.Mocks))
		}
	})

	t.Run("single file import with namespace prefixes operationIds", func(t *testing.T) {
		dir := t.TempDir()
		fpath := filepath.Join(dir, "api.yaml")
		if err := os.WriteFile(fpath, []byte(validMockdYAML), 0644); err != nil {
			t.Fatal(err)
		}

		col := &config.MockCollection{
			Imports: []*config.ImportEntry{
				{Path: "api.yaml", As: "myapi"},
			},
		}
		if err := processImports(col, dir); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(col.Mocks) != 2 {
			t.Fatalf("expected 2 mocks, got %d", len(col.Mocks))
		}

		for _, m := range col.Mocks {
			if m.OperationID == "" {
				continue
			}
			if !strings.HasPrefix(m.OperationID, "myapi.") {
				t.Errorf("operationId %q should have prefix 'myapi.'", m.OperationID)
			}
		}
	})

	t.Run("import file not found returns error", func(t *testing.T) {
		col := &config.MockCollection{
			Imports: []*config.ImportEntry{
				{Path: "nonexistent.yaml"},
			},
		}
		err := processImports(col, "/tmp")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
		if !strings.Contains(err.Error(), "nonexistent.yaml") {
			t.Errorf("error should contain filename, got: %v", err)
		}
	})

	t.Run("import with unknown explicit format returns error", func(t *testing.T) {
		dir := t.TempDir()
		fpath := filepath.Join(dir, "api.yaml")
		if err := os.WriteFile(fpath, []byte(validMockdYAML), 0644); err != nil {
			t.Fatal(err)
		}

		col := &config.MockCollection{
			Imports: []*config.ImportEntry{
				{Path: "api.yaml", Format: "blobformat"},
			},
		}
		err := processImports(col, dir)
		if err == nil {
			t.Fatal("expected error for unknown format")
		}
		if !strings.Contains(err.Error(), "unknown format") {
			t.Errorf("error should mention 'unknown format', got: %v", err)
		}
		if !strings.Contains(err.Error(), "blobformat") {
			t.Errorf("error should contain the bad format name, got: %v", err)
		}
	})

	t.Run("import with explicit format uses it", func(t *testing.T) {
		dir := t.TempDir()
		fpath := filepath.Join(dir, "config.txt") // extension won't help auto-detect
		if err := os.WriteFile(fpath, []byte(validMockdYAML), 0644); err != nil {
			t.Fatal(err)
		}

		col := &config.MockCollection{
			Imports: []*config.ImportEntry{
				{Path: "config.txt", Format: "mockd"},
			},
		}
		if err := processImports(col, dir); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(col.Mocks) < 1 {
			t.Error("expected at least 1 mock from explicit-format import")
		}
	})

	t.Run("import with undetectable format returns error", func(t *testing.T) {
		dir := t.TempDir()
		fpath := filepath.Join(dir, "unknown.bin")
		if err := os.WriteFile(fpath, []byte("totally random bytes"), 0644); err != nil {
			t.Fatal(err)
		}

		col := &config.MockCollection{
			Imports: []*config.ImportEntry{
				{Path: "unknown.bin"},
			},
		}
		err := processImports(col, dir)
		if err == nil {
			t.Fatal("expected error for undetectable format")
		}
		if !strings.Contains(err.Error(), "could not detect format") {
			t.Errorf("error should mention format detection failure, got: %v", err)
		}
	})

	t.Run("multiple imports merge mocks in order", func(t *testing.T) {
		dir := t.TempDir()

		yaml1 := `
version: "1.0"
mocks:
  - id: first-mock
    type: http
    operationId: First
    http:
      matcher:
        method: GET
        path: /first
      response:
        statusCode: 200
        body: "1"
`
		yaml2 := `
version: "1.0"
mocks:
  - id: second-mock
    type: http
    operationId: Second
    http:
      matcher:
        method: GET
        path: /second
      response:
        statusCode: 200
        body: "2"
`
		if err := os.WriteFile(filepath.Join(dir, "a.yaml"), []byte(yaml1), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "b.yaml"), []byte(yaml2), 0644); err != nil {
			t.Fatal(err)
		}

		col := &config.MockCollection{
			Imports: []*config.ImportEntry{
				{Path: "a.yaml"},
				{Path: "b.yaml"},
			},
		}
		if err := processImports(col, dir); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(col.Mocks) != 2 {
			t.Fatalf("expected 2 mocks, got %d", len(col.Mocks))
		}
		// Verify ordering: first import's mocks come first
		if col.Mocks[0].OperationID != "First" {
			t.Errorf("first mock operationId = %q, want %q", col.Mocks[0].OperationID, "First")
		}
		if col.Mocks[1].OperationID != "Second" {
			t.Errorf("second mock operationId = %q, want %q", col.Mocks[1].OperationID, "Second")
		}
	})

	t.Run("import from URL loads mocks", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/yaml")
			w.Write([]byte(validMockdYAML))
		}))
		defer srv.Close()

		col := &config.MockCollection{
			Imports: []*config.ImportEntry{
				{URL: srv.URL + "/api.yaml"},
			},
		}
		if err := processImports(col, ""); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(col.Mocks) != 2 {
			t.Fatalf("expected 2 mocks from URL import, got %d", len(col.Mocks))
		}
	})

	t.Run("existing mocks are preserved when imports add more", func(t *testing.T) {
		dir := t.TempDir()
		fpath := filepath.Join(dir, "api.yaml")
		if err := os.WriteFile(fpath, []byte(validMockdYAML), 0644); err != nil {
			t.Fatal(err)
		}

		existing := &mock.Mock{
			ID:          "existing-1",
			OperationID: "ExistingOp",
			Type:        mock.TypeHTTP,
			HTTP: &mock.HTTPSpec{
				Matcher:  &mock.HTTPMatcher{Method: "GET", Path: "/existing"},
				Response: &mock.HTTPResponse{StatusCode: 200},
			},
		}
		col := &config.MockCollection{
			Mocks: []*mock.Mock{existing},
			Imports: []*config.ImportEntry{
				{Path: "api.yaml"},
			},
		}
		if err := processImports(col, dir); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(col.Mocks) != 3 {
			t.Fatalf("expected 3 mocks (1 existing + 2 imported), got %d", len(col.Mocks))
		}
		if col.Mocks[0].OperationID != "ExistingOp" {
			t.Error("existing mock should be first in the list")
		}
	})
}

// ---------------------------------------------------------------------------
// processTablesAndExtend
// ---------------------------------------------------------------------------

func TestProcessTablesAndExtend(t *testing.T) {
	// --------------- Table conversion tests ---------------

	t.Run("empty tables is a no-op", func(t *testing.T) {
		col := &config.MockCollection{}
		if err := processTablesAndExtend(col); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(col.StatefulResources) != 0 {
			t.Errorf("stateful resources should be empty, got %d", len(col.StatefulResources))
		}
	})

	t.Run("nil tables is a no-op", func(t *testing.T) {
		col := &config.MockCollection{Tables: nil}
		if err := processTablesAndExtend(col); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("single table creates StatefulResourceConfig", func(t *testing.T) {
		seed := []map[string]interface{}{
			{"id": "1", "name": "Alice"},
		}
		relationships := map[string]*config.Relationship{
			"orders": {Table: "orders", Field: "customer_id"},
		}
		col := &config.MockCollection{
			Tables: []*config.TableConfig{
				{
					Name:          "users",
					IDField:       "id",
					IDStrategy:    "uuid",
					IDPrefix:      "usr_",
					MaxItems:      100,
					ParentField:   "org_id",
					SeedData:      seed,
					Relationships: relationships,
					Response: &config.ResponseTransform{
						List: &config.ListTransform{
							DataField: "results",
						},
					},
				},
			},
		}

		if err := processTablesAndExtend(col); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(col.StatefulResources) != 1 {
			t.Fatalf("expected 1 stateful resource, got %d", len(col.StatefulResources))
		}

		res := col.StatefulResources[0]
		if res.Name != "users" {
			t.Errorf("Name = %q, want %q", res.Name, "users")
		}
		if res.IDField != "id" {
			t.Errorf("IDField = %q, want %q", res.IDField, "id")
		}
		if res.IDStrategy != "uuid" {
			t.Errorf("IDStrategy = %q, want %q", res.IDStrategy, "uuid")
		}
		if res.IDPrefix != "usr_" {
			t.Errorf("IDPrefix = %q, want %q", res.IDPrefix, "usr_")
		}
		if res.MaxItems != 100 {
			t.Errorf("MaxItems = %d, want %d", res.MaxItems, 100)
		}
		if res.ParentField != "org_id" {
			t.Errorf("ParentField = %q, want %q", res.ParentField, "org_id")
		}
		if len(res.SeedData) != 1 {
			t.Errorf("SeedData length = %d, want 1", len(res.SeedData))
		}
		if res.Relationships == nil || res.Relationships["orders"] == nil {
			t.Error("Relationships should include 'orders'")
		}
		if res.Response == nil {
			t.Error("Response transform should be propagated")
		}
	})

	t.Run("multiple tables create multiple resources", func(t *testing.T) {
		col := &config.MockCollection{
			Tables: []*config.TableConfig{
				{Name: "users", IDField: "id"},
				{Name: "orders", IDField: "order_id"},
			},
		}
		if err := processTablesAndExtend(col); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(col.StatefulResources) != 2 {
			t.Fatalf("expected 2 resources, got %d", len(col.StatefulResources))
		}
		if col.StatefulResources[0].Name != "users" {
			t.Errorf("first resource = %q, want %q", col.StatefulResources[0].Name, "users")
		}
		if col.StatefulResources[1].Name != "orders" {
			t.Errorf("second resource = %q, want %q", col.StatefulResources[1].Name, "orders")
		}
	})

	t.Run("duplicate table name returns error", func(t *testing.T) {
		col := &config.MockCollection{
			Tables: []*config.TableConfig{
				{Name: "users"},
				{Name: "users"},
			},
		}
		err := processTablesAndExtend(col)
		if err == nil {
			t.Fatal("expected error for duplicate table name")
		}
		if !strings.Contains(err.Error(), "duplicate") {
			t.Errorf("error should mention 'duplicate', got: %v", err)
		}
		if !strings.Contains(err.Error(), "users") {
			t.Errorf("error should mention table name, got: %v", err)
		}
	})

	t.Run("empty table name returns error", func(t *testing.T) {
		col := &config.MockCollection{
			Tables: []*config.TableConfig{
				{Name: ""},
			},
		}
		err := processTablesAndExtend(col)
		if err == nil {
			t.Fatal("expected error for empty table name")
		}
		if !strings.Contains(err.Error(), "name is required") {
			t.Errorf("error should mention 'name is required', got: %v", err)
		}
	})

	// --------------- Extend binding tests ---------------

	t.Run("extend binding found by operationId sets StatefulBinding", func(t *testing.T) {
		col := &config.MockCollection{
			Mocks: []*mock.Mock{
				{
					OperationID: "ListUsers",
					Type:        mock.TypeHTTP,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/api/users"},
					},
				},
			},
			Tables: []*config.TableConfig{
				{Name: "users", IDField: "id"},
			},
			Extend: []*config.ExtendBinding{
				{Mock: "ListUsers", Table: "users", Action: "list"},
			},
		}

		if err := processTablesAndExtend(col); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sb := col.Mocks[0].HTTP.StatefulBinding
		if sb == nil {
			t.Fatal("StatefulBinding should be set")
		}
		if sb.Table != "users" {
			t.Errorf("Table = %q, want %q", sb.Table, "users")
		}
		if sb.Action != "list" {
			t.Errorf("Action = %q, want %q", sb.Action, "list")
		}
	})

	t.Run("extend binding found by METHOD /path fallback", func(t *testing.T) {
		col := &config.MockCollection{
			Mocks: []*mock.Mock{
				{
					// No operationId — must match via method+path
					Type: mock.TypeHTTP,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/api/users"},
					},
				},
			},
			Tables: []*config.TableConfig{
				{Name: "users", IDField: "id"},
			},
			Extend: []*config.ExtendBinding{
				{Mock: "GET /api/users", Table: "users", Action: "list"},
			},
		}

		if err := processTablesAndExtend(col); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sb := col.Mocks[0].HTTP.StatefulBinding
		if sb == nil {
			t.Fatal("StatefulBinding should be set via METHOD /path fallback")
		}
		if sb.Table != "users" {
			t.Errorf("Table = %q, want %q", sb.Table, "users")
		}
	})

	t.Run("extend binding method/path is case-insensitive on method", func(t *testing.T) {
		col := &config.MockCollection{
			Mocks: []*mock.Mock{
				{
					Type: mock.TypeHTTP,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{Method: "get", Path: "/api/items"},
					},
				},
			},
			Tables: []*config.TableConfig{
				{Name: "items"},
			},
			Extend: []*config.ExtendBinding{
				{Mock: "GET /api/items", Table: "items", Action: "list"},
			},
		}

		if err := processTablesAndExtend(col); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if col.Mocks[0].HTTP.StatefulBinding == nil {
			t.Fatal("binding should match with case-insensitive method")
		}
	})

	t.Run("extend binding mock not found returns error", func(t *testing.T) {
		col := &config.MockCollection{
			Mocks:  []*mock.Mock{},
			Tables: []*config.TableConfig{{Name: "users"}},
			Extend: []*config.ExtendBinding{
				{Mock: "NonExistentOp", Table: "users", Action: "list"},
			},
		}
		err := processTablesAndExtend(col)
		if err == nil {
			t.Fatal("expected error for mock not found")
		}
		if !strings.Contains(err.Error(), "mock not found") {
			t.Errorf("error should mention 'mock not found', got: %v", err)
		}
		if !strings.Contains(err.Error(), "NonExistentOp") {
			t.Errorf("error should contain the mock reference, got: %v", err)
		}
	})

	t.Run("extend binding non-HTTP mock returns error", func(t *testing.T) {
		col := &config.MockCollection{
			Mocks: []*mock.Mock{
				{
					OperationID: "SubTopic",
					Type:        mock.TypeMQTT,
					HTTP:        nil, // no HTTP spec
				},
			},
			Tables: []*config.TableConfig{{Name: "events"}},
			Extend: []*config.ExtendBinding{
				{Mock: "SubTopic", Table: "events", Action: "list"},
			},
		}
		err := processTablesAndExtend(col)
		if err == nil {
			t.Fatal("expected error for non-HTTP mock")
		}
		if !strings.Contains(err.Error(), "not an HTTP mock") {
			t.Errorf("error should mention 'not an HTTP mock', got: %v", err)
		}
	})

	t.Run("extend with action custom requires operation", func(t *testing.T) {
		col := &config.MockCollection{
			Mocks: []*mock.Mock{
				{
					OperationID: "ConfirmPayment",
					Type:        mock.TypeHTTP,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{Method: "POST", Path: "/confirm"},
					},
				},
			},
			Tables: []*config.TableConfig{{Name: "payments"}},
			Extend: []*config.ExtendBinding{
				{Mock: "ConfirmPayment", Table: "payments", Action: "custom"},
			},
		}
		err := processTablesAndExtend(col)
		if err == nil {
			t.Fatal("expected error when action=custom without operation")
		}
		if !strings.Contains(err.Error(), "requires an operation") {
			t.Errorf("error should mention 'requires an operation', got: %v", err)
		}
	})

	t.Run("extend with action custom and operation succeeds", func(t *testing.T) {
		col := &config.MockCollection{
			Mocks: []*mock.Mock{
				{
					OperationID: "ConfirmPayment",
					Type:        mock.TypeHTTP,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{Method: "POST", Path: "/confirm"},
					},
				},
			},
			Tables: []*config.TableConfig{{Name: "payments"}},
			Extend: []*config.ExtendBinding{
				{Mock: "ConfirmPayment", Table: "payments", Action: "custom", Operation: "confirm"},
			},
		}
		if err := processTablesAndExtend(col); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sb := col.Mocks[0].HTTP.StatefulBinding
		if sb == nil {
			t.Fatal("StatefulBinding should be set")
		}
		if sb.Action != "custom" {
			t.Errorf("Action = %q, want %q", sb.Action, "custom")
		}
		if sb.Operation != "confirm" {
			t.Errorf("Operation = %q, want %q", sb.Operation, "confirm")
		}
	})

	t.Run("extend binding missing mock field returns error", func(t *testing.T) {
		col := &config.MockCollection{
			Tables: []*config.TableConfig{{Name: "users"}},
			Extend: []*config.ExtendBinding{
				{Mock: "", Table: "users", Action: "list"},
			},
		}
		err := processTablesAndExtend(col)
		if err == nil {
			t.Fatal("expected error for missing mock field")
		}
		if !strings.Contains(err.Error(), "mock reference is required") {
			t.Errorf("error should mention 'mock reference is required', got: %v", err)
		}
	})

	t.Run("extend binding missing table field returns error", func(t *testing.T) {
		col := &config.MockCollection{
			Mocks: []*mock.Mock{
				{
					OperationID: "GetUsers",
					Type:        mock.TypeHTTP,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/users"},
					},
				},
			},
			Tables: []*config.TableConfig{{Name: "users"}},
			Extend: []*config.ExtendBinding{
				{Mock: "GetUsers", Table: "", Action: "list"},
			},
		}
		err := processTablesAndExtend(col)
		if err == nil {
			t.Fatal("expected error for missing table field")
		}
		if !strings.Contains(err.Error(), "table is required") {
			t.Errorf("error should mention 'table is required', got: %v", err)
		}
	})

	t.Run("extend binding missing action field returns error", func(t *testing.T) {
		col := &config.MockCollection{
			Mocks: []*mock.Mock{
				{
					OperationID: "GetUsers",
					Type:        mock.TypeHTTP,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/users"},
					},
				},
			},
			Tables: []*config.TableConfig{{Name: "users"}},
			Extend: []*config.ExtendBinding{
				{Mock: "GetUsers", Table: "users", Action: ""},
			},
		}
		err := processTablesAndExtend(col)
		if err == nil {
			t.Fatal("expected error for missing action field")
		}
		if !strings.Contains(err.Error(), "action is required") {
			t.Errorf("error should mention 'action is required', got: %v", err)
		}
	})

	t.Run("extend binding references unknown table returns error", func(t *testing.T) {
		col := &config.MockCollection{
			Mocks: []*mock.Mock{
				{
					OperationID: "GetOrders",
					Type:        mock.TypeHTTP,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/orders"},
					},
				},
			},
			Tables: []*config.TableConfig{{Name: "users"}},
			Extend: []*config.ExtendBinding{
				{Mock: "GetOrders", Table: "orders", Action: "list"},
			},
		}
		err := processTablesAndExtend(col)
		if err == nil {
			t.Fatal("expected error for unknown table")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error should mention 'not found', got: %v", err)
		}
		if !strings.Contains(err.Error(), "orders") {
			t.Errorf("error should mention the table name, got: %v", err)
		}
	})

	// --------------- Response transform priority ---------------

	t.Run("response transform: binding override wins over table default", func(t *testing.T) {
		bindingTransform := &config.ResponseTransform{
			List: &config.ListTransform{DataField: "binding_wrapper"},
		}
		tableTransform := &config.ResponseTransform{
			List: &config.ListTransform{DataField: "table_wrapper"},
		}

		col := &config.MockCollection{
			Mocks: []*mock.Mock{
				{
					OperationID: "ListUsers",
					Type:        mock.TypeHTTP,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/users"},
					},
				},
			},
			Tables: []*config.TableConfig{
				{Name: "users", Response: tableTransform},
			},
			Extend: []*config.ExtendBinding{
				{Mock: "ListUsers", Table: "users", Action: "list", Response: bindingTransform},
			},
		}

		if err := processTablesAndExtend(col); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sb := col.Mocks[0].HTTP.StatefulBinding
		if sb == nil || sb.Response == nil {
			t.Fatal("StatefulBinding.Response should be set")
		}
		transform, ok := sb.Response.Transform.(*config.ResponseTransform)
		if !ok {
			t.Fatalf("Response.Transform type = %T, want *config.ResponseTransform", sb.Response.Transform)
		}
		if transform.List == nil || transform.List.DataField != "binding_wrapper" {
			t.Error("binding response transform should override table default")
		}
	})

	t.Run("response transform: table default used when binding has none", func(t *testing.T) {
		tableTransform := &config.ResponseTransform{
			List: &config.ListTransform{DataField: "table_wrapper"},
		}

		col := &config.MockCollection{
			Mocks: []*mock.Mock{
				{
					OperationID: "ListUsers",
					Type:        mock.TypeHTTP,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/users"},
					},
				},
			},
			Tables: []*config.TableConfig{
				{Name: "users", Response: tableTransform},
			},
			Extend: []*config.ExtendBinding{
				{Mock: "ListUsers", Table: "users", Action: "list"}, // no Response
			},
		}

		if err := processTablesAndExtend(col); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sb := col.Mocks[0].HTTP.StatefulBinding
		if sb == nil || sb.Response == nil {
			t.Fatal("StatefulBinding.Response should be set from table default")
		}
		transform, ok := sb.Response.Transform.(*config.ResponseTransform)
		if !ok {
			t.Fatalf("Response.Transform type = %T, want *config.ResponseTransform", sb.Response.Transform)
		}
		if transform.List == nil || transform.List.DataField != "table_wrapper" {
			t.Error("table response transform should be used as default")
		}
	})

	t.Run("response transform: nil when neither binding nor table has one", func(t *testing.T) {
		col := &config.MockCollection{
			Mocks: []*mock.Mock{
				{
					OperationID: "ListUsers",
					Type:        mock.TypeHTTP,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/users"},
					},
				},
			},
			Tables: []*config.TableConfig{
				{Name: "users"}, // no Response
			},
			Extend: []*config.ExtendBinding{
				{Mock: "ListUsers", Table: "users", Action: "list"}, // no Response
			},
		}

		if err := processTablesAndExtend(col); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sb := col.Mocks[0].HTTP.StatefulBinding
		if sb == nil {
			t.Fatal("StatefulBinding should be set")
		}
		if sb.Response != nil {
			t.Error("StatefulBinding.Response should be nil when no transforms configured")
		}
	})

	// --------------- Complex / integration-ish scenarios ---------------

	t.Run("tables without extend creates resources but no bindings", func(t *testing.T) {
		col := &config.MockCollection{
			Mocks: []*mock.Mock{
				{
					OperationID: "GetUsers",
					Type:        mock.TypeHTTP,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/users"},
					},
				},
			},
			Tables: []*config.TableConfig{
				{Name: "users", IDField: "id"},
			},
			// No Extend
		}

		if err := processTablesAndExtend(col); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(col.StatefulResources) != 1 {
			t.Fatalf("expected 1 resource, got %d", len(col.StatefulResources))
		}
		if col.Mocks[0].HTTP.StatefulBinding != nil {
			t.Error("mock should not have a StatefulBinding when no extend is configured")
		}
	})

	t.Run("full lifecycle: two tables with multiple extend bindings", func(t *testing.T) {
		col := &config.MockCollection{
			Mocks: []*mock.Mock{
				{
					OperationID: "ListUsers",
					Type:        mock.TypeHTTP,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/api/users"},
					},
				},
				{
					OperationID: "CreateUser",
					Type:        mock.TypeHTTP,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{Method: "POST", Path: "/api/users"},
					},
				},
				{
					OperationID: "GetUser",
					Type:        mock.TypeHTTP,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/api/users/{id}"},
					},
				},
				{
					OperationID: "ListOrders",
					Type:        mock.TypeHTTP,
					HTTP: &mock.HTTPSpec{
						Matcher: &mock.HTTPMatcher{Method: "GET", Path: "/api/orders"},
					},
				},
			},
			Tables: []*config.TableConfig{
				{
					Name:    "users",
					IDField: "id",
					SeedData: []map[string]interface{}{
						{"id": "u1", "name": "Alice"},
					},
				},
				{
					Name:    "orders",
					IDField: "order_id",
				},
			},
			Extend: []*config.ExtendBinding{
				{Mock: "ListUsers", Table: "users", Action: "list"},
				{Mock: "CreateUser", Table: "users", Action: "create"},
				{Mock: "GetUser", Table: "users", Action: "get"},
				{Mock: "ListOrders", Table: "orders", Action: "list"},
			},
		}

		if err := processTablesAndExtend(col); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify resources
		if len(col.StatefulResources) != 2 {
			t.Fatalf("expected 2 resources, got %d", len(col.StatefulResources))
		}

		// Verify all bindings were set
		bindings := map[string]string{
			"ListUsers":  "list",
			"CreateUser": "create",
			"GetUser":    "get",
			"ListOrders": "list",
		}
		for _, m := range col.Mocks {
			wantAction, ok := bindings[m.OperationID]
			if !ok {
				continue
			}
			if m.HTTP.StatefulBinding == nil {
				t.Errorf("%s: StatefulBinding should be set", m.OperationID)
				continue
			}
			if m.HTTP.StatefulBinding.Action != wantAction {
				t.Errorf("%s: Action = %q, want %q", m.OperationID, m.HTTP.StatefulBinding.Action, wantAction)
			}
		}

		// Verify table→resource mapping for users
		usersRes := col.StatefulResources[0]
		if usersRes.Name != "users" {
			t.Errorf("first resource Name = %q, want %q", usersRes.Name, "users")
		}
		if len(usersRes.SeedData) != 1 {
			t.Errorf("users seed data length = %d, want 1", len(usersRes.SeedData))
		}
	})
}
