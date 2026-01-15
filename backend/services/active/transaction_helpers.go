package active

import (
	"github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
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

// withTxIfSupported wraps a repository with a transaction if it implements TransactionalRepository.
func withTxIfSupported[T any](repo T, tx bun.Tx) T {
	if txRepo, ok := any(repo).(base.TransactionalRepository); ok {
		return txRepo.WithTx(tx).(T)
	}
	return repo
}

// wrapRepositoriesWithTx wraps all repositories with the given transaction.
func wrapRepositoriesWithTx(s *service, tx bun.Tx) txRepositories {
	return txRepositories{
		groupRepo:          withTxIfSupported(s.groupRepo, tx),
		visitRepo:          withTxIfSupported(s.visitRepo, tx),
		supervisorRepo:     withTxIfSupported(s.supervisorRepo, tx),
		combinedGroupRepo:  withTxIfSupported(s.combinedGroupRepo, tx),
		groupMappingRepo:   withTxIfSupported(s.groupMappingRepo, tx),
		studentRepo:        withTxIfSupported(s.studentRepo, tx),
		roomRepo:           withTxIfSupported(s.roomRepo, tx),
		activityGroupRepo:  withTxIfSupported(s.activityGroupRepo, tx),
		activityCatRepo:    withTxIfSupported(s.activityCatRepo, tx),
		educationGroupRepo: withTxIfSupported(s.educationGroupRepo, tx),
		personRepo:         withTxIfSupported(s.personRepo, tx),
		attendanceRepo:     withTxIfSupported(s.attendanceRepo, tx),
		teacherRepo:        withTxIfSupported(s.teacherRepo, tx),
		staffRepo:          withTxIfSupported(s.staffRepo, tx),
	}
}
