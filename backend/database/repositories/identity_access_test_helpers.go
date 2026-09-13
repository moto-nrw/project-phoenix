package repositories

import (
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/uptrace/bun"
)

// NewIdentityAccessForTests binds the native owner with a no-op observation
// sink, matching the other module constructors used by repository fixtures.
func NewIdentityAccessForTests(db *bun.DB) (*identityaccess.Module, error) {
	return identityCompose.New(identityCompose.Dependencies{
		DB: db, Observe: func(identityCompose.Observation) {},
	})
}
