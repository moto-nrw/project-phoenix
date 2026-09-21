package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

type classArrivalPlanRow struct {
	ID           int64             `bun:"id"`
	TenantID     int64             `bun:"tenant_id"`
	CreatedAt    time.Time         `bun:"created_at"`
	UpdatedAt    time.Time         `bun:"updated_at"`
	SchoolClass  string            `bun:"school_class"`
	ArrivalTimes map[string]string `bun:"arrival_times,type:jsonb"`
	UpdatedBy    *int64            `bun:"updated_by"`
}

func classArrivalPlanValue(row classArrivalPlanRow) domain.ClassArrivalPlan {
	return domain.ClassArrivalPlan{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		SchoolClass: row.SchoolClass, ArrivalTimes: row.ArrivalTimes, UpdatedBy: row.UpdatedBy}
}

func (s *Store) SaveClassArrivalPlan(ctx context.Context, input domain.ClassArrivalPlan) (domain.ClassArrivalPlan, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.ClassArrivalPlan{}, domain.OperationStats{}, err
	}
	row := classArrivalPlanRow{TenantID: tenantID, SchoolClass: input.SchoolClass, ArrivalTimes: input.ArrivalTimes, UpdatedBy: input.UpdatedBy}
	if row.ArrivalTimes == nil {
		row.ArrivalTimes = map[string]string{}
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`education.class_arrival_times`).
		Column("tenant_id", "school_class", "arrival_times", "updated_by").
		On("CONFLICT (tenant_id, (LOWER(BTRIM(school_class)))) DO UPDATE").
		Set("arrival_times = EXCLUDED.arrival_times").Set("school_class = EXCLUDED.school_class").
		Set("updated_by = EXCLUDED.updated_by").Set("updated_at = NOW()").Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.ClassArrivalPlan{}, stats, classifyWriteError("upsert class arrival plan", err, &stats)
	}
	stats.Rows = 1
	return classArrivalPlanValue(row), stats, nil
}

func (s *Store) LockClassArrivalPlan(ctx context.Context, class string) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	// Keep the retained repository's key while both callers can update a class.
	key := fmt.Sprintf("class-arrival:%d:%s", tenantID, class)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewRaw("SELECT pg_advisory_xact_lock(hashtext(?))", key).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("lock class arrival plan: %w", err)
	}
	return stats, nil
}
