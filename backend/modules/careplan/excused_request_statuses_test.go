package careplan

import (
	"slices"
	"testing"
)

func TestDecidedExcusedRequestStatuses(t *testing.T) {
	t.Parallel()
	want := []string{ExcusedRequestStatusApproved, ExcusedRequestStatusRejected, ExcusedRequestStatusWithdrawn}
	if got := DecidedExcusedRequestStatuses(); !slices.Equal(got, want) {
		t.Errorf("DecidedExcusedRequestStatuses() = %v, want %v", got, want)
	}
}
