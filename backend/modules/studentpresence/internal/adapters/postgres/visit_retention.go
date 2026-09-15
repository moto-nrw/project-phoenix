package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/uptrace/bun"
)

func expiredVisitsQuery(db bun.IDB, tenantID int64) *bun.SelectQuery {
	return db.NewSelect().TableExpr("active.visits AS v").
		Join("INNER JOIN users.privacy_consents AS pc ON pc.student_id = v.student_id AND pc.tenant_id = v.tenant_id").
		Where("v.tenant_id = ?", tenantID).Where("v.exit_time IS NOT NULL").
		Where("v.created_at < NOW() - make_interval(days => pc.data_retention_days)")
}

func (s *Store) ListVisitRetentionCounts(ctx context.Context) ([]ports.VisitRetentionCount, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	rows := []visitRetentionCountRow{}
	started := time.Now()
	err = expiredVisitsQuery(db, tenantID).ColumnExpr("v.student_id, COUNT(*) AS visit_count").Group("v.student_id").Scan(ctx, &rows)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	result := make([]ports.VisitRetentionCount, 0, len(rows))
	for _, row := range rows {
		result = append(result, ports.VisitRetentionCount(row))
	}
	return result, stats, err
}

func (s *Store) CountExpiredVisits(ctx context.Context) (int64, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, ports.Stats{}, err
	}
	started := time.Now()
	count, err := expiredVisitsQuery(db, tenantID).Count(ctx)
	return int64(count), ports.Stats{Queries: 1, Rows: 1, StatementDuration: time.Since(started)}, err
}

func (s *Store) OldestExpiredVisitDate(ctx context.Context) (*time.Time, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	var oldest *time.Time
	started := time.Now()
	err = expiredVisitsQuery(db, tenantID).ColumnExpr("MIN(v.created_at)").Scan(ctx, &oldest)
	return oldest, ports.Stats{Queries: 1, Rows: 1, StatementDuration: time.Since(started)}, err
}

func (s *Store) ListExpiredVisitMonths(ctx context.Context) ([]ports.VisitMonthCount, ports.Stats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, ports.Stats{}, err
	}
	rows := []visitMonthCountRow{}
	started := time.Now()
	err = expiredVisitsQuery(db, tenantID).ColumnExpr("TO_CHAR(v.created_at, 'YYYY-MM') AS month, COUNT(*) AS count").
		GroupExpr("TO_CHAR(v.created_at, 'YYYY-MM')").Scan(ctx, &rows)
	stats := ports.Stats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	result := make([]ports.VisitMonthCount, 0, len(rows))
	for _, row := range rows {
		result = append(result, ports.VisitMonthCount(row))
	}
	return result, stats, err
}
