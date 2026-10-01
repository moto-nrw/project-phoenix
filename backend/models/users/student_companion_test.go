package users

import (
	"testing"
)

// TestStudentCompanion_Other checks the far-end lookup from both endpoints and
// from a child that is not part of the edge at all — the "ok" flag is what
// keeps unrelated edges out of a child's companion list.
func TestStudentCompanion_Other(t *testing.T) {
	t.Parallel()

	edge := &StudentCompanion{StudentLowID: 10, StudentHighID: 20, Weekday: 1}

	t.Run("from the low endpoint", func(t *testing.T) {
		other, ok := edge.Other(10)
		if !ok {
			t.Fatal("expected the low endpoint to be part of the edge")
		}
		if other != 20 {
			t.Errorf("Other(10) = %d, want 20", other)
		}
	})

	t.Run("from the high endpoint", func(t *testing.T) {
		other, ok := edge.Other(20)
		if !ok {
			t.Fatal("expected the high endpoint to be part of the edge")
		}
		if other != 10 {
			t.Errorf("Other(20) = %d, want 10", other)
		}
	})

	t.Run("from a non-member", func(t *testing.T) {
		other, ok := edge.Other(30)
		if ok {
			t.Error("a child outside the edge must report ok=false")
		}
		if other != 0 {
			t.Errorf("Other(30) = %d, want 0", other)
		}
	})
}
