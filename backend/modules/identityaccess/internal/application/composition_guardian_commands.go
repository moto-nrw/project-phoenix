package application

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

func ObserveGuardianAccessGrant[G any](ctx context.Context, grant G, command func(context.Context, G) (domain.OperationStats, error), observe func(ports.Observation)) error {
	started := time.Now()
	stats, err := command(ctx, grant)
	err = MapError(err)
	observe(ports.Observation{Operation: "grant_guardian_student_access", Duration: time.Since(started), Stats: stats, Err: err})
	return err
}
func ObserveGuardianPermissionChange(ctx context.Context, tenantID, relationshipID int64, permissions json.RawMessage, command func(context.Context, int64, int64, json.RawMessage) (bool, domain.OperationStats, error), observe func(ports.Observation)) (bool, error) {
	started := time.Now()
	found, stats, err := command(ctx, tenantID, relationshipID, permissions)
	err = MapError(err)
	observe(ports.Observation{Operation: "set_guardian_student_permissions", Duration: time.Since(started), Stats: stats, Err: err})
	return found, err
}
