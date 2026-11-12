package postgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/domain"
)

// CreateGuardianPickupPermissions inserts the pickup halves of many new
// relationships in one statement with the columns
// CreateGuardianPickupPermission writes; pickup_weekdays stays NULL as there.
// A NULL note stays NULL, an empty one stays empty.
func (s *Store) CreateGuardianPickupPermissions(ctx context.Context, permissions []careplan.GuardianPickupPermission) (domain.OperationStats, error) {
	if len(permissions) == 0 {
		return domain.OperationStats{}, nil
	}
	tenantID := permissions[0].TenantID
	relationships, pickups := make([]int64, len(permissions)), make([]bool, len(permissions))
	notes, noteSet := make([]string, len(permissions)), make([]bool, len(permissions))
	for i, permission := range permissions {
		if permission.TenantID != tenantID {
			return domain.OperationStats{}, fmt.Errorf("%w: create guardian pickup permissions", careplan.ErrGuardianPickupTenantMismatch)
		}
		relationships[i], pickups[i] = permission.RelationshipID, permission.CanPickup
		if permission.PickupNotes != nil {
			notes[i], noteSet[i] = *permission.PickupNotes, true
		}
	}
	db, err := s.guardianPickupDatabase(ctx, tenantID, "create guardian pickup permissions")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewRaw(`INSERT INTO users.student_guardian_pickup_permissions
		(tenant_id, relationship_id, can_pickup, pickup_notes)
		SELECT ?, k.relationship_id, k.can_pickup, CASE WHEN k.note_set THEN k.note END
		FROM unnest(?::bigint[], ?::boolean[], ?::text[], ?::boolean[]) WITH ORDINALITY AS k(relationship_id, can_pickup, note, note_set, ord)
		ORDER BY k.ord`,
		tenantID, pgdialect.Array(relationships), pgdialect.Array(pickups), pgdialect.Array(notes), pgdialect.Array(noteSet)),
		"create guardian pickup permissions")
}
