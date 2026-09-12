package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
)

func (s *Store) ListClassArrivalTimes(ctx context.Context, classes []string) (map[string]map[string]string, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []struct {
		SchoolClass  string            `bun:"school_class"`
		ArrivalTimes map[string]string `bun:"arrival_times,type:jsonb"`
	}{}
	query := db.NewSelect().Model(&rows).TableExpr(`education.class_arrival_times AS arrival`).
		ColumnExpr(`LOWER(BTRIM(arrival.school_class)) AS school_class`).
		ColumnExpr(`arrival.arrival_times`).
		Where(`arrival.tenant_id = ?`, tenantID).
		Where(`LOWER(BTRIM(arrival.school_class)) IN (?)`, bun.List(classes)).
		OrderExpr(`LOWER(BTRIM(arrival.school_class)) ASC`)
	stats, err := scanAll(ctx, query, "list class arrival times")
	if err != nil {
		return nil, stats, err
	}
	times := make(map[string]map[string]string, len(rows))
	for _, row := range rows {
		times[row.SchoolClass] = row.ArrivalTimes
	}
	stats.Rows = int64(len(rows))
	return times, stats, nil
}
