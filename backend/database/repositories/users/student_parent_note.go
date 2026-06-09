package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

const tableExprStudentParentNotesAsNote = `users.student_parent_notes AS "student_parent_note"`

// StudentParentNoteRepository is the tenant-scoped data-access layer for
// parent notes. It embeds the generic base.Repository for Create and
// adds the newest-first listing the parent + staff views need.
type StudentParentNoteRepository struct {
	*base.Repository[*users.StudentParentNote]
}

// NewStudentParentNoteRepository wires a fresh repository.
func NewStudentParentNoteRepository(db *bun.DB) users.StudentParentNoteRepository {
	repo := base.NewRepository[*users.StudentParentNote](db, "users.student_parent_notes", "StudentParentNote")
	repo.TenantScoped = true
	return &StudentParentNoteRepository{Repository: repo}
}

// ListByStudent returns the student's notes newest-first. limit <= 0
// returns every note.
func (r *StudentParentNoteRepository) ListByStudent(ctx context.Context, studentID int64, limit int) ([]*users.StudentParentNote, error) {
	var notes []*users.StudentParentNote
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(&notes).
		ModelTableExpr(tableExprStudentParentNotesAsNote).
		Where(`"student_parent_note".student_id = ?`, studentID).
		OrderExpr(`"student_parent_note".created_at DESC`).
		OrderExpr(`"student_parent_note".id DESC`)

	if where, val, ok := base.TenantWhere(ctx, "student_parent_note"); ok {
		query = query.Where(where, val)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list student parent notes", Err: err}
	}
	return notes, nil
}

// CountByStudent returns the total number of notes for the student.
func (r *StudentParentNoteRepository) CountByStudent(ctx context.Context, studentID int64) (int, error) {
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model((*users.StudentParentNote)(nil)).
		ModelTableExpr(tableExprStudentParentNotesAsNote).
		Where(`"student_parent_note".student_id = ?`, studentID)

	if where, val, ok := base.TenantWhere(ctx, "student_parent_note"); ok {
		query = query.Where(where, val)
	}

	count, err := query.Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "count student parent notes", Err: err}
	}
	return count, nil
}
