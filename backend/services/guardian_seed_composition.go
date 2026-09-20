package services

import (
	"context"

	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/uptrace/bun"
)

// NewGuardianSeedAccess binds the dev CLI's guardian access command without
// composing sessions, token signers, or the serving service factory. The
// caller supplies its transaction and tenant through the context.
func NewGuardianSeedAccess(db *bun.DB) (func(context.Context, int64) error, error) {
	access, err := identityCompose.New(identityCompose.Dependencies{
		DB: db, Observe: func(identityCompose.Observation) {},
	})
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, accountID int64) error {
		_, err := access.GrantGuardianTenantAccess(ctx, accountID)
		return err
	}, nil
}
