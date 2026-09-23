package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

// GuardianStudentAccessRecord is the access half of one relationship.
type GuardianStudentAccessRecord struct {
	TenantID       int64
	RelationshipID int64
	AccountID      *int64
	Permissions    json.RawMessage
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
