package parentstore

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentpostgres"
	"github.com/uptrace/bun"
)

// StudentConversations is the deletion seam a permanent child deletion uses:
// Communication answers how many conversation rows the child has and locks its
// threads, so the deletion preview never joins another owner's tables.
type StudentConversations struct {
	store *parentpostgres.StudentConversationStore
}

// NewStudentConversations builds the deletion seam.
func NewStudentConversations(db *bun.DB) *StudentConversations {
	return &StudentConversations{store: parentpostgres.NewStudentConversationStore(postgresDatabase(db))}
}

// CountStudentConversationRecords returns the child's thread, message and read
// cursor rows as one number for the deletion preview.
func (c *StudentConversations) CountStudentConversationRecords(ctx context.Context, studentID int64) (int, error) {
	return c.store.CountRecords(ctx, studentID)
}

// LockStudentMessageThreads holds the child's threads until the deletion
// commits or rolls back, so no message or cursor can be appended behind a
// rechecked preview.
func (c *StudentConversations) LockStudentMessageThreads(ctx context.Context, studentID int64) error {
	return c.store.LockThreads(ctx, studentID)
}
