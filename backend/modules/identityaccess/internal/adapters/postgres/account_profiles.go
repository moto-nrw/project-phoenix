package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (s *Store) FindAccountProfile(ctx context.Context, accountID, tenantID int64) (domain.AccountProfile, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.AccountProfile{}, false, domain.OperationStats{}, err
	}
	var profiles []struct {
		Bio      string
		Settings string
	}
	started := time.Now()
	err = db.NewRaw(`SELECT COALESCE(bio, '') AS bio, COALESCE(settings::text, '') AS settings
		FROM users.profiles WHERE account_id = ? AND tenant_id = ?`, accountID, tenantID).Scan(ctx, &profiles)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return domain.AccountProfile{}, false, stats, fmt.Errorf("identity access postgres: find account profile: %w", err)
	}
	if len(profiles) == 0 {
		return domain.AccountProfile{}, false, stats, nil
	}
	stats.Rows = 1
	return domain.AccountProfile(profiles[0]), true, stats, nil
}

func (s *Store) SetAccountBio(ctx context.Context, accountID, tenantID int64, bio string) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	_, err = db.NewRaw(`INSERT INTO users.profiles (account_id, tenant_id, bio, settings)
		VALUES (?, ?, ?, '{}'::jsonb)
		ON CONFLICT (tenant_id, account_id) DO UPDATE SET bio = EXCLUDED.bio`, accountID, tenantID, bio).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: set account bio: %w", err)
	}
	stats.Rows = 1
	return stats, nil
}
