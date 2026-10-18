package communication

import "testing"

func TestParentMessageCleanupResultDeletedCountsMessagesAndThreads(t *testing.T) {
	t.Parallel()
	if got := (ParentMessageCleanupResult{MessagesDeleted: 5, ThreadsDeleted: 2, RetentionDays: 30}).Deleted(); got != 7 {
		t.Errorf("Deleted() = %d, want 7", got)
	}
}
