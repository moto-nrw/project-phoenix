package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// SetChildClassSwitch plans the class switch of an approved re-enrollment;
// nil clears it (#3917).
func (r *Store) SetChildClassSwitch(ctx context.Context, requestChildID int64, change *enrollment.ClassSwitch) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	var from, to *string
	var on *enrollment.Date
	if change != nil {
		from, to, on = &change.From, &change.To, &change.On
	}
	res, err := db.NewUpdate().TableExpr("enrollment.request_children").
		Set("class_switch_from = ?", from).Set("class_switch_to = ?", to).Set("class_switch_on = ?", on).
		Set("updated_at = ?", time.Now()).Where("tenant_id = ? AND id = ?", tenantID, requestChildID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update request child class switch: %w", err)
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

// ChildClassSwitch returns the planned class switch of one request child, or
// nil when none is planned.
func (r *Store) ChildClassSwitch(ctx context.Context, requestChildID int64) (*enrollment.ClassSwitch, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var row requestChildRow
	err = db.NewSelect().Model(&row).Column("id", "class_switch_from", "class_switch_to", "class_switch_on").
		Where("request_child.tenant_id = ? AND request_child.id = ?", tenantID, requestChildID).
		Where("request_child.class_switch_on IS NOT NULL").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read request child class switch: %w", err)
	}
	change := classSwitchFromRow(&row)
	return &change, nil
}

// DueClassSwitches lists the planned class switches due on or before asOf,
// oldest first.
func (r *Store) DueClassSwitches(ctx context.Context, asOf enrollment.Date) ([]enrollment.DueClassSwitch, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []requestChildRow
	err = db.NewSelect().Model(&rows).
		Column("id", "status", "created_student_id", "class_switch_from", "class_switch_to", "class_switch_on").
		Where("request_child.tenant_id = ?", tenantID).
		Where("request_child.class_switch_on IS NOT NULL AND request_child.class_switch_on <= ?", asOf).
		OrderExpr("request_child.class_switch_on, request_child.id").Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list due class switches: %w", err)
	}
	due := make([]enrollment.DueClassSwitch, 0, len(rows))
	for i := range rows {
		due = append(due, enrollment.DueClassSwitch{
			ClassSwitch: classSwitchFromRow(&rows[i]), ChildStatus: rows[i].Status, CreatedStudentID: rows[i].CreatedStudentID,
		})
	}
	return due, nil
}

func classSwitchFromRow(row *requestChildRow) enrollment.ClassSwitch {
	change := enrollment.ClassSwitch{RequestChildID: row.ID}
	if row.ClassSwitchFrom != nil {
		change.From = *row.ClassSwitchFrom
	}
	if row.ClassSwitchTo != nil {
		change.To = *row.ClassSwitchTo
	}
	if row.ClassSwitchOn != nil {
		change.On = *row.ClassSwitchOn
	}
	return change
}
