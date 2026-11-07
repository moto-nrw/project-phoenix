package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// SetChildWeeklyPlan plans the weekly plan an approved re-enrollment
// applies at its start; nil clears it (#6043).
func (r *Store) SetChildWeeklyPlan(ctx context.Context, requestChildID int64, plan *enrollment.WeeklyPlan) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	var on *enrollment.Date
	replaces := false
	if plan != nil {
		on, replaces = &plan.On, plan.Replace
	}
	res, err := db.NewUpdate().TableExpr("enrollment.request_children").
		Set("weekly_plan_on = ?", on).Set("weekly_plan_replaces = ?", replaces).Set("updated_at = ?", time.Now()).
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

// ChildWeeklyPlan returns the planned weekly plan of one request child, or
// nil when none is planned.
func (r *Store) ChildWeeklyPlan(ctx context.Context, requestChildID int64) (*enrollment.WeeklyPlan, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var row requestChildRow
	err = db.NewSelect().Model(&row).Column("id", "weekly_plan_on", "weekly_plan_replaces").
		Where("request_child.tenant_id = ? AND request_child.id = ?", tenantID, requestChildID).
		Where("request_child.weekly_plan_on IS NOT NULL").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read request child weekly plan: %w", err)
	}
	plan := weeklyPlanFromRow(&row)
	return &plan, nil
}

// DueWeeklyPlans lists the planned weekly plans due on or before asOf,
// oldest first.
func (r *Store) DueWeeklyPlans(ctx context.Context, asOf enrollment.Date) ([]enrollment.WeeklyPlan, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []requestChildRow
	err = db.NewSelect().Model(&rows).Column("id", "weekly_plan_on", "weekly_plan_replaces").
		Where("request_child.tenant_id = ?", tenantID).
		Where("request_child.weekly_plan_on IS NOT NULL AND request_child.weekly_plan_on <= ?", asOf).
		OrderExpr("request_child.weekly_plan_on, request_child.id").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list due weekly plans: %w", err)
	}
	due := make([]enrollment.WeeklyPlan, 0, len(rows))
	for i := range rows {
		due = append(due, weeklyPlanFromRow(&rows[i]))
	}
	return due, nil
}

func weeklyPlanFromRow(row *requestChildRow) enrollment.WeeklyPlan {
	return enrollment.WeeklyPlan{RequestChildID: row.ID, On: *row.WeeklyPlanOn, Replace: row.WeeklyPlanReplaces}
}
