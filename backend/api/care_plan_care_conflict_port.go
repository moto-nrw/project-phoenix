package api

import (
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
)

// The coordinator's care port only translates values and errors. Care Plan
// owns pending checks, conflict-key parsing, day locks, and staff writes.
var _ carePlanCompose.ParentRequestConflicts = (*owners.CareScheduleRequestService)(nil)
