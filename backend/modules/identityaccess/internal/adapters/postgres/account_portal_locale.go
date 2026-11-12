package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
)

func (s *Store) FindAccountPortalLocale(ctx context.Context, accountID int64) (*string, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, false, domain.OperationStats{}, err
	}
	var rows []struct{ PortalLocale *string }
	started := time.Now()
	err = db.NewRaw(`SELECT portal_locale FROM auth.accounts WHERE id = ? AND personal_data_erased_at IS NULL`, accountID).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return nil, false, stats, fmt.Errorf("identity access postgres: find account portal locale: %w", err)
	}
	if len(rows) == 0 {
		return nil, false, stats, nil
	}
	return rows[0].PortalLocale, true, stats, nil
}

func (s *Store) SetAccountPortalLocale(ctx context.Context, accountID int64, locale string) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewRaw(`UPDATE auth.accounts SET portal_locale = ? WHERE id = ? AND personal_data_erased_at IS NULL`, locale, accountID).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: set account portal locale: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats.Rows > 0, stats, err
}
