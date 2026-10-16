package compose

import usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"

// Repositories are the retained person, student, guardian, staff and teacher
// row stores the typed root builds over the owners and hands the services it
// composes (#2747), as the Timetable composition's Repositories carry the
// schedule rows.
type Repositories struct {
	Persons          usersRepo.PersonRepository
	Students         usersRepo.StudentRepository
	StudentGuardians usersRepo.StudentGuardianRepository
	GuardianProfiles usersRepo.GuardianProfileRepository
	Staff            usersRepo.StaffRepository
	Teachers         usersRepo.TeacherRepository
}
