package recording

import (
	"testing"
	"time"
)

func TestUpdateTimestampRange(t *testing.T) {
	t.Run("nil initial pointers", func(t *testing.T) {
		var oldest, newest *time.Time
		ts := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

		UpdateTimestampRange(&oldest, &newest, ts)

		if oldest == nil || newest == nil {
			t.Fatal("expected both pointers to be set")
		}
		if !oldest.Equal(ts) {
			t.Errorf("oldest = %v, want %v", *oldest, ts)
		}
		if !newest.Equal(ts) {
			t.Errorf("newest = %v, want %v", *newest, ts)
		}
	})

	t.Run("older timestamp updates oldest", func(t *testing.T) {
		existing := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		oldest := &existing
		newest := &existing
		earlier := time.Date(2025, 6, 14, 12, 0, 0, 0, time.UTC)

		UpdateTimestampRange(&oldest, &newest, earlier)

		if !oldest.Equal(earlier) {
			t.Errorf("oldest = %v, want %v", *oldest, earlier)
		}
		// newest should remain unchanged
		if !newest.Equal(existing) {
			t.Errorf("newest = %v, want %v", *newest, existing)
		}
	})

	t.Run("newer timestamp updates newest", func(t *testing.T) {
		existing := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		oldest := &existing
		newest := &existing
		later := time.Date(2025, 6, 16, 12, 0, 0, 0, time.UTC)

		UpdateTimestampRange(&oldest, &newest, later)

		// oldest should remain unchanged
		if !oldest.Equal(existing) {
			t.Errorf("oldest = %v, want %v", *oldest, existing)
		}
		if !newest.Equal(later) {
			t.Errorf("newest = %v, want %v", *newest, later)
		}
	})

	t.Run("equal timestamp leaves both unchanged", func(t *testing.T) {
		existing := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		oldest := &existing
		newest := &existing

		UpdateTimestampRange(&oldest, &newest, existing)

		if !oldest.Equal(existing) {
			t.Errorf("oldest = %v, want %v", *oldest, existing)
		}
		if !newest.Equal(existing) {
			t.Errorf("newest = %v, want %v", *newest, existing)
		}
	})

	t.Run("zero-value time", func(t *testing.T) {
		var oldest, newest *time.Time
		zero := time.Time{}

		UpdateTimestampRange(&oldest, &newest, zero)

		if oldest == nil || newest == nil {
			t.Fatal("expected both pointers to be set for zero time")
		}
		if !oldest.Equal(zero) {
			t.Errorf("oldest = %v, want zero time", *oldest)
		}
		if !newest.Equal(zero) {
			t.Errorf("newest = %v, want zero time", *newest)
		}
	})

	t.Run("multiple updates track range correctly", func(t *testing.T) {
		var oldest, newest *time.Time
		t1 := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
		t2 := time.Date(2025, 6, 10, 12, 0, 0, 0, time.UTC)
		t3 := time.Date(2025, 6, 20, 12, 0, 0, 0, time.UTC)
		t4 := time.Date(2025, 6, 13, 12, 0, 0, 0, time.UTC)

		UpdateTimestampRange(&oldest, &newest, t1)
		UpdateTimestampRange(&oldest, &newest, t2)
		UpdateTimestampRange(&oldest, &newest, t3)
		UpdateTimestampRange(&oldest, &newest, t4)

		if !oldest.Equal(t2) {
			t.Errorf("oldest = %v, want %v", *oldest, t2)
		}
		if !newest.Equal(t3) {
			t.Errorf("newest = %v, want %v", *newest, t3)
		}
	})
}
