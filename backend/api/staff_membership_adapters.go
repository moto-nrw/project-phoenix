package api

import (
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// The adapters in this file serve the legacy users.StaffRepository,
// users.TeacherRepository and users.GuestRepository interfaces over the School
// Membership capability. The membership rows come from the owner; the person
// half comes from the People Directory repository and the account, role and
// permission half from identity access. Nothing here builds SQL — a
// composition that used to be one join is now one call per owner.

var _ peoplePorts.StaffRepository = owners.StaffMembershipRepository{}

// --- CRUD ---

// --- person compositions ---

// The account, contact and birthday reads are the staff directory view's
// joins; the methods below bind the owners and map its answers onto the
// legacy shapes.

// The employment-only writes go to their Workforce owner through the
// staff offboarding workflow's live-staff guard: the opt-out and the notes
// apply to live staff members only, as before the split.
