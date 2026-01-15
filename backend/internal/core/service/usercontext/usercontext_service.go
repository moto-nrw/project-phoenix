package usercontext

import (
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/activities"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/education"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	authPort "github.com/moto-nrw/project-phoenix/internal/core/port/auth"
)

// Operation name constants to avoid string duplication
const (
	opGetCurrentStaff  = "get current staff"
	opGetGroupStudents = "get group students"
)

// UserContextRepositories groups all repository dependencies for UserContextService
// This struct reduces the number of parameters passed to the constructor
type UserContextRepositories struct {
	AccountRepo        authPort.AccountRepository
	PersonRepo         users.PersonRepository
	StaffRepo          users.StaffRepository
	TeacherRepo        users.TeacherRepository
	StudentRepo        users.StudentRepository
	EducationGroupRepo education.GroupRepository
	ActivityGroupRepo  activities.GroupRepository
	ActiveGroupRepo    active.GroupRepository
	VisitsRepo         active.VisitRepository
	SupervisorRepo     active.GroupSupervisorRepository
	ProfileRepo        users.ProfileRepository
	SubstitutionRepo   education.GroupSubstitutionRepository
}

// userContextService implements the UserContextService interface
type userContextService struct {
	accountRepo        authPort.AccountRepository
	personRepo         users.PersonRepository
	staffRepo          users.StaffRepository
	teacherRepo        users.TeacherRepository
	studentRepo        users.StudentRepository
	educationGroupRepo education.GroupRepository
	activityGroupRepo  activities.GroupRepository
	activeGroupRepo    active.GroupRepository
	visitsRepo         active.VisitRepository
	supervisorRepo     active.GroupSupervisorRepository
	profileRepo        users.ProfileRepository
	substitutionRepo   education.GroupSubstitutionRepository
	db                 *bun.DB
	txHandler          *base.TxHandler
}

// NewUserContextServiceWithRepos creates a new user context service using a repositories struct
func NewUserContextServiceWithRepos(repos UserContextRepositories, db *bun.DB) UserContextService {
	return &userContextService{
		accountRepo:        repos.AccountRepo,
		personRepo:         repos.PersonRepo,
		staffRepo:          repos.StaffRepo,
		teacherRepo:        repos.TeacherRepo,
		studentRepo:        repos.StudentRepo,
		educationGroupRepo: repos.EducationGroupRepo,
		activityGroupRepo:  repos.ActivityGroupRepo,
		activeGroupRepo:    repos.ActiveGroupRepo,
		visitsRepo:         repos.VisitsRepo,
		supervisorRepo:     repos.SupervisorRepo,
		profileRepo:        repos.ProfileRepo,
		substitutionRepo:   repos.SubstitutionRepo,
		db:                 db,
		txHandler:          base.NewTxHandler(db),
	}
}

// WithTx returns a new service that uses the provided transaction
func (s *userContextService) WithTx(tx bun.Tx) any {
	// Get repositories with transaction
	var accountRepo = s.accountRepo
	var personRepo = s.personRepo
	var staffRepo = s.staffRepo
	var teacherRepo = s.teacherRepo
	var studentRepo = s.studentRepo
	var educationGroupRepo = s.educationGroupRepo
	var activityGroupRepo = s.activityGroupRepo
	var activeGroupRepo = s.activeGroupRepo
	var visitsRepo = s.visitsRepo
	var supervisorRepo = s.supervisorRepo
	var profileRepo = s.profileRepo

	// Apply transaction to repositories that implement TransactionalRepository
	if txRepo, ok := s.accountRepo.(base.TransactionalRepository); ok {
		accountRepo = txRepo.WithTx(tx).(authPort.AccountRepository)
	}
	if txRepo, ok := s.personRepo.(base.TransactionalRepository); ok {
		personRepo = txRepo.WithTx(tx).(users.PersonRepository)
	}
	if txRepo, ok := s.staffRepo.(base.TransactionalRepository); ok {
		staffRepo = txRepo.WithTx(tx).(users.StaffRepository)
	}
	if txRepo, ok := s.teacherRepo.(base.TransactionalRepository); ok {
		teacherRepo = txRepo.WithTx(tx).(users.TeacherRepository)
	}
	if txRepo, ok := s.studentRepo.(base.TransactionalRepository); ok {
		studentRepo = txRepo.WithTx(tx).(users.StudentRepository)
	}
	if txRepo, ok := s.educationGroupRepo.(base.TransactionalRepository); ok {
		educationGroupRepo = txRepo.WithTx(tx).(education.GroupRepository)
	}
	if txRepo, ok := s.activityGroupRepo.(base.TransactionalRepository); ok {
		activityGroupRepo = txRepo.WithTx(tx).(activities.GroupRepository)
	}
	if txRepo, ok := s.activeGroupRepo.(base.TransactionalRepository); ok {
		activeGroupRepo = txRepo.WithTx(tx).(active.GroupRepository)
	}
	if txRepo, ok := s.visitsRepo.(base.TransactionalRepository); ok {
		visitsRepo = txRepo.WithTx(tx).(active.VisitRepository)
	}
	if txRepo, ok := s.supervisorRepo.(base.TransactionalRepository); ok {
		supervisorRepo = txRepo.WithTx(tx).(active.GroupSupervisorRepository)
	}
	if txRepo, ok := s.profileRepo.(base.TransactionalRepository); ok {
		profileRepo = txRepo.WithTx(tx).(users.ProfileRepository)
	}

	// Return a new service with the transaction
	return &userContextService{
		accountRepo:        accountRepo,
		personRepo:         personRepo,
		staffRepo:          staffRepo,
		teacherRepo:        teacherRepo,
		studentRepo:        studentRepo,
		educationGroupRepo: educationGroupRepo,
		activityGroupRepo:  activityGroupRepo,
		activeGroupRepo:    activeGroupRepo,
		visitsRepo:         visitsRepo,
		supervisorRepo:     supervisorRepo,
		profileRepo:        profileRepo,
		db:                 s.db,
		txHandler:          s.txHandler.WithTx(tx),
	}
}
