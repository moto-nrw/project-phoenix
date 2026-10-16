package compose

import (
	workforcePostgres "github.com/moto-nrw/project-phoenix/database/repositories/workforce"
	workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/legacy"
)

// The retained Workforce records the typed root reads through: the group
// substitutions over the Workforce facade and the absence type allowance
// stores. The root may not import the adapters, so the composition names
// them (ADR 0053 point 2).

type (
	// GroupSubstitution is the retained education.group_substitution row the
	// root's School Structure handovers translate (#2747).
	GroupSubstitution                         = workforceLegacy.GroupSubstitution
	SubstitutionBlocker                       = workforceLegacy.SubstitutionBlocker
	StaffAbsenceTypeAllowanceRepository       = workforcePostgres.StaffAbsenceTypeAllowanceRepository
	StaffAbsenceTypeAllowanceChangeRepository = workforcePostgres.StaffAbsenceTypeAllowanceChangeRepository
)

var NewGroupSubstitutionRepository = workforceLegacy.NewGroupSubstitutionRepository
