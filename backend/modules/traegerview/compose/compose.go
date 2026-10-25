// Package compose builds the Träger projection (ADR 0084 section 1, ADR
// 0104, ADR 0109): its administrative read unit and the Postgres reads over
// the ports the root binds.
package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/traegerview"
	"github.com/moto-nrw/project-phoenix/modules/traegerview/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/traegerview/internal/application"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// New builds the Träger projection.
func New(ports traegerview.Ports) (traegerview.Overview, error) {
	overview, err := application.New(ports, postgres.New(postgres.AmbientAdminDB), tenant.WithinAdmin)
	if err != nil {
		return nil, err
	}
	return overview, nil
}
