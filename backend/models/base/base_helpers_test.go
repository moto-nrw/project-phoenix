package base

import (
	"testing"
	"time"
)

func TestPointerHelpers(t *testing.T) {
	t.Run("StringPtr", func(t *testing.T) {
		s := "test"
		ptr := StringPtr(s)
		if ptr == nil || *ptr != s {
			t.Errorf("StringPtr(%q) = %v, want %q", s, ptr, s)
		}
	})

	t.Run("IntPtr", func(t *testing.T) {
		i := 42
		ptr := IntPtr(i)
		if ptr == nil || *ptr != i {
			t.Errorf("IntPtr(%d) = %v, want %d", i, ptr, i)
		}
	})

	t.Run("Int64Ptr", func(t *testing.T) {
		i := int64(42)
		ptr := Int64Ptr(i)
		if ptr == nil || *ptr != i {
			t.Errorf("Int64Ptr(%d) = %v, want %d", i, ptr, i)
		}
	})

	t.Run("TimePtr", func(t *testing.T) {
		tm := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
		ptr := TimePtr(tm)
		if ptr == nil || !ptr.Equal(tm) {
			t.Errorf("TimePtr(%v) = %v, want %v", tm, ptr, tm)
		}
	})
}
