package ports

import (
	"context"
	"errors"
)

// ErrStudentNoteDeletionAuditUnavailable keeps a deletion from succeeding
// without the append-only evidence Audit Platform owns.
var ErrStudentNoteDeletionAuditUnavailable = errors.New("people directory: student note deletion audit is not configured")

// StudentNoteDeletionAudit is the consumer-owned port over Audit Platform's
// deletion ledger. The directory owns the note and decides when it is hidden;
// Audit Platform owns audit.data_deletions and records the durable evidence.
type StudentNoteDeletionAudit interface {
	RecordStudentNoteDeletion(context.Context, int64, int64, int64) error
}
