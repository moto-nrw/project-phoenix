package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// FindExternalIDHolder returns the child that holds key and its lifecycle
// status, read through the directory projection (#1446).
func (s *StudentStore) FindExternalIDHolder(ctx context.Context, key string) (domain.ExternalIDHolder, bool, domain.OperationStats, error) {
	holder, found, stats, err := s.reads.FindExternalIDHolder(ctx, key)
	return domain.ExternalIDHolder(holder), found, domain.OperationStats(stats), projectionError(err)
}

// SetExternalID stores the key another system knows the child by; nil clears
// it. It is the column's only writer: full-row writes leave it alone, so
// editing a child never drops the key. It reports false when the tenant has
// no such row.
func (s *StudentStore) SetExternalID(ctx context.Context, studentID int64, key *string) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(tenantID); err != nil {
		return false, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewUpdate().
		TableExpr(`users.student_profiles AS "student"`).
		Set(`external_id = ?`, key).
		Set(`updated_at = NOW()`).
		Where(`"student".id = ?`, studentID).
		Where(`"student".tenant_id = ?`, tenantID).
		Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return false, stats, fmt.Errorf("people directory postgres: set student external id: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, stats, fmt.Errorf("people directory postgres: set student external id: %w", err)
	}
	stats.Rows = affected
	return affected > 0, stats, nil
}
