package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/filestorage"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/application"
)

// NewCleanupRetention composes the retention of the technical remains of the
// nightly cleanup (#4046, #4045). It runs on the tenant transaction of the school in
// context and needs neither the object store nor another owner.
func NewCleanupRetention() filestorage.CleanupRetention {
	return application.NewCleanupRetention(postgres.NewCleanupRetentionStore(postgres.RetentionDatabase))
}
