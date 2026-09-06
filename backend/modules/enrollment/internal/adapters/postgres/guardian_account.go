package postgres

import (
	"context"
	"fmt"
)

func (r *Store) BackfillGuardianAccountID(ctx context.Context, accountID int64, email string) (int, error) {
	db, err := r.resolve(ctx)
	if err != nil {
		return 0, err
	}
	result, err := db.NewRaw(`
  UPDATE enrollment.requests
  SET guardian_account_id = ?
  WHERE guardian_account_id IS NULL
    AND LOWER(TRIM(guardian_email)) = ?
 `, accountID, email).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("parent: backfill guardian_account_id: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("parent: backfill guardian_account_id: rows affected: %w", err)
	}
	return int(rows), nil
}
