package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/uptrace/bun"
)

// guardianPickupDatabase resolves the caller's transaction for a pickup
// permission command. The command names its school; a tenant transaction must
// be scoped to the same one, while an administrative transaction (tenant zero)
// writes the named school directly.
func (s *Store) guardianPickupDatabase(ctx context.Context, tenantID int64, operation string) (bun.IDB, error) {
	if tenantID <= 0 {
		return nil, fmt.Errorf("care plan postgres: tenant is required to %s", operation)
	}
	db, scoped, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	if scoped > 0 && scoped != tenantID {
		return nil, fmt.Errorf("%w: %s", careplan.ErrGuardianPickupTenantMismatch, operation)
	}
	return db, nil
}

func (s *Store) CreateGuardianPickupPermission(ctx context.Context, permission careplan.GuardianPickupPermission) (domain.OperationStats, error) {
	db, err := s.guardianPickupDatabase(ctx, permission.TenantID, "create guardian pickup permission")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewRaw(`INSERT INTO users.student_guardian_pickup_permissions
		(tenant_id, relationship_id, can_pickup, pickup_notes) VALUES (?, ?, ?, ?)`,
		permission.TenantID, permission.RelationshipID, permission.CanPickup, permission.PickupNotes),
		"create guardian pickup permission")
}

// ChangeGuardianPickupPermission writes the supplied columns. A change without
// any column only reports whether the permission exists.
func (s *Store) ChangeGuardianPickupPermission(ctx context.Context, change careplan.GuardianPickupPermissionChange) (bool, domain.OperationStats, error) {
	db, err := s.guardianPickupDatabase(ctx, change.TenantID, "change guardian pickup permission")
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	if change.CanPickup == nil && !change.SetPickupNotes {
		query := db.NewSelect().TableExpr(`users.student_guardian_pickup_permissions AS "pickup"`).
			Where(`"pickup".tenant_id = ?`, change.TenantID).
			Where(`"pickup".relationship_id = ?`, change.RelationshipID)
		stats := domain.OperationStats{Queries: 1}
		started := time.Now()
		exists, err := query.Exists(ctx)
		stats.StatementDuration = time.Since(started)
		if err != nil {
			return false, stats, fmt.Errorf("care plan postgres: find guardian pickup permission: %w", err)
		}
		return exists, stats, nil
	}
	query := db.NewUpdate().TableExpr(`users.student_guardian_pickup_permissions AS "pickup"`).
		Where(`"pickup".tenant_id = ?`, change.TenantID).
		Where(`"pickup".relationship_id = ?`, change.RelationshipID)
	if change.CanPickup != nil {
		query = query.Set("can_pickup = ?", *change.CanPickup)
	}
	if change.SetPickupNotes {
		query = query.Set("pickup_notes = ?", change.PickupNotes)
	}
	stats, affected, err := execute(ctx, query, "change guardian pickup permission")
	return affected > 0, stats, err
}
