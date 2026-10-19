package compose

import (
	"context"
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	internalports "github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

type retainedGuardianStudentAccess struct {
	postgres.GuardianStudentAccessStore
	Observe func(internalports.Observation)
}

func (a retainedGuardianStudentAccess) GrantGuardianStudentAccess(ctx context.Context, grant identityaccess.GuardianStudentAccessGrant) error {
	return application.ObserveGuardianAccessGrant(ctx, postgres.GuardianStudentAccessRecord(grant), a.GuardianStudentAccessStore.GrantGuardianStudentAccess, a.Observe)
}
func (a retainedGuardianStudentAccess) SetGuardianStudentPermissions(ctx context.Context, tenantID, relationshipID int64, permissions json.RawMessage) (bool, error) {
	return application.ObserveGuardianPermissionChange(ctx, tenantID, relationshipID, permissions, a.GuardianStudentAccessStore.SetGuardianStudentPermissions, a.Observe)
}
