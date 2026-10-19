package application

import (
	"context"
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

type guardianAccessCommands struct {
	identityaccess.GuardianStudentAccess
	run func(context.Context, func(context.Context) error) error
}

// NewGuardianStudentAccessCommands owns the single-owner access command's unit
// of work. A relationship workflow supplies the ambient transaction.
func NewGuardianStudentAccessCommands(store identityaccess.GuardianStudentAccess, run func(context.Context, func(context.Context) error) error) identityaccess.GuardianStudentAccess {
	return guardianAccessCommands{store, run}
}
func (c guardianAccessCommands) GrantGuardianStudentAccess(ctx context.Context, grant identityaccess.GuardianStudentAccessGrant) error {
	return c.run(ctx, func(txCtx context.Context) error {
		return c.GuardianStudentAccess.GrantGuardianStudentAccess(txCtx, grant)
	})
}
func (c guardianAccessCommands) SetGuardianStudentPermissions(ctx context.Context, schoolID, relationshipID int64, permissions json.RawMessage) (found bool, err error) {
	err = c.run(ctx, func(txCtx context.Context) error {
		found, err = c.GuardianStudentAccess.SetGuardianStudentPermissions(txCtx, schoolID, relationshipID, permissions)
		return err
	})
	return found, err
}
