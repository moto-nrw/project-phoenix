package active

import (
	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/internal/core/domain/activities"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	educationModels "github.com/moto-nrw/project-phoenix/internal/core/domain/education"
	facilityModels "github.com/moto-nrw/project-phoenix/internal/core/domain/facilities"
	userModels "github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	"github.com/uptrace/bun"
)

// txRepositories holds all repository instances that can be wrapped in a transaction.
type txRepositories struct {
	groupRepo          active.GroupRepository
	visitRepo          active.VisitRepository
	supervisorRepo     active.GroupSupervisorRepository
	combinedGroupRepo  active.CombinedGroupRepository
	groupMappingRepo   active.GroupMappingRepository
	studentRepo        userModels.StudentRepository
	roomRepo           facilityModels.RoomRepository
	activityGroupRepo  activitiesModels.GroupRepository
	activityCatRepo    activitiesModels.CategoryRepository
	educationGroupRepo educationModels.GroupRepository
	personRepo         userModels.PersonRepository
	attendanceRepo     active.AttendanceRepository
	teacherRepo        userModels.TeacherRepository
	staffRepo          userModels.StaffRepository
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
