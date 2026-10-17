package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

// MarkNextRetiredStaffErased marks the next tombstone of the school in
// context erased: the one with the lowest ID above afterID whose staff
// member left before the cutoff and is not erased yet. It locks the row and
// skips one a concurrent run holds. found is false when none is due.
func (s *Store) MarkNextRetiredStaffErased(ctx context.Context, before time.Time, afterID int64) (domain.RetiredStaffErasure, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.RetiredStaffErasure{}, false, domain.OperationStats{}, err
	}
	due := withTenant(db.NewSelect().
		TableExpr(`users.staff_school_memberships AS "due"`).
		Column("due.id").
		Where(`"due".id > ?`, afterID).
		Where(`"due".deleted_at IS NOT NULL`).
		Where(`"due".deleted_at < ?`, before).
		Where(`"due".personal_data_erased_at IS NULL`).
		OrderExpr(`"due".id ASC`).Limit(1).For("UPDATE SKIP LOCKED"), "due", tenantID)
	var rows []domain.RetiredStaffErasure
	query := withTenant(db.NewUpdate().Model((*staffRow)(nil)).
		ModelTableExpr(`users.staff_school_memberships AS "staff"`).
		Set(`personal_data_erased_at = NOW()`).
		Set(`updated_at = NOW()`).
		Where(`"staff".id = (?)`, due).
		Returning(`"staff".id AS staff_id, "staff".person_id`), "staff", tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = query.Exec(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.RetiredStaffErasure{}, false, stats, fmt.Errorf("school membership postgres: mark next retired staff erased: %w", err)
	}
	if len(rows) == 0 {
		return domain.RetiredStaffErasure{}, false, stats, nil
	}
	stats.Rows = 1
	return rows[0], true, stats, nil
}

// DeleteTeachersByStaff hard-deletes the teacher records of a staff member,
// including the one the retirement soft-deleted. Their group assignments go
// with them.
func (s *Store) DeleteTeachersByStaff(ctx context.Context, staffID int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*teacherRow)(nil)).
		ModelTableExpr(`users.teachers AS "teacher"`).
		Where(`"teacher".staff_id = ?`, staffID), "teacher", tenantID)
	return execDelete(ctx, query, "delete teachers by staff")
}

// DeleteGuestsByStaff deletes the guest profile of a staff member.
func (s *Store) DeleteGuestsByStaff(ctx context.Context, staffID int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenant(db.NewDelete().Model((*guestRow)(nil)).
		ModelTableExpr(`users.guests AS "guest"`).
		Where(`"guest".staff_id = ?`, staffID), "guest", tenantID)
	return execDelete(ctx, query, "delete guests by staff")
}
