package projection

import (
	"time"

	postgres "github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentaudience"
)

// New binds the projection constructor to its root-owned inputs.
func New(database postgres.Database, applicants postgres.PendingApplicants, links postgres.GuardianLinks, clocks ...func() time.Time) *postgres.Projection {
	return postgres.New(database, applicants, links, clocks...)
}
