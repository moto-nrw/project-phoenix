package postgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func (r *Store) CountPhaseSchemaReferences(ctx context.Context, schemaIDs []int64) (int, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return 0, err
	}
	if len(schemaIDs) == 0 {
		return 0, nil
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return 0, err
	}
	count, err := db.NewSelect().TableExpr("enrollment.phases").Where("tenant_id = ?", tenantID).Where("form_schema_id IN (?)", bun.List(schemaIDs)).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count phase schema references: %w", err)
	}
	return count, nil
}

func (r *Store) RepointPhaseSchemas(ctx context.Context, fromIDs []int64, toID int64) (int64, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return 0, err
	}
	if len(fromIDs) == 0 {
		return 0, nil
	}
	// A foreign-key check alone does not enforce tenant ownership of the target.
	if _, err := r.Schema(ctx, toID); err != nil {
		return 0, fmt.Errorf("load phase schema target: %w", err)
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return 0, err
	}
	result, err := db.NewUpdate().TableExpr("enrollment.phases").Set("form_schema_id = ?", toID).Set("updated_at = NOW()").Where("tenant_id = ?", tenantID).Where("form_schema_id IN (?)", bun.List(fromIDs)).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to repoint phases to schema %d: %w", toID, err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read repointed phase count: %w", err)
	}
	return count, nil
}

func (r *Store) PhaseCountsByCalendarPeriod(ctx context.Context) (map[int64]int, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		CalendarPeriodID int64 `bun:"calendar_period_id"`
		Count            int   `bun:"count"`
	}
	err = db.NewRaw(`SELECT calendar_period_id, COUNT(*)::int AS count
		FROM enrollment.phases
		WHERE tenant_id = ? AND calendar_period_id IS NOT NULL
		GROUP BY calendar_period_id`, tenantID).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("failed to count enrollment phase calendar references: %w", err)
	}
	counts := make(map[int64]int, len(rows))
	for _, row := range rows {
		counts[row.CalendarPeriodID] = row.Count
	}
	return counts, nil
}
