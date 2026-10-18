package compose

import workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/internal/adapters/substitutions"

// Retained group-substitution records serve School Structure handovers through the typed root.
// Composition names the adapters the root may not import (ADR 0053 point 2).

type (
	// GroupSubstitution is the retained education.group_substitution row the
	// root's School Structure handovers translate (#2747).
	GroupSubstitution   = workforceLegacy.GroupSubstitution
	SubstitutionBlocker = workforceLegacy.SubstitutionBlocker
)

var NewGroupSubstitutionRepository = workforceLegacy.NewGroupSubstitutionRepository
