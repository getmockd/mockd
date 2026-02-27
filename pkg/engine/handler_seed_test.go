package engine

import (
	"net/http/httptest"
	"testing"

	"github.com/getmockd/mockd/pkg/mock"
)

func TestResolveSeed(t *testing.T) {
	int64Ptr := func(v int64) *int64 { return &v }

	t.Run("query parameter takes priority", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/test?_mockd_seed=42", nil)
		r.Header.Set("X-Mockd-Seed", "99")
		resp := &mock.HTTPResponse{Seed: int64Ptr(123)}

		seed, ok := resolveSeed(r, resp)
		if !ok {
			t.Fatal("expected seed to be found")
		}
		if seed != 42 {
			t.Errorf("seed = %d, want 42 (query param)", seed)
		}
	})

	t.Run("header takes priority over config", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/test", nil)
		r.Header.Set("X-Mockd-Seed", "99")
		resp := &mock.HTTPResponse{Seed: int64Ptr(123)}

		seed, ok := resolveSeed(r, resp)
		if !ok {
			t.Fatal("expected seed to be found")
		}
		if seed != 99 {
			t.Errorf("seed = %d, want 99 (header)", seed)
		}
	})

	t.Run("config seed used when no request seed", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/test", nil)
		resp := &mock.HTTPResponse{Seed: int64Ptr(123)}

		seed, ok := resolveSeed(r, resp)
		if !ok {
			t.Fatal("expected seed to be found")
		}
		if seed != 123 {
			t.Errorf("seed = %d, want 123 (config)", seed)
		}
	})

	t.Run("no seed returns false", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/test", nil)
		resp := &mock.HTTPResponse{}

		_, ok := resolveSeed(r, resp)
		if ok {
			t.Error("expected no seed to be found")
		}
	})

	t.Run("invalid query param is skipped", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/test?_mockd_seed=notanumber", nil)
		resp := &mock.HTTPResponse{Seed: int64Ptr(50)}

		seed, ok := resolveSeed(r, resp)
		if !ok {
			t.Fatal("expected seed from config fallback")
		}
		if seed != 50 {
			t.Errorf("seed = %d, want 50 (config fallback)", seed)
		}
	})

	t.Run("invalid header is skipped", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/test", nil)
		r.Header.Set("X-Mockd-Seed", "bad")
		resp := &mock.HTTPResponse{Seed: int64Ptr(75)}

		seed, ok := resolveSeed(r, resp)
		if !ok {
			t.Fatal("expected seed from config fallback")
		}
		if seed != 75 {
			t.Errorf("seed = %d, want 75 (config fallback)", seed)
		}
	})

	t.Run("zero seed is valid", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/test?_mockd_seed=0", nil)
		resp := &mock.HTTPResponse{}

		seed, ok := resolveSeed(r, resp)
		if !ok {
			t.Fatal("expected seed=0 to be found")
		}
		if seed != 0 {
			t.Errorf("seed = %d, want 0", seed)
		}
	})

	t.Run("negative seed is valid", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/test?_mockd_seed=-42", nil)
		resp := &mock.HTTPResponse{}

		seed, ok := resolveSeed(r, resp)
		if !ok {
			t.Fatal("expected negative seed to be found")
		}
		if seed != -42 {
			t.Errorf("seed = %d, want -42", seed)
		}
	})
}
