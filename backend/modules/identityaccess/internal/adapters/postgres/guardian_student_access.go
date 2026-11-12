package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// GuardianStudentAccessRecord is the access half of one relationship.
type GuardianStudentAccessRecord struct {
	TenantID       int64
	RelationshipID int64
	AccountID      *int64
	Permissions    json.RawMessage
}

// GuardianStudentAccessStore is Identity & Access's persistence surface for
// the guardian relationship owner command.
type GuardianStudentAccessStore interface {
	LockLiveGuardianAccount(context.Context, int64) (bool, error)
	GrantGuardianStudentAccess(context.Context, GuardianStudentAccessRecord) (domain.OperationStats, error)
	InsertGuardianStudentAccesses(context.Context, []domain.GuardianStudentAccessGrant) (domain.OperationStats, error)
	SetGuardianStudentPermissions(context.Context, int64, int64, json.RawMessage) (bool, domain.OperationStats, error)
}

// guardianStudentAccessDatabase resolves the caller's transaction for a
// command that names its school. A tenant transaction must be scoped to the
// same school; an administrative transaction writes the named one.
func (s *Store) guardianStudentAccessDatabase(ctx context.Context, tenantID int64, operation string) (bun.IDB, error) {
	if tenantID <= 0 {
		return nil, fmt.Errorf("identity access postgres: tenant is required to %s", operation)
	}
	if scoped := s.scope(ctx).TenantID; scoped > 0 && scoped != tenantID {
		return nil, fmt.Errorf("%w: %s", domain.ErrGuardianStudentAccessTenantMismatch, operation)
	}
	return s.database(ctx)
}

func guardianPermissionsJSON(permissions json.RawMessage) string {
	if len(permissions) == 0 {
		return "{}"
	}
	return string(permissions)
}

func (s *Store) GrantGuardianStudentAccess(ctx context.Context, record GuardianStudentAccessRecord) (domain.OperationStats, error) {
	db, err := s.guardianStudentAccessDatabase(ctx, record.TenantID, "grant guardian student access")
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewRaw(`INSERT INTO auth.guardian_student_access (tenant_id, relationship_id, account_id, permissions)
		VALUES (?, ?, ?, ?::jsonb)`,
		record.TenantID, record.RelationshipID, record.AccountID, guardianPermissionsJSON(record.Permissions)).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: grant guardian student access: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

// InsertGuardianStudentAccesses inserts the access halves of many new
// relationships in one statement with the columns GrantGuardianStudentAccess
// writes. Every record names the same school; the account reference trigger
// runs per row as there.
func (s *Store) InsertGuardianStudentAccesses(ctx context.Context, records []domain.GuardianStudentAccessGrant) (domain.OperationStats, error) {
	if len(records) == 0 {
		return domain.OperationStats{}, nil
	}
	tenantID := records[0].TenantID
	relationships, accounts := make([]int64, len(records)), make([]int64, len(records))
	permissions := make([]string, len(records))
	for i, record := range records {
		if record.TenantID != tenantID {
			return domain.OperationStats{}, fmt.Errorf("%w: grant guardian student accesses", domain.ErrGuardianStudentAccessTenantMismatch)
		}
		relationships[i], permissions[i] = record.RelationshipID, guardianPermissionsJSON(record.Permissions)
		if record.AccountID != nil {
			accounts[i] = *record.AccountID
		}
	}
	db, err := s.guardianStudentAccessDatabase(ctx, tenantID, "grant guardian student accesses")
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewRaw(`INSERT INTO auth.guardian_student_access (tenant_id, relationship_id, account_id, permissions)
		SELECT ?, k.relationship_id, NULLIF(k.account_id, 0), k.permissions::jsonb
		FROM unnest(?::bigint[], ?::bigint[], ?::text[]) WITH ORDINALITY AS k(relationship_id, account_id, permissions, ord)
		ORDER BY k.ord`,
		tenantID, pgdialect.Array(relationships), pgdialect.Array(accounts), pgdialect.Array(permissions)).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: grant guardian student accesses: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	if stats.Rows != int64(len(records)) {
		return stats, fmt.Errorf("identity access postgres: grant guardian student accesses: %d of %d rows written", stats.Rows, len(records))
	}
	return stats, nil
}

func (s *Store) SetGuardianStudentPermissions(ctx context.Context, tenantID, relationshipID int64, permissions json.RawMessage) (bool, domain.OperationStats, error) {
	db, err := s.guardianStudentAccessDatabase(ctx, tenantID, "set guardian student permissions")
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewRaw(`UPDATE auth.guardian_student_access SET permissions = ?::jsonb
		WHERE tenant_id = ? AND relationship_id = ?`,
		guardianPermissionsJSON(permissions), tenantID, relationshipID).Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: set guardian student permissions: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats.Rows > 0, stats, nil
}
