package usercontext

import (
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	activePort "github.com/moto-nrw/project-phoenix/internal/core/port/active"
	activitiesPort "github.com/moto-nrw/project-phoenix/internal/core/port/activities"
	authPort "github.com/moto-nrw/project-phoenix/internal/core/port/auth"
	educationPort "github.com/moto-nrw/project-phoenix/internal/core/port/education"
	userPort "github.com/moto-nrw/project-phoenix/internal/core/port/users"
)

// Operation name constants to avoid string duplication
const (
	opGetCurrentStaff  = "get current staff"
	opGetGroupStudents = "get group students"
)

// UserContextRepositories groups all repository dependencies for UserContextService
// This struct reduces the number of parameters passed to the constructor
type UserContextRepositories struct {
	AccountRepo               authPort.AccountRepository
	PersonRepo                userPort.PersonRepository
	StaffRepo                 userPort.StaffRepository
	TeacherRepo               userPort.TeacherRepository
	StudentRepo               userPort.StudentRepository
	EducationGroupRepo        educationPort.GroupRepository
	ActivityGroupRepo         activitiesPort.GroupRepository
	ActiveGroupRepo           activePort.GroupReadRepository
	VisitsRepo                activePort.VisitReadRepository
	SupervisorRepo            activePort.GroupSupervisorRepository
	ProfileRepo               userPort.ProfileRepository
	SubstitutionRepo          educationPort.GroupSubstitutionRepository
	SubstitutionRelationsRepo educationPort.GroupSubstitutionRelationsRepository
}

// userContextService implements the UserContextService interface
type userContextService struct {
	accountRepo               authPort.AccountRepository
	personRepo                userPort.PersonRepository
	staffRepo                 userPort.StaffRepository
	teacherRepo               userPort.TeacherRepository
	studentRepo               userPort.StudentRepository
	educationGroupRepo        educationPort.GroupRepository
	activityGroupRepo         activitiesPort.GroupRepository
	activeGroupRepo           activePort.GroupReadRepository
	visitsRepo                activePort.VisitReadRepository
	supervisorRepo            activePort.GroupSupervisorRepository
	profileRepo               userPort.ProfileRepository
	substitutionRepo          educationPort.GroupSubstitutionRepository
	substitutionRelationsRepo educationPort.GroupSubstitutionRelationsRepository
	db                        *bun.DB
	txHandler                 *base.TxHandler
}

// NewUserContextServiceWithRepos creates a new user context service using a repositories struct
func NewUserContextServiceWithRepos(repos UserContextRepositories, db *bun.DB) UserContextService {
	return &userContextService{
		accountRepo:               repos.AccountRepo,
		personRepo:                repos.PersonRepo,
		staffRepo:                 repos.StaffRepo,
		teacherRepo:               repos.TeacherRepo,
		studentRepo:               repos.StudentRepo,
		educationGroupRepo:        repos.EducationGroupRepo,
		activityGroupRepo:         repos.ActivityGroupRepo,
		activeGroupRepo:           repos.ActiveGroupRepo,
		visitsRepo:                repos.VisitsRepo,
		supervisorRepo:            repos.SupervisorRepo,
		profileRepo:               repos.ProfileRepo,
		substitutionRepo:          repos.SubstitutionRepo,
		substitutionRelationsRepo: repos.SubstitutionRelationsRepo,
		db:                        db,
		txHandler:                 base.NewTxHandler(db),
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
	var substitutionRepo = s.substitutionRepo
	var substitutionRelationsRepo = s.substitutionRelationsRepo

	// Apply transaction to repositories that implement TransactionalRepository
	if txRepo, ok := s.accountRepo.(base.TransactionalRepository); ok {
		accountRepo = txRepo.WithTx(tx).(authPort.AccountRepository)
	}
	if txRepo, ok := s.personRepo.(base.TransactionalRepository); ok {
		personRepo = txRepo.WithTx(tx).(userPort.PersonRepository)
	}
	if txRepo, ok := s.staffRepo.(base.TransactionalRepository); ok {
		staffRepo = txRepo.WithTx(tx).(userPort.StaffRepository)
	}
	if txRepo, ok := s.teacherRepo.(base.TransactionalRepository); ok {
		teacherRepo = txRepo.WithTx(tx).(userPort.TeacherRepository)
	}
	if txRepo, ok := s.studentRepo.(base.TransactionalRepository); ok {
		studentRepo = txRepo.WithTx(tx).(userPort.StudentRepository)
	}
	if txRepo, ok := s.educationGroupRepo.(base.TransactionalRepository); ok {
		educationGroupRepo = txRepo.WithTx(tx).(educationPort.GroupRepository)
	}
	if txRepo, ok := s.activityGroupRepo.(base.TransactionalRepository); ok {
		activityGroupRepo = txRepo.WithTx(tx).(activitiesPort.GroupRepository)
	}
	if txRepo, ok := s.activeGroupRepo.(base.TransactionalRepository); ok {
		activeGroupRepo = txRepo.WithTx(tx).(activePort.GroupReadRepository)
	}
	if txRepo, ok := s.visitsRepo.(base.TransactionalRepository); ok {
		visitsRepo = txRepo.WithTx(tx).(activePort.VisitReadRepository)
	}
	if txRepo, ok := s.supervisorRepo.(base.TransactionalRepository); ok {
		supervisorRepo = txRepo.WithTx(tx).(activePort.GroupSupervisorRepository)
	}
	if txRepo, ok := s.profileRepo.(base.TransactionalRepository); ok {
		profileRepo = txRepo.WithTx(tx).(userPort.ProfileRepository)
	}
	if txRepo, ok := s.substitutionRepo.(base.TransactionalRepository); ok {
		substitutionRepo = txRepo.WithTx(tx).(educationPort.GroupSubstitutionRepository)
	}
	if txRepo, ok := s.substitutionRelationsRepo.(base.TransactionalRepository); ok {
		substitutionRelationsRepo = txRepo.WithTx(tx).(educationPort.GroupSubstitutionRelationsRepository)
	}

	// Return a new service with the transaction
	return &userContextService{
		accountRepo:               accountRepo,
		personRepo:                personRepo,
		staffRepo:                 staffRepo,
		teacherRepo:               teacherRepo,
		studentRepo:               studentRepo,
		educationGroupRepo:        educationGroupRepo,
		activityGroupRepo:         activityGroupRepo,
		activeGroupRepo:           activeGroupRepo,
		visitsRepo:                visitsRepo,
		supervisorRepo:            supervisorRepo,
		profileRepo:               profileRepo,
		substitutionRepo:          substitutionRepo,
		substitutionRelationsRepo: substitutionRelationsRepo,
		db:                        s.db,
		txHandler:                 s.txHandler.WithTx(tx),
	}
}
