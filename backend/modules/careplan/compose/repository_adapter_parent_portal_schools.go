package compose

import carePlanBindingPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"

func (r ParentPortalRepositories) WithSchools(schools carePlanBindingPorts.ParentPortalSchools) ParentPortalRepositories {
	return ParentPortalRepositories{ParentPortalRepositories: r.ParentPortalRepositories.WithSchools(schools)}
}
