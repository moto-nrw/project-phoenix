package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
	"github.com/uptrace/bun"
)

// Class gates precede row locks, using the established grade-transition key.
func (s *Store) ChangeStudentClass(ctx context.Context, ids []int64, from, to string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	if tenantID <= 0 || tenantID > 0x7fffffff {
		return 0, domain.OperationStats{}, errors.New("school membership: valid tenant is required")
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	{
		result, writeErr := db.NewUpdate().TableExpr(`users.student_school_memberships AS "membership"`).
			Set("school_class = ?", to).
			Set("updated_at = NOW()").
			Where("membership.tenant_id = ?", tenantID).
			Where("membership.student_profile_id IN (?)", bun.List(ids)).
			Where("membership.deleted_at IS NULL").
			Where("membership.status <> 'alumnus'").
			Where("membership.school_class = ?", from).Exec(ctx)
		err = writeErr
		if err == nil {
			stats.Rows, err = result.RowsAffected()
		}
	}
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("school membership: change student class: %w", err)
	}
	return stats.Rows, stats, nil
}

func (s *Store) LockStudentClassWrites(ctx context.Context, exclusive bool) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	if tenantID <= 0 || tenantID > 0x7fffffff {
		return domain.OperationStats{}, errors.New("school membership: valid tenant is required")
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	if exclusive {
		_, err = db.NewRaw("SELECT pg_advisory_xact_lock(?, ?)", int32(0x636c6173), int32(tenantID)).Exec(ctx)
	} else {
		_, err = db.NewRaw("SELECT pg_advisory_xact_lock_shared(?, ?)", int32(0x636c6173), int32(tenantID)).Exec(ctx)
	}
	stats.StatementDuration = time.Since(started)
	return stats, err
}

// StudentClassWriteGateQuery is referenced as a materialized CTE before the
// identity row lock/insert, or before the first membership insert.
func StudentClassWriteGateQuery(db bun.IDB, tenantID int64) (*bun.SelectQuery, error) {
	if tenantID <= 0 || tenantID > 0x7fffffff {
		return nil, errors.New("school membership: valid tenant is required")
	}
	return db.NewSelect().ColumnExpr("pg_advisory_xact_lock_shared(?, ?)", int32(0x636c6173), int32(tenantID)), nil
}
