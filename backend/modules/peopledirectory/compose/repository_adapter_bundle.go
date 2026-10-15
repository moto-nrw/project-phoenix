package compose

import userModels "github.com/moto-nrw/project-phoenix/models/users"

// Repositories are the retained person, student, guardian, staff and teacher
// row stores the typed root builds over the owners and hands the services it
// composes (#2747), as the Timetable composition's Repositories carry the
// schedule rows.
type Repositories struct {
	Persons          userModels.PersonRepository
	Students         userModels.StudentRepository
	StudentGuardians userModels.StudentGuardianRepository
	GuardianProfiles userModels.GuardianProfileRepository
	Staff            userModels.StaffRepository
	Teachers         userModels.TeacherRepository
}
