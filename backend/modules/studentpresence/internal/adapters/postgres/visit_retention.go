package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
)

// retentionDaysExpr is the window a visit's child gets: the shortest accepted
// consent window, otherwise the tenant default bound as the argument.
const retentionDaysExpr = "COALESCE(cw.data_retention_days, ?)"

// expiredVisitsQuery selects the tenant's completed visits past their child's
// window. Children without an accepted consent fall back to defaultDays, so no
// child's visits are kept forever (#3835).
func expiredVisitsQuery(db bun.IDB, tenantID int64, policy studentpresence.VisitRetentionPolicy) *bun.SelectQuery {
	if policy.CompletedBefore != nil {
		return db.NewSelect().TableExpr("active.visits AS v").
			Where("v.tenant_id = ?", tenantID).Where("v.exit_time < ?", *policy.CompletedBefore)
	}
	acceptedWindows := db.NewSelect().Table("users.privacy_consents").
		ColumnExpr("student_id, MIN(data_retention_days) AS data_retention_days").
		Where("tenant_id = ?", tenantID).Where("accepted = TRUE").Group("student_id")
	return db.NewSelect().TableExpr("active.visits AS v").
		Join("LEFT JOIN (?) AS cw ON cw.student_id = v.student_id", acceptedWindows).
		Where("v.tenant_id = ?", tenantID).Where("v.exit_time IS NOT NULL").
		Where("v.created_at < NOW() - make_interval(days => "+retentionDaysExpr+")", policy.DefaultDays)
}

func (s *Store) ListVisitRetentionCounts(ctx context.Context, policy studentpresence.VisitRetentionPolicy) ([]studentpresence.VisitRetentionCount, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	rows := []visitRetentionCountRow{}
	started := time.Now()
	// cw (consent windows) holds one row per student, so grouping by its
	// window keeps one row per student.
	query := expiredVisitsQuery(db, tenantID, policy).ColumnExpr("v.student_id, COUNT(*) AS visit_count").GroupExpr("v.student_id")
	if policy.CompletedBefore == nil {
		query = query.ColumnExpr(retentionDaysExpr+" AS retention_days", policy.DefaultDays).GroupExpr("cw.data_retention_days")
	} else {
		query = query.ColumnExpr("0 AS retention_days")
	}
	err = query.OrderExpr("v.student_id").Scan(ctx, &rows)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	result := make([]studentpresence.VisitRetentionCount, 0, len(rows))
	for _, row := range rows {
		result = append(result, studentpresence.VisitRetentionCount{StudentID: row.StudentID, Count: row.Count, RetentionDays: row.RetentionDays, CompletedBefore: policy.CompletedBefore})
	}
	return result, stats, err
}

func (s *Store) CountExpiredVisits(ctx context.Context, policy studentpresence.VisitRetentionPolicy) (int64, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, ports.Stats{}, err
	}
	started := time.Now()
	count, err := expiredVisitsQuery(db, tenantID, policy).Count(ctx)
	return int64(count), ports.Stats{Queries: 1, Rows: 1, StatementDuration: time.Since(started)}, err
}

func (s *Store) OldestExpiredVisitDate(ctx context.Context, policy studentpresence.VisitRetentionPolicy) (*time.Time, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	var oldest *time.Time
	started := time.Now()
	err = expiredVisitsQuery(db, tenantID, policy).ColumnExpr("MIN(v.created_at)").Scan(ctx, &oldest)
	return oldest, ports.Stats{Queries: 1, Rows: 1, StatementDuration: time.Since(started)}, err
}

func (s *Store) ListExpiredVisitMonths(ctx context.Context, policy studentpresence.VisitRetentionPolicy) ([]studentpresence.VisitMonthCount, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	rows := []visitMonthCountRow{}
	started := time.Now()
	err = expiredVisitsQuery(db, tenantID, policy).ColumnExpr("TO_CHAR(v.created_at, 'YYYY-MM') AS month, COUNT(*) AS count").
		GroupExpr("TO_CHAR(v.created_at, 'YYYY-MM')").Scan(ctx, &rows)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	result := make([]studentpresence.VisitMonthCount, 0, len(rows))
	for _, row := range rows {
		result = append(result, studentpresence.VisitMonthCount(row))
	}
	return result, stats, err
}
