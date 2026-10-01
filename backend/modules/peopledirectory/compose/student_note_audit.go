package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
)

// StudentNoteDeletionAudit is the Audit Platform seam behind note removals.
// The composition root binds it so the note's soft-delete and its append-only
// deletion evidence commit or roll back in the same tenant transaction.
type StudentNoteDeletionAudit interface {
	RecordStudentNoteDeletion(context.Context, int64, int64, int64) error
	RecordLegacyStudentNoteDeletion(context.Context, int64, int64) error
}

type studentNoteDeletionAudit struct{ audit StudentNoteDeletionAudit }

func (a studentNoteDeletionAudit) RecordStudentNoteDeletion(
	ctx context.Context, studentID, noteID, actorAccountID int64,
) error {
	return a.audit.RecordStudentNoteDeletion(ctx, studentID, noteID, actorAccountID)
}

func (a studentNoteDeletionAudit) RecordLegacyStudentNoteDeletion(
	ctx context.Context, studentID, noteID int64,
) error {
	return a.audit.RecordLegacyStudentNoteDeletion(ctx, studentID, noteID)
}

var _ ports.StudentNoteDeletionAudit = studentNoteDeletionAudit{}
