package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

// MarkNextRetiredStaffErased marks the next tombstone of the school in
// context erased: the one with the lowest ID above afterID whose staff
// member left before the cutoff and is not erased yet. It locks the row and
// skips one a concurrent run holds. found is false when none is due.
func (s *Store) MarkNextRetiredStaffErased(ctx context.Context, before time.Time, afterID int64) (schoolmembership.RetiredStaffErasure, bool, domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return schoolmembership.RetiredStaffErasure{}, false, domain.OperationStats{}, err
	}
	due := withTenant(db.NewSelect().
		TableExpr(`users.staff_school_memberships AS "due"`).
		Column("due.id").
		Where(`"due".id > ?`, afterID).
		Where(`"due".deleted_at IS NOT NULL`).
		Where(`"due".deleted_at < ?`, before).
		Where(`"due".personal_data_erased_at IS NULL`).
		OrderExpr(`"due".id ASC`).Limit(1).For("UPDATE SKIP LOCKED"), "due", scope)
	var rows []schoolmembership.RetiredStaffErasure
	query := withTenant(db.NewUpdate().Model((*staffRow)(nil)).
		ModelTableExpr(`users.staff_school_memberships AS "staff"`).
		Set(`personal_data_erased_at = NOW()`).
		Set(`updated_at = NOW()`).
		Where(`"staff".id = (?)`, due).
		Returning(`"staff".id AS staff_id, "staff".person_id`), "staff", scope)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = query.Exec(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return schoolmembership.RetiredStaffErasure{}, false, stats, fmt.Errorf("school membership postgres: mark next retired staff erased: %w", err)
	}
	if len(rows) == 0 {
		return schoolmembership.RetiredStaffErasure{}, false, stats, nil
	}
	stats.Rows = 1
	return rows[0], true, stats, nil
}

// DeleteGroupAssignmentsByStaff explicitly deletes the group assignments of
// the staff member's teacher records before the teachers themselves go. This
// keeps the retention deletion log's row count accurate instead of relying on
// the foreign-key cascade.
func (s *Store) DeleteGroupAssignmentsByStaff(ctx context.Context, staffID int64) (domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	teachers := withTenant(db.NewSelect().Model((*teacherRow)(nil)).
		ModelTableExpr(`users.teachers AS "teacher"`).
		ColumnExpr(`"teacher".id`).
		Where(`"teacher".staff_id = ?`, staffID), "teacher", scope)
	query := withTenant(db.NewDelete().Model((*groupAssignmentRow)(nil)).
		ModelTableExpr(`education.group_teacher AS "group_assignment"`).
		Where(`"group_assignment".teacher_id IN (?)`, teachers), "group_assignment", scope)
	return execDelete(ctx, query, "delete group assignments by staff")
}

// DeleteTeachersByStaff hard-deletes the teacher records of a staff member,
// including the one the retirement soft-deleted.
func (s *Store) DeleteTeachersByStaff(ctx context.Context, staffID int64) (domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*teacherRow)(nil)).
		ModelTableExpr(`users.teachers AS "teacher"`).
		Where(`"teacher".staff_id = ?`, staffID), "teacher", scope)
	return execDelete(ctx, query, "delete teachers by staff")
}

// DeleteExternalCaregiverRecordsByStaff deletes the external caregiver record of a staff member.
func (s *Store) DeleteExternalCaregiverRecordsByStaff(ctx context.Context, staffID int64) (domain.OperationStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*externalCaregiverRecordRow)(nil)).
		ModelTableExpr(`users.external_caregivers AS "external_caregiver"`).
		Where(`"external_caregiver".staff_id = ?`, staffID), "external_caregiver", scope)
	return execDelete(ctx, query, "delete external caregiver records by staff")
}
