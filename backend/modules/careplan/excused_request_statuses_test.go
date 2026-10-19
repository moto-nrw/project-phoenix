package careplan

import (
	"slices"
	"testing"
)

func TestPendingExcusedRequestStatuses(t *testing.T) {
	t.Parallel()
	if got := PendingExcusedRequestStatuses(); !slices.Equal(got, []string{ExcusedRequestStatusPending}) {
		t.Errorf("PendingExcusedRequestStatuses() = %v", got)
	}
}
