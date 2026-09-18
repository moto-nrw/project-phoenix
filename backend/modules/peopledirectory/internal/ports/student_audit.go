package ports

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// ErrStudentAuditUnavailable reports a directory composed without the audit
// seam. The change history then refuses rather than silently dropping rows.
var ErrStudentAuditUnavailable = errors.New("people directory: student change history is not configured")

// StudentFieldAuditLog is the consumer-owned port over the change-history
// trail. Audit Platform owns audit.student_field_edits and its retention; the
// directory only decides which of its fields changed and how they read.
type StudentFieldAuditLog interface {
	Append(context.Context, []domain.StudentFieldEdit) error
	ListByStudent(context.Context, int64) ([]domain.StudentFieldEdit, error)
}
