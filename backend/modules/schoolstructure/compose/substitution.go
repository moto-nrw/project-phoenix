package compose

import groups "github.com/moto-ogs/project-phoenix/modules/schoolstructure/internal/application/groups"

func NewSubstitutionModule(deps SubstitutionDependencies) SubstitutionModule {
	return groups.NewPublicSubstitutionModule(deps)
}
