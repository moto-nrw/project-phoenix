package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

type accountMetadataRow struct {
	ID            int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Email         string
	Username      *string
	Avatar        string
	Active        bool
	IsPasswordOTP bool
	LastLogin     *time.Time
}

func (s *Store) FindAccountMetadata(ctx context.Context, accountID int64) (domain.AccountMetadata, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.AccountMetadata{}, false, domain.OperationStats{}, err
	}
	var rows []accountMetadataRow
	started := time.Now()
	err = db.NewRaw(`SELECT id, created_at, updated_at, email, username, avatar, active, is_password_otp, last_login
		FROM auth.accounts WHERE id = ?`, accountID).Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started), Rows: int64(len(rows))}
	if err != nil {
		return domain.AccountMetadata{}, false, stats, fmt.Errorf("identity access postgres: find account metadata: %w", err)
	}
	if len(rows) == 0 {
		return domain.AccountMetadata{}, false, stats, nil
	}
	return domain.AccountMetadata(rows[0]), true, stats, nil
}

func (s *Store) SetAccountUsername(ctx context.Context, accountID int64, username string) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewRaw(`UPDATE auth.accounts SET username = NULLIF(?, '') WHERE id = ?`, username, accountID).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: set account username: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats.Rows > 0, stats, err
}

func (s *Store) SetAccountAvatar(ctx context.Context, accountID int64, avatar string) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewRaw(`UPDATE auth.accounts SET avatar = ? WHERE id = ?`, avatar, accountID).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: set account avatar: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	return stats.Rows > 0, stats, err
}
