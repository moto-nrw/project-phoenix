package compose

import (
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// Repositories are the retained person, student, guardian, staff and teacher
// row stores the typed root builds over the owners and hands the services it
// composes (#2747), as the Timetable composition's Repositories carry the
// schedule rows.
type Repositories struct {
	Persons          peoplePorts.PersonRepository
	Students         peoplePorts.StudentRepository
	StudentGuardians peoplePorts.StudentGuardianRepository
	GuardianProfiles peoplePorts.GuardianProfileRepository
	Staff            peoplePorts.StaffRepository
	Teachers         peoplePorts.TeacherRepository
}
