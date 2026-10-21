package application

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// CapabilityRepositories are the retained activities and schedule
// repositories served over the Timetable capability alone (#2743); the
// composition adds the instance and participant adapters, which also run on
// the database and Student Presence.
type CapabilityRepositories struct {
	ActivitySupervisor ports.PlannedSupervisorRecords
	StudentEnrollment  ports.ActivityEnrollmentRecords
	InstanceStaff      ports.InstanceStaffRecords
}

// NewCapabilityRepositories builds the retained adapters over the Timetable
// owner. The supervisor rows are enriched with the staff and person names
// the RepositoryDirectories resolve.
func NewCapabilityRepositories(capability timetable.Capability, directories ports.RepositoryDirectories) CapabilityRepositories {
	supervisors := staffSupervisorPlannedRepository{PlannedSupervisorRecords: timetableActivitySupervisorRepository{timetable: capability}, membership: directories.StaffPersons}
	return CapabilityRepositories{
		ActivitySupervisor: personSupervisorPlannedRepository{PlannedSupervisorRecords: supervisors, persons: directories.PersonNames},
		StudentEnrollment:  timetableStudentEnrollmentRepository{timetable: capability, alumni: directories.StudentAlumni, names: directories.StudentNames},
		InstanceStaff:      timetableInstanceStaffRepository{timetable: capability},
	}
}
