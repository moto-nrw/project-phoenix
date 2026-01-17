package users

import (
	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	authPort "github.com/moto-nrw/project-phoenix/internal/core/port/auth"
	userPort "github.com/moto-nrw/project-phoenix/internal/core/port/users"
	"github.com/uptrace/bun"
)

const (
	// opGetPerson is the operation name for Get operations
	opGetPerson = "get person"
	// opCreatePerson is the operation name for Create operations
	opCreatePerson = "create person"
	// opUpdatePerson is the operation name for Update operations
	opUpdatePerson = "update person"
	// opDeletePerson is the operation name for Delete operations
	opDeletePerson = "delete person"
	// opLinkToAccount is the operation name for LinkToAccount operations
	opLinkToAccount = "link to account"
	// opLinkToRFIDCard is the operation name for LinkToRFIDCard operations
	opLinkToRFIDCard = "link to RFID card"
	// opGetStudentsByTeacher is the operation name for GetStudentsByTeacher operations
	opGetStudentsByTeacher = "get students by teacher"
	// opGetStudentsWithGroupsByTeacher is the operation name for GetStudentsWithGroupsByTeacher operations
	opGetStudentsWithGroupsByTeacher = "get students with groups by teacher"
)

// PersonServiceDependencies contains all dependencies required by the person service
type PersonServiceDependencies struct {
	// Repository dependencies
	PersonRepo         userPort.PersonRepository
	RFIDRepo           userPort.RFIDCardRepository
	AccountRepo        authPort.AccountRepository
	PersonGuardianRepo userPort.PersonGuardianRepository
	StudentRepo        userPort.StudentRepository
	StaffRepo          userPort.StaffRepository
	TeacherRepo        userPort.TeacherRepository

	// Infrastructure
	DB *bun.DB
}

// personService implements the PersonService interface
type personService struct {
	personRepo         userPort.PersonRepository
	rfidRepo           userPort.RFIDCardRepository
	accountRepo        authPort.AccountRepository
	personGuardianRepo userPort.PersonGuardianRepository
	studentRepo        userPort.StudentRepository
	staffRepo          userPort.StaffRepository
	teacherRepo        userPort.TeacherRepository
	db                 *bun.DB
	txHandler          *base.TxHandler
}

// NewPersonService creates a new person service
func NewPersonService(deps PersonServiceDependencies) PersonService {
	return &personService{
		personRepo:         deps.PersonRepo,
		rfidRepo:           deps.RFIDRepo,
		accountRepo:        deps.AccountRepo,
		personGuardianRepo: deps.PersonGuardianRepo,
		studentRepo:        deps.StudentRepo,
		staffRepo:          deps.StaffRepo,
		teacherRepo:        deps.TeacherRepo,
		db:                 deps.DB,
		txHandler:          base.NewTxHandler(deps.DB),
	}
}

// WithTx returns a new service that uses the provided transaction
func (s *personService) WithTx(tx bun.Tx) any {
	// Get repositories with transaction if they implement the TransactionalRepository interface
	var personRepo = s.personRepo
	var rfidRepo = s.rfidRepo
	var accountRepo = s.accountRepo
	var personGuardianRepo = s.personGuardianRepo
	var studentRepo = s.studentRepo
	var staffRepo = s.staffRepo
	var teacherRepo = s.teacherRepo

	// Try to cast repositories to TransactionalRepository and apply the transaction
	if txRepo, ok := s.personRepo.(base.TransactionalRepository); ok {
		personRepo = txRepo.WithTx(tx).(userPort.PersonRepository)
	}
	if txRepo, ok := s.rfidRepo.(base.TransactionalRepository); ok {
		rfidRepo = txRepo.WithTx(tx).(userPort.RFIDCardRepository)
	}
	if txRepo, ok := s.accountRepo.(base.TransactionalRepository); ok {
		accountRepo = txRepo.WithTx(tx).(authPort.AccountRepository)
	}
	if txRepo, ok := s.personGuardianRepo.(base.TransactionalRepository); ok {
		personGuardianRepo = txRepo.WithTx(tx).(userPort.PersonGuardianRepository)
	}
	if txRepo, ok := s.studentRepo.(base.TransactionalRepository); ok {
		studentRepo = txRepo.WithTx(tx).(userPort.StudentRepository)
	}
	if txRepo, ok := s.staffRepo.(base.TransactionalRepository); ok {
		staffRepo = txRepo.WithTx(tx).(userPort.StaffRepository)
	}
	if txRepo, ok := s.teacherRepo.(base.TransactionalRepository); ok {
		teacherRepo = txRepo.WithTx(tx).(userPort.TeacherRepository)
	}

	// Return a new service with the transaction
	return &personService{
		personRepo:         personRepo,
		rfidRepo:           rfidRepo,
		accountRepo:        accountRepo,
		personGuardianRepo: personGuardianRepo,
		studentRepo:        studentRepo,
		staffRepo:          staffRepo,
		teacherRepo:        teacherRepo,
		db:                 s.db,
		txHandler:          s.txHandler.WithTx(tx),
	}
}
