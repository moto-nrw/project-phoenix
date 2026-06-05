package users

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const tableUsersStudentParentNotes = "users.student_parent_notes"

// StudentParentNote is a single free-text note a guardian left for a
// student via the parents portal. Notes are append-only: each submission
// is a new row, never an overwrite, so both parents and staff can read a
// chronological log and surface only the newest few.
type StudentParentNote struct {
	base.Model `bun:"schema:users,table:student_parent_notes"`
	base.TenantModel
	StudentID         int64  `bun:"student_id,notnull" json:"student_id"`
	GuardianAccountID int64  `bun:"guardian_account_id,notnull" json:"guardian_account_id"`
	Body              string `bun:"body,notnull" json:"body"`
}

func (n *StudentParentNote) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableUsersStudentParentNotes)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableUsersStudentParentNotes)
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr(tableUsersStudentParentNotes)
	}
	return nil
}

func (n *StudentParentNote) GetID() any              { return n.ID }
func (n *StudentParentNote) GetCreatedAt() time.Time { return n.CreatedAt }
func (n *StudentParentNote) GetUpdatedAt() time.Time { return n.UpdatedAt }
func (n *StudentParentNote) TableName() string       { return tableUsersStudentParentNotes }

// StudentParentNoteRepository is the data-access contract for parent
// notes. All methods are tenant-scoped and MUST run inside a tenant
// transaction (the parent service resolves the child's tenant first,
// then wraps the call in tenant.WithTenantTx).
type StudentParentNoteRepository interface {
	Create(ctx context.Context, note *StudentParentNote) error
	// ListByStudent returns the student's notes newest-first. A limit of
	// 0 or less returns all notes.
	ListByStudent(ctx context.Context, studentID int64, limit int) ([]*StudentParentNote, error)
	// CountByStudent returns the total number of notes for the student.
	CountByStudent(ctx context.Context, studentID int64) (int, error)
}
