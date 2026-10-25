// Package compose builds the Träger projection (ADR 0084 section 1, ADR
// 0104): its administrative read unit, the Postgres reads and the school set
// Organisation & Tenancy supplies.
package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/traegerview"
	"github.com/moto-nrw/project-phoenix/modules/traegerview/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/traegerview/internal/application"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// New builds the Träger projection over the school set of Organisation &
// Tenancy.
func New(schoolIDs traegerview.SchoolIDs) (traegerview.Overview, error) {
	if schoolIDs == nil {
		return nil, errors.New("traeger view compose: the school IDs of Organisation & Tenancy are required")
	}
	return application.New(schoolIDs, postgres.New(postgres.AmbientAdminDB), tenant.WithinAdmin), nil
}
