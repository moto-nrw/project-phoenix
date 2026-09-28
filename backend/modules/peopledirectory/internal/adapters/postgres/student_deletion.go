package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// Placeholder identity of an anonymized child person. The values are part of
// the erasure contract: later reads must never resolve the child again.
const (
	anonymizedFirstName = "Gelöscht"
	anonymizedLastName  = "Benutzer"
)

func (s *StudentStore) CountGuardianLinks(ctx context.Context, studentID, personID int64) (int, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(tenantID); err != nil {
		return 0, domain.OperationStats{}, err
	}
	var count int
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`
		SELECT (
			(SELECT COUNT(*) FROM users.student_guardian_relationships WHERE tenant_id = ? AND student_id = ?) +
			(SELECT COUNT(*) FROM users.persons_guardians WHERE tenant_id = ? AND person_id = ?)
		)::int`, tenantID, studentID, tenantID, personID).Scan(ctx, &count)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("people directory postgres: count student guardian links: %w", err)
	}
	stats.Rows = 1
	return count, stats, nil
}

func (s *StudentStore) DeleteLegacyGuardianLinks(ctx context.Context, personID int64) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(tenantID); err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewDelete().TableExpr(`users.persons_guardians AS "person_guardian"`).
		Where(`"person_guardian".tenant_id = ?`, tenantID).
		Where(`"person_guardian".person_id = ?`, personID).
		Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("people directory postgres: delete legacy guardian links: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return 0, stats, fmt.Errorf("people directory postgres: delete legacy guardian links: count rows: %w", err)
	}
	return stats.Rows, stats, nil
}

func (s *StudentStore) Delete(ctx context.Context, id int64) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(tenantID); err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewDelete().TableExpr(`users.student_profiles AS "student"`).
		Where(`"student".tenant_id = ?`, tenantID).
		Where(`"student".id = ?`, id).
		Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("people directory postgres: delete student: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return 0, stats, fmt.Errorf("people directory postgres: delete student: count rows: %w", err)
	}
	return stats.Rows, stats, nil
}

// AnonymizeIfUnchanged is the person half of a permanent child deletion. The
// updated_at predicate closes the race between the previewed name the actor
// confirmed and a concurrent person edit; a moved row leaves zero rows.
func (s *Store) AnonymizeIfUnchanged(ctx context.Context, personID int64, updatedAt time.Time) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return false, domain.OperationStats{}, fmt.Errorf("people directory postgres: tenant is required to anonymize a person")
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := withTenant(db.NewUpdate().Model((*personRow)(nil)).
		ModelTableExpr(`users.persons AS "person"`).
		Set(`first_name = ?`, anonymizedFirstName).
		Set(`last_name = ?`, anonymizedLastName).
		Set(`birthday = NULL`).
		Set(`tag_id = NULL`).
		Set(`account_id = NULL`).
		Set(`deleted_at = NOW()`).
		Where(`"person".id = ?`, personID).
		Where(`"person".updated_at = ?`, updatedAt).
		Where(`"person".deleted_at IS NULL`), tenantID).
		Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return false, stats, fmt.Errorf("people directory postgres: anonymize deleted student person: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return false, stats, fmt.Errorf("people directory postgres: anonymize deleted student person: count rows: %w", err)
	}
	return stats.Rows == 1, stats, nil
}
