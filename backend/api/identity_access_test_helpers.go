//go:build testhooks

package api

import (
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewIdentityAccessForTests binds the native owner with a no-op observation
// sink, matching the other module constructors used by repository fixtures.
// Identity's own suites call its composition themselves; the suites of other
// owners (Delivery, Data Import) may not import it and
// build the owner through the root (ADR 0053 point 7.2, #4005).
func NewIdentityAccessForTests(db transactionruntime.Database) (identityaccess.Owner, error) {
	return identityCompose.New(identityCompose.Dependencies{
		DB: db.Pool(), Observe: func(identityCompose.Observation) {},
	})
}
