package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/models/audit"
)

const (
	tableStudentFieldEdits        = "audit.student_field_edits"
	tableStudentFieldEditsAliased = `audit.student_field_edits AS "student_field_edit"`
	whereStudentIDEquals          = `"student_field_edit".student_id = ?`
)

// StudentFieldEditRepository implements audit.StudentFieldEditRepository.
type StudentFieldEditRepository struct {
	runtime Runtime
}

// NewStudentFieldEditRepository creates a new StudentFieldEditRepository.
func NewStudentFieldEditRepository(runtime Runtime) audit.StudentFieldEditRepository {
	return &StudentFieldEditRepository{runtime: requireRuntime(runtime)}
}

// CreateBatch inserts multiple student field edit audit records.
func (r *StudentFieldEditRepository) CreateBatch(ctx context.Context, edits []*audit.StudentFieldEdit) error {
	if len(edits) == 0 {
		return nil
	}

	appender := NewAppender(r.runtime)
	for _, edit := range edits {
		if edit == nil {
			return fmt.Errorf("edit cannot be nil")
		}
		if err := appender.Append(ctx, edit); err != nil {
			return err
		}
	}

	return nil
}

// GetByStudentID returns all edit records for a student, newest first.
func (r *StudentFieldEditRepository) GetByStudentID(ctx context.Context, studentID int64) ([]*audit.StudentFieldEdit, error) {
	var edits []*audit.StudentFieldEdit
	err := runtimeDB(ctx, r.runtime).NewSelect().
		Model(&edits).
		ModelTableExpr(tableStudentFieldEditsAliased).
		Where(whereStudentIDEquals, studentID).
		OrderExpr(`"student_field_edit".created_at DESC, "student_field_edit".id DESC`).
		Scan(ctx)
	if err != nil {
		return nil, wrapDatabase("get student field edits", err)
	}

	return edits, nil
}

// CountOlderThanByStudent returns per-student counts of edit rows created
// strictly before cutoff. created_at is TIMESTAMPTZ, so cutoff is bound as an
// instant (precise, no DATE/timestamptz coercion fuzz).
func (r *StudentFieldEditRepository) CountOlderThanByStudent(ctx context.Context, cutoff time.Time) (map[int64]int, error) {
	var rows []struct {
		StudentID int64 `bun:"student_id"`
		Count     int   `bun:"count"`
	}
	err := runtimeDB(ctx, r.runtime).NewSelect().
		ModelTableExpr(tableStudentFieldEditsAliased).
		ColumnExpr(`"student_field_edit".student_id AS student_id`).
		ColumnExpr(`COUNT(*) AS count`).
		Where(`"student_field_edit".created_at < ?`, cutoff).
		GroupExpr(`"student_field_edit".student_id`).
		Scan(ctx, &rows)
	if err != nil {
		return nil, wrapDatabase("count old student field edits", err)
	}

	counts := make(map[int64]int, len(rows))
	for _, row := range rows {
		counts[row.StudentID] = row.Count
	}
	return counts, nil
}

// DeleteOlderThan removes edit rows through the fixed, tenant-bound retention
// capability and returns the number deleted.
func (r *StudentFieldEditRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tenantID := runtimeTenantID(ctx, r.runtime)
	if tenantID <= 0 {
		return 0, wrapDatabase("delete expired student field edits", fmt.Errorf("tenant context is required"))
	}
	var deleted int64
	if err := runtimeDB(ctx, r.runtime).NewRaw(
		`SELECT audit.delete_expired_student_field_edits(?, ?)`, tenantID, cutoff,
	).Scan(ctx, &deleted); err != nil {
		return 0, wrapDatabase("delete expired student field edits", err)
	}
	return deleted, nil
}
