package repositories

import (
	"github.com/moto-nrw/project-phoenix/database/repositories/config"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
)

// NewHomeLayoutRepository builds the store behind the personal start page
// composition (#2875).
//
// It is a free function rather than a field on Factory on purpose: the start
// page is assembled once, next to the settings services that share its tenant
// runtime, and a new field on a composition root would widen the wiring surface
// the architecture ratchet holds flat. Callers pass the same tenant-aware
// runtime they hand to SetConfigRuntime, so the repository takes part in the
// settings tenant transaction and its rows stay behind RLS.
func NewHomeLayoutRepository(runtime config.Runtime) configModels.HomeLayoutRepository {
	return config.NewHomeLayoutRepository(runtime)
}
