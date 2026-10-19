package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// StudentCompanionEdge is one stored "läuft mit" link as the composition root
// hands it over. Care Plan owns the row; this is the shape the directory needs
// to reconcile a narrowed departure plan against it.
type StudentCompanionEdge struct {
	ID                 int64
	StudentID          int64
	CompanionStudentID int64
	Weekday            int
}

// StudentCompanions is the Care Plan seam the directory's write path needs.
// The composition root binds modules/careplan behind it; the directory never
// reaches users.student_companions itself.
type StudentCompanions interface {
	ListCompanionEdgesForStudent(ctx context.Context, studentID int64) ([]StudentCompanionEdge, error)
	CompanionDaysCoveredExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]map[string]bool, error)
	DeleteCompanionEdges(ctx context.Context, edgeIDs []int64) error
}

type StudentOwners interface {
	LockClassWrites(context.Context, bool) error
	Enroll(context.Context, peopledirectory.StudentRecord) (int64, error)
	Renew(context.Context, peopledirectory.StudentRecord) (int64, error)
	SaveCare(context.Context, int64, peopledirectory.StudentRecord, peopledirectory.StudentPlan, *string, bool) error
}

// StudentNoteDeletionAudit is the Audit Platform seam behind note removals.
// The composition root binds it so the note's soft-delete and its append-only
// deletion evidence commit or roll back in the same tenant transaction.
type StudentNoteDeletionAudit interface {
	RecordStudentNoteDeletion(context.Context, int64, int64, int64) error
}
