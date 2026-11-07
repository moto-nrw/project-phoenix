package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// SetChildWeeklyPlanOn plans the day an approved re-enrollment's weekly plan
// applies; nil clears it (#6043).
func (r *Store) SetChildWeeklyPlanOn(ctx context.Context, requestChildID int64, on *enrollment.Date) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	res, err := db.NewUpdate().TableExpr("enrollment.request_children").
		Set("weekly_plan_on = ?", on).Set("updated_at = ?", time.Now()).
		Where("tenant_id = ? AND id = ?", tenantID, requestChildID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update request child weekly plan: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("request child %d not found", requestChildID)
	}
	return nil
}

// ChildWeeklyPlanOn returns the day one request child's planned weekly plan
// applies, or nil when none is planned.
func (r *Store) ChildWeeklyPlanOn(ctx context.Context, requestChildID int64) (*enrollment.Date, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var row requestChildRow
	err = db.NewSelect().Model(&row).Column("id", "weekly_plan_on").
		Where("request_child.tenant_id = ? AND request_child.id = ?", tenantID, requestChildID).
		Where("request_child.weekly_plan_on IS NOT NULL").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read request child weekly plan: %w", err)
	}
	return row.WeeklyPlanOn, nil
}

// DueWeeklyPlans lists the planned weekly plans due on or before asOf,
// oldest first.
func (r *Store) DueWeeklyPlans(ctx context.Context, asOf enrollment.Date) ([]enrollment.DueWeeklyPlan, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []requestChildRow
	err = db.NewSelect().Model(&rows).Column("id", "weekly_plan_on").
		Where("request_child.tenant_id = ?", tenantID).
		Where("request_child.weekly_plan_on IS NOT NULL AND request_child.weekly_plan_on <= ?", asOf).
		OrderExpr("request_child.weekly_plan_on, request_child.id").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list due weekly plans: %w", err)
	}
	due := make([]enrollment.DueWeeklyPlan, 0, len(rows))
	for i := range rows {
		due = append(due, enrollment.DueWeeklyPlan{RequestChildID: rows[i].ID, On: *rows[i].WeeklyPlanOn})
	}
	return due, nil
}
