package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
)

func (s *Store) ListClassArrivalPlans(ctx context.Context, classes []string) ([]domain.ClassArrivalPlan, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []classArrivalPlanRow{}
	query := db.NewSelect().Model(&rows).ModelTableExpr(`education.class_arrival_times AS arrival`).
		ColumnExpr(`arrival.id, arrival.tenant_id, arrival.created_at, arrival.updated_at, arrival.school_class, arrival.arrival_times, arrival.updated_by`).
		Where(`arrival.tenant_id = ?`, tenantID).
		Where(`LOWER(BTRIM(arrival.school_class)) IN (?)`, bun.List(classes)).
		OrderExpr(`LOWER(BTRIM(arrival.school_class)) ASC`)
	stats, err := scanAll(ctx, query, "list class arrival times")
	if err != nil {
		return nil, stats, err
	}
	times := make([]domain.ClassArrivalPlan, len(rows))
	for i, row := range rows {
		times[i] = classArrivalPlanValue(row)
	}
	stats.Rows = int64(len(rows))
	return times, stats, nil
}
