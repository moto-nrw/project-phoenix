package services

import (
	"context"

	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/uptrace/bun"
)

// NewGuardianSeedAccess binds the dev CLI's guardian access command without
// composing sessions, token signers, or the serving service factory. The
// caller supplies its transaction and tenant through the context.
func NewGuardianSeedAccess(db *bun.DB) (func(context.Context, string, string) (int64, bool, error), error) {
	access, err := identityCompose.New(identityCompose.Dependencies{
		DB: db, Observe: func(identityCompose.Observation) {},
	})
	if err != nil {
		return nil, err
	}
	return access.SeedGuardianAccount, nil
}
