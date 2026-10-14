package api

import (
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewIdentityAccessForTests binds the native owner with a no-op observation
// sink, matching the other module constructors used by repository fixtures.
func NewIdentityAccessForTests(db transactionruntime.Database) (*identityaccess.Module, error) {
	return identityCompose.New(identityCompose.Dependencies{
		DB: db.Pool(), Observe: func(identityCompose.Observation) {},
	})
}
