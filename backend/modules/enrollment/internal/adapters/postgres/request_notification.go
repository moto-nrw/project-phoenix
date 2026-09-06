package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (r *Store) PinDecisionNotificationMode(ctx context.Context, requestID int64, proposed string) (string, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return "", err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return "", err
	}
	var mode string
	err = db.NewRaw(`
		UPDATE enrollment.requests
		SET decision_notification_mode = COALESCE(decision_notification_mode, ?),
			updated_at = CASE
				WHEN decision_notification_mode IS NULL THEN NOW()
				ELSE updated_at
			END
		WHERE id = ? AND tenant_id = ?
		RETURNING decision_notification_mode
	`, proposed, requestID, tenantID).Scan(ctx, &mode)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("enrollment request %d not found for notification mode pin", requestID)
		}
		return "", fmt.Errorf("failed to pin enrollment decision notification mode: %w", err)
	}
	return mode, nil
}
