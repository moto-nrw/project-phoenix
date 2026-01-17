package active

import (
	"context"
	"time"

	activeDomain "github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
)

// DeleteExpiredVisits deletes visits older than retention days for a specific student
func (r *VisitRepository) DeleteExpiredVisits(ctx context.Context, studentID int64, retentionDays int) (int64, error) {
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

	result, err := r.db.NewDelete().
		Model((*activeDomain.Visit)(nil)).
		ModelTableExpr(tableExprActiveVisitsAsVisit).
		Where(`"visit".student_id = ?`, studentID).
		Where(`"visit".created_at < ?`, cutoffDate).
		Where(`"visit".exit_time IS NOT NULL`). // Only delete completed visits
		Exec(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete expired visits",
			Err: err,
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "get rows affected",
			Err: err,
		}
	}

	return rowsAffected, nil
}

// DeleteVisitsBeforeDate deletes visits created before a specific date for a student
func (r *VisitRepository) DeleteVisitsBeforeDate(ctx context.Context, studentID int64, beforeDate time.Time) (int64, error) {
	result, err := r.db.NewDelete().
		Model((*activeDomain.Visit)(nil)).
		ModelTableExpr(tableExprActiveVisitsAsVisit).
		Where(`"visit".student_id = ?`, studentID).
		Where(`"visit".created_at < ?`, beforeDate).
		Where(`"visit".exit_time IS NOT NULL`). // Only delete completed visits
		Exec(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete visits before date",
			Err: err,
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "get rows affected",
			Err: err,
		}
	}

	return rowsAffected, nil
}

// GetVisitRetentionStats gets statistics about visits that are candidates for deletion
func (r *VisitRepository) GetVisitRetentionStats(ctx context.Context) (map[int64]int, error) {
	type studentVisitCount struct {
		StudentID  int64 `bun:"student_id"`
		VisitCount int   `bun:"visit_count"`
	}

	var results []studentVisitCount
	err := r.db.NewSelect().
		TableExpr("active.visits AS v").
		ColumnExpr("v.student_id").
		ColumnExpr("COUNT(*) AS visit_count").
		Join("INNER JOIN users.privacy_consents AS pc ON pc.student_id = v.student_id").
		Where("v.exit_time IS NOT NULL").
		Where("v.created_at < NOW() - make_interval(days => pc.data_retention_days)").
		Group("v.student_id").
		Scan(ctx, &results)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get visit retention stats",
			Err: err,
		}
	}

	// Convert to map
	stats := make(map[int64]int)
	for _, result := range results {
		stats[result.StudentID] = result.VisitCount
	}

	return stats, nil
}

// CountExpiredVisits counts visits that are older than retention period for all students
func (r *VisitRepository) CountExpiredVisits(ctx context.Context) (int64, error) {
	count, err := r.db.NewSelect().
		TableExpr("active.visits AS v").
		Join("INNER JOIN users.privacy_consents AS pc ON pc.student_id = v.student_id").
		Where("v.exit_time IS NOT NULL").
		Where("v.created_at < NOW() - make_interval(days => pc.data_retention_days)").
		Count(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count expired visits",
			Err: err,
		}
	}

	return int64(count), nil
}

// GetOldestExpiredVisit returns the timestamp of the oldest visit that is past retention
func (r *VisitRepository) GetOldestExpiredVisit(ctx context.Context) (*time.Time, error) {
	var result struct {
		CreatedAt time.Time `bun:"created_at"`
	}

	err := r.db.NewRaw(`
		SELECT MIN(v.created_at) as created_at
		FROM active.visits v
		INNER JOIN users.privacy_consents pc ON pc.student_id = v.student_id
		WHERE v.exit_time IS NOT NULL
			AND v.created_at < NOW() - (pc.data_retention_days || ' days')::INTERVAL
	`).Scan(ctx, &result)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get oldest expired visit",
			Err: err,
		}
	}

	if result.CreatedAt.IsZero() {
		return nil, nil
	}

	return &result.CreatedAt, nil
}

// GetExpiredVisitsByMonth returns counts of expired visits grouped by month
func (r *VisitRepository) GetExpiredVisitsByMonth(ctx context.Context) (map[string]int64, error) {
	var monthlyStats []struct {
		Month string `bun:"month"`
		Count int64  `bun:"count"`
	}

	err := r.db.NewRaw(`
		SELECT
			TO_CHAR(v.created_at, 'YYYY-MM') as month,
			COUNT(*) as count
		FROM active.visits v
		INNER JOIN users.privacy_consents pc ON pc.student_id = v.student_id
		WHERE v.exit_time IS NOT NULL
			AND v.created_at < NOW() - (pc.data_retention_days || ' days')::INTERVAL
		GROUP BY TO_CHAR(v.created_at, 'YYYY-MM')
		ORDER BY month
	`).Scan(ctx, &monthlyStats)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get expired visits by month",
			Err: err,
		}
	}

	result := make(map[string]int64, len(monthlyStats))
	for _, ms := range monthlyStats {
		result[ms.Month] = ms.Count
	}

	return result, nil
}
