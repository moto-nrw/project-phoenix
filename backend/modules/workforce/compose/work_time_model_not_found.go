package compose

import postgresBindings "github.com/moto-nrw/project-phoenix/modules/workforce/internal/adapters/postgres"

func LegacyWorkTimeModelNotFound(err error) error {
	return postgresBindings.LegacyWorkTimeModelNotFound(err)
}
