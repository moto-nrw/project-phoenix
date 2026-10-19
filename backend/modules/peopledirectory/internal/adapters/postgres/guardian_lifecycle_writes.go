package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/uptrace/bun"
)

// guardianAccountLockKey is the advisory lock of one parent account in one
// school. The retained guardian profile repository takes the same key, so
// both paths serialize against each other while they coexist.
func guardianAccountLockKey(tenantID, accountID int64) string {
	return fmt.Sprintf("guardian-account:%d:%d", tenantID, accountID)
}

// LockGuardianAccount takes the transaction-scoped advisory lock of the
// account in the tenant.
func (s *GuardianStore) LockGuardianAccount(ctx context.Context, accountID int64) (domain.OperationStats, error) {
	db, scope, err := s.tenantDB(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", guardianAccountLockKey(scope.TenantID, accountID))
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("people directory postgres: lock guardian account link: %w", err)
	}
	return stats, nil
}

// LockAccountHolders row-locks the profile and the profiles holding the
// account; the locks also stop concurrent relationship inserts on them until
// commit.
func (s *GuardianStore) LockAccountHolders(ctx context.Context, guardianID, accountID int64) ([]domain.GuardianAccountHolder, domain.OperationStats, error) {
	db, scope, err := s.tenantDB(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []struct {
		ID        int64  `bun:"id"`
		AccountID *int64 `bun:"account_id"`
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewSelect().TableExpr(`users.guardian_profiles AS "guardian_profile"`).
		ColumnExpr(`"guardian_profile".id, "guardian_profile".account_id`).
		Where(`"guardian_profile".tenant_id = ?`, scope.TenantID).
		Where(`("guardian_profile".id = ? OR "guardian_profile".account_id = ?)`, guardianID, accountID).
		OrderExpr(`"guardian_profile".id`).For("UPDATE").Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: lock guardian profiles: %w", err)
	}
	stats.Rows = int64(len(rows))
	holders := make([]domain.GuardianAccountHolder, 0, len(rows))
	for _, row := range rows {
		holders = append(holders, domain.GuardianAccountHolder{ID: row.ID, AccountID: row.AccountID})
	}
	return holders, stats, nil
}

func (s *GuardianStore) GuardianHasLinks(ctx context.Context, guardianID int64) (bool, domain.OperationStats, error) {
	db, scope, err := s.tenantDB(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	linked, err := db.NewSelect().TableExpr("users.student_guardian_relationships").
		Where("tenant_id = ?", scope.TenantID).
		Where("guardian_profile_id = ?", guardianID).Exists(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return false, stats, fmt.Errorf("people directory postgres: check guardian links: %w", err)
	}
	return linked, stats, nil
}

// ClearChildlessSince marks a profile as linked before the relationship is
// inserted. The row lock serializes this write with retention's SKIP LOCKED
// selection, so an unlink later starts the retention window at its next run.
func (s *GuardianStore) ClearChildlessSince(ctx context.Context, guardianID int64) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.tenantDB(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	affected, stats, err := exec(ctx, "clear linked guardian retention mark", db.NewUpdate().
		TableExpr("users.guardian_profiles").
		Set("childless_since = NULL").
		Where("tenant_id = ?", tenantID).
		Where("id = ?", guardianID).Exec)
	return affected > 0, stats, err
}

// SetGuardianAccount writes account_id and has_account together.
func (s *GuardianStore) SetGuardianAccount(ctx context.Context, guardianID int64, accountID *int64) (bool, domain.OperationStats, error) {
	db, scope, err := s.tenantDB(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	affected, stats, err := exec(ctx, "set guardian account", db.NewUpdate().
		TableExpr("users.guardian_profiles").
		Set("account_id = ?", accountID).
		Set("has_account = ?", accountID != nil).
		Where("tenant_id = ?", scope.TenantID).
		Where("id = ?", guardianID).Exec)
	return affected > 0, stats, err
}

func (s *GuardianStore) DeleteGuardian(ctx context.Context, guardianID int64) (bool, domain.OperationStats, error) {
	db, scope, err := s.tenantDB(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	affected, stats, err := exec(ctx, "delete guardian profile", db.NewDelete().
		TableExpr("users.guardian_profiles").
		Where("tenant_id = ?", scope.TenantID).
		Where("id = ?", guardianID).Exec)
	return affected > 0, stats, err
}

// SetLinkRole writes only the relationship's role; the caller holds the row
// lock.
func (s *GuardianStore) SetLinkRole(ctx context.Context, linkID int64, role string) (bool, domain.OperationStats, error) {
	db, scope, err := s.tenantDB(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	affected, stats, err := exec(ctx, "set guardian link role", db.NewUpdate().
		TableExpr("users.student_guardian_relationships").
		Set("guardian_role = ?", role).
		Where("tenant_id = ?", scope.TenantID).
		Where("id = ?", linkID).Exec)
	return affected > 0, stats, err
}

func (s *GuardianStore) DeleteLink(ctx context.Context, linkID int64) (domain.OperationStats, error) {
	db, scope, err := s.tenantDB(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	_, stats, err := exec(ctx, "delete guardian link", db.NewDelete().
		TableExpr("users.student_guardian_relationships").
		Where("tenant_id = ?", scope.TenantID).
		Where("id = ?", linkID).Exec)
	return stats, err
}

// guardianHasNoLink is true for a profile without a relationship to a child.
const guardianHasNoLink = `NOT EXISTS (SELECT 1 FROM users.student_guardian_relationships AS link
	WHERE link.tenant_id = guardian.tenant_id AND link.guardian_profile_id = guardian.id)`

// MarkChildlessGuardians clears the mark of the tenant's profiles that have
// a relationship and marks those without one from now (#4043). The nightly
// run calls it before it deletes, so a profile counts from the first run that
// sees it without a child.
func (s *GuardianStore) MarkChildlessGuardians(ctx context.Context) (domain.OperationStats, error) {
	db, tenantID, err := s.tenantDB(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	_, cleared, err := exec(ctx, "clear childless guardian marks", db.NewUpdate().
		TableExpr("users.guardian_profiles AS guardian").
		Set("childless_since = NULL").
		Where("guardian.tenant_id = ?", tenantID).
		Where("guardian.childless_since IS NOT NULL").
		Where("NOT ("+guardianHasNoLink+")").Exec)
	if err != nil {
		return cleared, err
	}
	_, marked, err := exec(ctx, "mark childless guardians", db.NewUpdate().
		TableExpr("users.guardian_profiles AS guardian").
		Set("childless_since = NOW()").
		Where("guardian.tenant_id = ?", tenantID).
		Where("guardian.childless_since IS NULL").
		Where(guardianHasNoLink).Exec)
	cleared.Add(marked)
	return cleared, err
}

// LockChildlessGuardiansBefore locks the tenant's profiles childless since
// before cutoff (#4043). SKIP LOCKED leaves a profile a concurrent write
// holds, a relationship insert among them, to the next nightly run.
func (s *GuardianStore) LockChildlessGuardiansBefore(ctx context.Context, cutoff time.Time) ([]int64, domain.OperationStats, error) {
	db, tenantID, err := s.tenantDB(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var ids []int64
	started := time.Now()
	err = db.NewSelect().
		TableExpr("users.guardian_profiles AS guardian").
		ColumnExpr("guardian.id").
		Where("guardian.tenant_id = ?", tenantID).
		Where("guardian.childless_since < ?", cutoff).
		Where(guardianHasNoLink).
		OrderExpr("guardian.id").
		For("UPDATE SKIP LOCKED").
		Scan(ctx, &ids)
	stats := domain.OperationStats{Queries: 1, Rows: int64(len(ids)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: lock childless guardians: %w", err)
	}
	return ids, stats, nil
}

// DeleteChildlessGuardians deletes the locked profiles in one statement. Its
// fresh snapshot checks the mark and the relationships again, so a
// relationship committed between the lock read and this delete keeps its
// profile. Phone numbers, payment data and invitations follow through their
// foreign keys.
func (s *GuardianStore) DeleteChildlessGuardians(ctx context.Context, guardianIDs []int64, cutoff time.Time) (int, domain.OperationStats, error) {
	if len(guardianIDs) == 0 {
		return 0, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.tenantDB(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	affected, stats, err := exec(ctx, "delete childless guardians", db.NewDelete().
		TableExpr("users.guardian_profiles AS guardian").
		Where("guardian.tenant_id = ?", tenantID).
		Where("guardian.id IN (?)", bun.List(guardianIDs)).
		Where("guardian.childless_since < ?", cutoff).
		Where(guardianHasNoLink).Exec)
	return int(affected), stats, err
}
