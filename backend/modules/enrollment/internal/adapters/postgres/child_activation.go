package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (r *Store) UpdateChildActivationPlan(ctx context.Context, id int64, mode string, on *enrollment.Date) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	result, err := db.NewUpdate().TableExpr("enrollment.request_children").
		Set("activation_mode = ?", mode).Set("activate_on = ?", on).
		Set("updated_at = ?", time.Now()).Where("tenant_id = ? AND id = ?", tenantID, id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update request child activation plan: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("request child %d not found", id)
	}
	return nil
}
