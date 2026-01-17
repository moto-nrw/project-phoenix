package education

import (
	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	educationPort "github.com/moto-nrw/project-phoenix/internal/core/port/education"
	facilitiesPort "github.com/moto-nrw/project-phoenix/internal/core/port/facilities"
	usersPort "github.com/moto-nrw/project-phoenix/internal/core/port/users"
	"github.com/uptrace/bun"
)

// service implements the Education Service interface
// and acts as the coordinator for education-related operations.
type service struct {
	groupRepo                 educationPort.GroupRepository
	groupTeacherRepo          educationPort.GroupTeacherRepository
	substitutionRepo          educationPort.GroupSubstitutionRepository
	substitutionRelationsRepo educationPort.GroupSubstitutionRelationsRepository
	roomRepo                  facilitiesPort.RoomRepository
	teacherRepo               usersPort.TeacherRepository
	staffRepo                 usersPort.StaffRepository
	db                        *bun.DB
	txHandler                 *base.TxHandler
}

// NewService creates a new education service instance
func NewService(
	groupRepo educationPort.GroupRepository,
	groupTeacherRepo educationPort.GroupTeacherRepository,
	substitutionRepo educationPort.GroupSubstitutionRepository,
	substitutionRelationsRepo educationPort.GroupSubstitutionRelationsRepository,
	roomRepo facilitiesPort.RoomRepository,
	teacherRepo usersPort.TeacherRepository,
	staffRepo usersPort.StaffRepository,
	db *bun.DB,
) Service {
	return &service{
		groupRepo:                 groupRepo,
		groupTeacherRepo:          groupTeacherRepo,
		substitutionRepo:          substitutionRepo,
		substitutionRelationsRepo: substitutionRelationsRepo,
		roomRepo:                  roomRepo,
		teacherRepo:               teacherRepo,
		staffRepo:                 staffRepo,
		db:                        db,
		txHandler:                 base.NewTxHandler(db),
	}
}

// WithTx returns a new service that uses the provided transaction
func (s *service) WithTx(tx bun.Tx) any {
	// Get repositories with transaction if they implement the TransactionalRepository interface
	var groupRepo = s.groupRepo
	var groupTeacherRepo = s.groupTeacherRepo
	var substitutionRepo = s.substitutionRepo
	var substitutionRelationsRepo = s.substitutionRelationsRepo
	var roomRepo = s.roomRepo
	var teacherRepo = s.teacherRepo
	var staffRepo = s.staffRepo

	// Try to cast repositories to TransactionalRepository and apply the transaction
	if txRepo, ok := s.groupRepo.(base.TransactionalRepository); ok {
		groupRepo = txRepo.WithTx(tx).(educationPort.GroupRepository)
	}
	if txRepo, ok := s.groupTeacherRepo.(base.TransactionalRepository); ok {
		groupTeacherRepo = txRepo.WithTx(tx).(educationPort.GroupTeacherRepository)
	}
	if txRepo, ok := s.substitutionRepo.(base.TransactionalRepository); ok {
		substitutionRepo = txRepo.WithTx(tx).(educationPort.GroupSubstitutionRepository)
	}
	if txRepo, ok := s.substitutionRelationsRepo.(base.TransactionalRepository); ok {
		substitutionRelationsRepo = txRepo.WithTx(tx).(educationPort.GroupSubstitutionRelationsRepository)
	}
	if txRepo, ok := s.roomRepo.(base.TransactionalRepository); ok {
		roomRepo = txRepo.WithTx(tx).(facilitiesPort.RoomRepository)
	}
	if txRepo, ok := s.teacherRepo.(base.TransactionalRepository); ok {
		teacherRepo = txRepo.WithTx(tx).(usersPort.TeacherRepository)
	}
	if txRepo, ok := s.staffRepo.(base.TransactionalRepository); ok {
		staffRepo = txRepo.WithTx(tx).(usersPort.StaffRepository)
	}

	// Return a new service with the transaction
	return &service{
		groupRepo:                 groupRepo,
		groupTeacherRepo:          groupTeacherRepo,
		substitutionRepo:          substitutionRepo,
		substitutionRelationsRepo: substitutionRelationsRepo,
		roomRepo:                  roomRepo,
		teacherRepo:               teacherRepo,
		staffRepo:                 staffRepo,
		db:                        s.db,
		txHandler:                 s.txHandler.WithTx(tx),
	}
}
