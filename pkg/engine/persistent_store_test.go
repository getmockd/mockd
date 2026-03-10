package engine

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/store"
)

// ---------------------------------------------------------------------------
// fakeMockStore — test double implementing store.MockStore
// ---------------------------------------------------------------------------

type fakeMockStore struct {
	mu      sync.Mutex
	mocks   map[string]*mock.Mock
	listErr error // inject error on List
	countN  int   // override Count return
	countOK bool  // when true, use countN instead of len(mocks)

	// call counters for verification
	creates atomic.Int64
	updates atomic.Int64
}

func newFakeMockStore() *fakeMockStore {
	return &fakeMockStore{mocks: make(map[string]*mock.Mock)}
}

func (f *fakeMockStore) Get(_ context.Context, id string) (*mock.Mock, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m, ok := f.mocks[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return m, nil
}

func (f *fakeMockStore) Create(_ context.Context, m *mock.Mock) error {
	f.creates.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.mocks[m.ID]; ok {
		return store.ErrAlreadyExists
	}
	f.mocks[m.ID] = m
	return nil
}

func (f *fakeMockStore) Update(_ context.Context, m *mock.Mock) error {
	f.updates.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mocks[m.ID] = m
	return nil
}

func (f *fakeMockStore) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.mocks[id]; !ok {
		return store.ErrNotFound
	}
	delete(f.mocks, id)
	return nil
}

func (f *fakeMockStore) DeleteByType(_ context.Context, _ mock.Type) error { return nil }

func (f *fakeMockStore) DeleteAll(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mocks = make(map[string]*mock.Mock)
	return nil
}

func (f *fakeMockStore) List(_ context.Context, filter *store.MockFilter) ([]*mock.Mock, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []*mock.Mock
	for _, m := range f.mocks {
		if filter != nil && filter.Type != "" && m.Type != filter.Type {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

func (f *fakeMockStore) Count(_ context.Context, _ mock.Type) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.countOK {
		return f.countN, nil
	}
	return len(f.mocks), nil
}

func (f *fakeMockStore) BulkCreate(_ context.Context, mocks []*mock.Mock) error {
	for _, m := range mocks {
		if err := f.Create(context.Background(), m); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeMockStore) BulkUpdate(_ context.Context, mocks []*mock.Mock) error {
	for _, m := range mocks {
		if err := f.Update(context.Background(), m); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestPersistentMockStore_Set(t *testing.T) {
	t.Parallel()

	t.Run("creates when item does not exist", func(t *testing.T) {
		t.Parallel()
		fs := newFakeMockStore()
		ps := NewPersistentMockStore(fs)

		m := &mock.Mock{ID: "http_001", Type: mock.TypeHTTP, Name: "test"}
		err := ps.Set(m)
		require.NoError(t, err)

		// Should have been stored via Create
		assert.Equal(t, int64(1), fs.creates.Load())
		assert.Equal(t, int64(0), fs.updates.Load())

		// Verify it's actually in the backing store
		got := ps.Get("http_001")
		require.NotNil(t, got)
		assert.Equal(t, "test", got.Name)
	})

	t.Run("updates when item already exists", func(t *testing.T) {
		t.Parallel()
		fs := newFakeMockStore()
		ps := NewPersistentMockStore(fs)

		original := &mock.Mock{ID: "http_002", Type: mock.TypeHTTP, Name: "original"}
		err := ps.Set(original)
		require.NoError(t, err)

		updated := &mock.Mock{ID: "http_002", Type: mock.TypeHTTP, Name: "updated"}
		err = ps.Set(updated)
		require.NoError(t, err)

		// First Set → Create, second Set → Update
		assert.Equal(t, int64(1), fs.creates.Load())
		assert.Equal(t, int64(1), fs.updates.Load())

		got := ps.Get("http_002")
		require.NotNil(t, got)
		assert.Equal(t, "updated", got.Name)
	})

	t.Run("concurrent sets for same ID are serialised by mutex", func(t *testing.T) {
		t.Parallel()
		fs := newFakeMockStore()
		ps := NewPersistentMockStore(fs)

		const n = 50
		var wg sync.WaitGroup
		wg.Add(n)
		for i := 0; i < n; i++ {
			go func() {
				defer wg.Done()
				_ = ps.Set(&mock.Mock{ID: "race_id", Type: mock.TypeHTTP})
			}()
		}
		wg.Wait()

		// Exactly one Create should have occurred; the rest should be Updates.
		assert.Equal(t, int64(1), fs.creates.Load(),
			"expected exactly 1 Create — mutex should serialise the TOCTOU check")
		assert.Equal(t, int64(n-1), fs.updates.Load(),
			"remaining calls should be Updates")
	})
}

func TestPersistentMockStore_Get(t *testing.T) {
	t.Parallel()

	t.Run("returns mock when found", func(t *testing.T) {
		t.Parallel()
		fs := newFakeMockStore()
		ps := NewPersistentMockStore(fs)

		_ = ps.Set(&mock.Mock{ID: "get_1", Type: mock.TypeGraphQL, Name: "gql"})

		got := ps.Get("get_1")
		require.NotNil(t, got)
		assert.Equal(t, mock.TypeGraphQL, got.Type)
		assert.Equal(t, "gql", got.Name)
	})

	t.Run("returns nil when not found", func(t *testing.T) {
		t.Parallel()
		fs := newFakeMockStore()
		ps := NewPersistentMockStore(fs)

		got := ps.Get("nonexistent")
		assert.Nil(t, got)
	})
}

func TestPersistentMockStore_Delete(t *testing.T) {
	t.Parallel()

	t.Run("returns true when mock is deleted", func(t *testing.T) {
		t.Parallel()
		fs := newFakeMockStore()
		ps := NewPersistentMockStore(fs)

		_ = ps.Set(&mock.Mock{ID: "del_1", Type: mock.TypeHTTP})
		ok := ps.Delete("del_1")
		assert.True(t, ok)

		// Confirm it's gone
		assert.Nil(t, ps.Get("del_1"))
	})

	t.Run("returns false when mock does not exist", func(t *testing.T) {
		t.Parallel()
		fs := newFakeMockStore()
		ps := NewPersistentMockStore(fs)

		ok := ps.Delete("no_such_id")
		assert.False(t, ok)
	})
}

func TestPersistentMockStore_List(t *testing.T) {
	t.Parallel()

	t.Run("returns all mocks", func(t *testing.T) {
		t.Parallel()
		fs := newFakeMockStore()
		ps := NewPersistentMockStore(fs)

		_ = ps.Set(&mock.Mock{ID: "l1", Type: mock.TypeHTTP})
		_ = ps.Set(&mock.Mock{ID: "l2", Type: mock.TypeGRPC})

		list := ps.List()
		assert.Len(t, list, 2)
	})

	t.Run("returns nil when store is empty", func(t *testing.T) {
		t.Parallel()
		fs := newFakeMockStore()
		ps := NewPersistentMockStore(fs)

		list := ps.List()
		// Empty slice or nil — both are acceptable for "nothing stored"
		assert.Empty(t, list)
	})
}

func TestPersistentMockStore_ListByType(t *testing.T) {
	t.Parallel()

	t.Run("filters by type", func(t *testing.T) {
		t.Parallel()
		fs := newFakeMockStore()
		ps := NewPersistentMockStore(fs)

		_ = ps.Set(&mock.Mock{ID: "t1", Type: mock.TypeHTTP})
		_ = ps.Set(&mock.Mock{ID: "t2", Type: mock.TypeHTTP})
		_ = ps.Set(&mock.Mock{ID: "t3", Type: mock.TypeGRPC})

		httpMocks := ps.ListByType(mock.TypeHTTP)
		assert.Len(t, httpMocks, 2)
		for _, m := range httpMocks {
			assert.Equal(t, mock.TypeHTTP, m.Type)
		}
	})

	t.Run("returns nil for non-matching type", func(t *testing.T) {
		t.Parallel()
		fs := newFakeMockStore()
		ps := NewPersistentMockStore(fs)

		_ = ps.Set(&mock.Mock{ID: "t4", Type: mock.TypeHTTP})

		result := ps.ListByType(mock.TypeMQTT)
		assert.Empty(t, result)
	})
}

func TestPersistentMockStore_Count(t *testing.T) {
	t.Parallel()

	t.Run("delegates to backing store", func(t *testing.T) {
		t.Parallel()
		fs := newFakeMockStore()
		ps := NewPersistentMockStore(fs)

		_ = ps.Set(&mock.Mock{ID: "c1", Type: mock.TypeHTTP})
		_ = ps.Set(&mock.Mock{ID: "c2", Type: mock.TypeHTTP})

		assert.Equal(t, 2, ps.Count())
	})

	t.Run("returns zero when empty", func(t *testing.T) {
		t.Parallel()
		fs := newFakeMockStore()
		ps := NewPersistentMockStore(fs)

		assert.Equal(t, 0, ps.Count())
	})
}

func TestPersistentMockStore_Clear(t *testing.T) {
	t.Parallel()

	t.Run("removes all mocks", func(t *testing.T) {
		t.Parallel()
		fs := newFakeMockStore()
		ps := NewPersistentMockStore(fs)

		_ = ps.Set(&mock.Mock{ID: "cl1", Type: mock.TypeHTTP})
		_ = ps.Set(&mock.Mock{ID: "cl2", Type: mock.TypeGRPC})
		require.Equal(t, 2, ps.Count())

		ps.Clear()
		assert.Equal(t, 0, ps.Count())
		assert.Empty(t, ps.List())
	})
}

func TestPersistentMockStore_Exists(t *testing.T) {
	t.Parallel()

	t.Run("returns true when mock exists", func(t *testing.T) {
		t.Parallel()
		fs := newFakeMockStore()
		ps := NewPersistentMockStore(fs)

		_ = ps.Set(&mock.Mock{ID: "ex_1", Type: mock.TypeHTTP})
		assert.True(t, ps.Exists("ex_1"))
	})

	t.Run("returns false when mock does not exist", func(t *testing.T) {
		t.Parallel()
		fs := newFakeMockStore()
		ps := NewPersistentMockStore(fs)

		assert.False(t, ps.Exists("no_exist"))
	})
}
