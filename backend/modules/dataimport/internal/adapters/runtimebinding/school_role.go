package runtimebinding

import "github.com/moto-nrw/project-phoenix/modules/dataimport"

func AssignmentFacts[F any](role *dataimport.SchoolRole, construct func(int64, *int64, string, bool, *string) F) *F {
	if role == nil {
		return nil
	}
	value := construct(role.ID, role.TenantID, role.Name, role.IsSystem, role.BaseRole)
	return &value
}
