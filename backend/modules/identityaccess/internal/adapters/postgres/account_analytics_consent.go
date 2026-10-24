package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

type analyticsConsentRow struct {
	AnalyticsConsent          *bool      `bun:"analytics_consent"`
	AnalyticsConsentDecidedAt *time.Time `bun:"analytics_consent_decided_at"`
}

// FindAnalyticsConsent reads the usage analytics answer of a live account.
func (s *Store) FindAnalyticsConsent(ctx context.Context, accountID int64) (domain.AnalyticsConsent, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.AnalyticsConsent{}, false, domain.OperationStats{}, err
	}
	var rows []analyticsConsentRow
	started := time.Now()
	err = db.NewRaw(`SELECT analytics_consent, analytics_consent_decided_at
		FROM auth.accounts WHERE id = ? AND personal_data_erased_at IS NULL`, accountID).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return domain.AnalyticsConsent{}, false, stats, fmt.Errorf("identity access postgres: find analytics consent: %w", err)
	}
	if len(rows) == 0 {
		return domain.AnalyticsConsent{}, false, stats, nil
	}
	return domain.AnalyticsConsent{Granted: rows[0].AnalyticsConsent, DecidedAt: rows[0].AnalyticsConsentDecidedAt}, true, stats, nil
}

// SetAnalyticsConsent records the answer of a live account with the time of
// the decision.
func (s *Store) SetAnalyticsConsent(ctx context.Context, accountID int64, granted bool) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewRaw(`UPDATE auth.accounts
		SET analytics_consent = ?, analytics_consent_decided_at = NOW()
		WHERE id = ? AND personal_data_erased_at IS NULL`, granted, accountID).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: set analytics consent: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats.Rows > 0, stats, err
}
