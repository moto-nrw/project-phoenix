package api

import (
	owners "github.com/moto-ogs/project-phoenix/roots/owners"

	peoplePorts "github.com/moto-ogs/project-phoenix/modules/peopledirectory/ports"
)

var _ peoplePorts.TeacherRepository = owners.TeacherMembershipRepository{}

// --- CRUD ---

// --- staff + person compositions ---

// --- caregivers ---
