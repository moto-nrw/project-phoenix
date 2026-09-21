package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

func (s *Store) FindStaffCalendarFeedOwner(ctx context.Context, tokenHash string) (domain.StaffCalendarFeedOwner, bool, domain.OperationStats, error) {
	if tokenHash == "" {
		return domain.StaffCalendarFeedOwner{}, false, domain.OperationStats{}, nil
	}
	db, err := s.database(ctx)
	if err != nil {
		return domain.StaffCalendarFeedOwner{}, false, domain.OperationStats{}, err
	}
	var owner domain.StaffCalendarFeedOwner
	started := time.Now()
	err = db.NewRaw("SELECT account_id, tenant_id FROM auth.account_tenants WHERE staff_calendar_feed_token = ? AND status = 'active'", tokenHash).Scan(ctx, &owner)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.StaffCalendarFeedOwner{}, false, stats, nil
	}
	if err != nil {
		return domain.StaffCalendarFeedOwner{}, false, stats, fmt.Errorf("database error during find staff calendar feed owner: %w", err)
	}
	stats.Rows = 1
	return owner, true, stats, nil
}

// EnsureStaffCalendarFeedToken installs only the first hash. A concurrent
// loser reads the winner's hash, never replacing it with its own candidate.
func (s *Store) EnsureStaffCalendarFeedToken(ctx context.Context, accountID, tenantID int64, tokenHash string) (string, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return "", domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewRaw(`UPDATE auth.account_tenants SET staff_calendar_feed_token = ?
 WHERE account_id = ? AND tenant_id = ? AND status = 'active'
 AND (staff_calendar_feed_token IS NULL OR staff_calendar_feed_token = '')`, tokenHash, accountID, tenantID).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return "", stats, fmt.Errorf("database error during ensure staff calendar feed token: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr == nil && rows > 0 {
		stats.Rows = rows
		return tokenHash, stats, nil
	}
	stored, readStats, err := readStaffCalendarFeedToken(ctx, db, accountID, tenantID)
	stats.Add(readStats)
	return stored, stats, err
}

func readStaffCalendarFeedToken(ctx context.Context, db bun.IDB, accountID, tenantID int64) (string, domain.OperationStats, error) {
	var stored string
	started := time.Now()
	err := db.NewRaw("SELECT COALESCE(staff_calendar_feed_token, '') FROM auth.account_tenants WHERE account_id = ? AND tenant_id = ? AND status = 'active'", accountID, tenantID).Scan(ctx, &stored)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return "", stats, nil
	}
	if err != nil {
		return "", stats, fmt.Errorf("database error during read staff calendar feed token: %w", err)
	}
	stats.Rows = 1
	return stored, stats, nil
}

func (s *Store) RotateStaffCalendarFeedToken(ctx context.Context, accountID, tenantID int64, tokenHash string) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewRaw("UPDATE auth.account_tenants SET staff_calendar_feed_token = ? WHERE account_id = ? AND tenant_id = ? AND status = 'active'", tokenHash, accountID, tenantID).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("database error during rotate staff calendar feed token: %w", err)
	}
	rows, err := result.RowsAffected()
	stats.Rows = rows
	return err == nil && rows > 0, stats, err
}
