package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
)

// NewCleanupRetention composes the Care Plan retention of finished records in
// the nightly cleanup (#4046). It runs on the tenant transaction of the
// school in context and needs no other owner.
func NewCleanupRetention() careplan.CleanupRetention {
	return postgres.CleanupRetention{Store: postgres.New(postgres.RetentionDatabase)}
}
