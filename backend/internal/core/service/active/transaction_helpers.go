package active

import (
	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	activePort "github.com/moto-nrw/project-phoenix/internal/core/port/active"
	activitiesPort "github.com/moto-nrw/project-phoenix/internal/core/port/activities"
	educationPort "github.com/moto-nrw/project-phoenix/internal/core/port/education"
	facilityPort "github.com/moto-nrw/project-phoenix/internal/core/port/facilities"
	userPort "github.com/moto-nrw/project-phoenix/internal/core/port/users"
	"github.com/uptrace/bun"
)

// txRepositories holds all repository instances that can be wrapped in a transaction.
type txRepositories struct {
	groupRepo          activePort.GroupRepository
	visitRepo          activePort.VisitRepository
	supervisorRepo     activePort.GroupSupervisorRepository
	combinedGroupRepo  activePort.CombinedGroupRepository
	groupMappingRepo   activePort.GroupMappingRepository
	studentRepo        userPort.StudentRepository
	roomRepo           facilityPort.RoomRepository
	activityGroupRepo  activitiesPort.GroupRepository
	activityCatRepo    activitiesPort.CategoryRepository
	educationGroupRepo educationPort.GroupRepository
	personRepo         userPort.PersonRepository
	attendanceRepo     activePort.AttendanceRepository
	teacherRepo        userPort.TeacherRepository
	staffRepo          userPort.StaffRepository
}

// wrapRepositoriesWithTx wraps all repositories with the given transaction.
func wrapRepositoriesWithTx(s *service, tx bun.Tx) txRepositories {
	return txRepositories{
		groupRepo:          base.WithTxIfSupported(s.groupRepo, tx),
		visitRepo:          base.WithTxIfSupported(s.visitRepo, tx),
		supervisorRepo:     base.WithTxIfSupported(s.supervisorRepo, tx),
		combinedGroupRepo:  base.WithTxIfSupported(s.combinedGroupRepo, tx),
		groupMappingRepo:   base.WithTxIfSupported(s.groupMappingRepo, tx),
		studentRepo:        base.WithTxIfSupported(s.studentRepo, tx),
		roomRepo:           base.WithTxIfSupported(s.roomRepo, tx),
		activityGroupRepo:  base.WithTxIfSupported(s.activityGroupRepo, tx),
		activityCatRepo:    base.WithTxIfSupported(s.activityCatRepo, tx),
		educationGroupRepo: base.WithTxIfSupported(s.educationGroupRepo, tx),
		personRepo:         base.WithTxIfSupported(s.personRepo, tx),
		attendanceRepo:     base.WithTxIfSupported(s.attendanceRepo, tx),
		teacherRepo:        base.WithTxIfSupported(s.teacherRepo, tx),
		staffRepo:          base.WithTxIfSupported(s.staffRepo, tx),
	}
}
