package repositories

import (
	"time"

	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

type GradeTransitionTestRepositories struct {
	Timetable           TimetableTestRepositories
	Transition          educationModels.GradeTransitionRepository
	Attendance          activeModels.AttendanceRepository
	ClassListEntry      usersModels.ClassListEntryRepository
	ClassListEntryAudit auditModels.ClassListEntryChangeRepository
}

func NewAttendanceTestRepository(db *bun.DB, clocks ...func() time.Time) activeModels.AttendanceRepository {
	return activeRepo.NewAttendanceRepository(db, clocks...)
}

func NewGradeTransitionTestRepositories(db *bun.DB, command auditModels.Command, clocks ...func() time.Time) (GradeTransitionTestRepositories, error) {
	timetable, err := NewTimetableTestRepositories(db, clocks...)
	if err != nil {
		return GradeTransitionTestRepositories{}, err
	}
	persons, err := NewPeopleDirectory(db)
	if err != nil {
		return GradeTransitionTestRepositories{}, err
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return GradeTransitionTestRepositories{}, err
	}
	transition := educationRepo.NewGradeTransitionRepository(db)
	transition.(*educationRepo.GradeTransitionRepository).BindStudentDirectory(educationStudentDirectory{students: persons, commands: persons})
	return GradeTransitionTestRepositories{
		Timetable:           timetable,
		Transition:          personGradeTransitionRepository{GradeTransitionRepository: transition, persons: persons},
		Attendance:          activeRepo.NewAttendanceRepository(db, clocks...),
		ClassListEntry:      classListEntryMembershipRepository{membership: membership},
		ClassListEntryAudit: classListEntryChangeCommand{command: command},
	}, nil
}
