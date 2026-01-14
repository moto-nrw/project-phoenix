package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

const (
	// Student service operations
	opGetStudent           = "get student"
	opCreateStudent        = "create student"
	opUpdateStudent        = "update student"
	opDeleteStudent        = "delete student"
	opFindStudentsByGroup  = "find students by group"
	opListStudents         = "list students"
	opCountStudents        = "count students"
	opGetPrivacyConsent    = "get privacy consent"
	opCreatePrivacyConsent = "create privacy consent"
	opUpdatePrivacyConsent = "update privacy consent"
)

// StudentServiceDependencies contains all dependencies required by the student service
type StudentServiceDependencies struct {
	StudentRepo        userModels.StudentRepository
	PrivacyConsentRepo userModels.PrivacyConsentRepository
	DB                 *bun.DB
}

// studentService implements the StudentService interface
type studentService struct {
	studentRepo        userModels.StudentRepository
	privacyConsentRepo userModels.PrivacyConsentRepository
	db                 *bun.DB
	txHandler          *base.TxHandler
}

// NewStudentService creates a new student service
func NewStudentService(deps StudentServiceDependencies) StudentService {
	return &studentService{
		studentRepo:        deps.StudentRepo,
		privacyConsentRepo: deps.PrivacyConsentRepo,
		db:                 deps.DB,
		txHandler:          base.NewTxHandler(deps.DB),
	}
}

// WithTx returns a new service that uses the provided transaction
func (s *studentService) WithTx(tx bun.Tx) interface{} {
	var studentRepo = s.studentRepo
	var privacyConsentRepo = s.privacyConsentRepo

	if txRepo, ok := s.studentRepo.(base.TransactionalRepository); ok {
		studentRepo = txRepo.WithTx(tx).(userModels.StudentRepository)
	}
	if txRepo, ok := s.privacyConsentRepo.(base.TransactionalRepository); ok {
		privacyConsentRepo = txRepo.WithTx(tx).(userModels.PrivacyConsentRepository)
	}

	return &studentService{
		studentRepo:        studentRepo,
		privacyConsentRepo: privacyConsentRepo,
		db:                 s.db,
		txHandler:          s.txHandler.WithTx(tx),
	}
}

// Get retrieves a student by their ID
func (s *studentService) Get(ctx context.Context, id int64) (*userModels.Student, error) {
	student, err := s.studentRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &UsersError{Op: opGetStudent, Err: err}
	}
	if student == nil {
		return nil, &UsersError{Op: opGetStudent, Err: ErrStudentNotFound}
	}
	return student, nil
}

// Create creates a new student
func (s *studentService) Create(ctx context.Context, student *userModels.Student) error {
	if err := student.Validate(); err != nil {
		return &UsersError{Op: opCreateStudent, Err: err}
	}

	if err := s.studentRepo.Create(ctx, student); err != nil {
		return &UsersError{Op: opCreateStudent, Err: err}
	}

	return nil
}

// Update updates an existing student
func (s *studentService) Update(ctx context.Context, student *userModels.Student) error {
	if err := student.Validate(); err != nil {
		return &UsersError{Op: opUpdateStudent, Err: err}
	}

	// Verify student exists
	existing, err := s.studentRepo.FindByID(ctx, student.ID)
	if err != nil {
		return &UsersError{Op: opUpdateStudent, Err: err}
	}
	if existing == nil {
		return &UsersError{Op: opUpdateStudent, Err: ErrStudentNotFound}
	}

	if err := s.studentRepo.Update(ctx, student); err != nil {
		return &UsersError{Op: opUpdateStudent, Err: err}
	}

	return nil
}

// Delete removes a student
func (s *studentService) Delete(ctx context.Context, id int64) error {
	// Verify student exists
	existing, err := s.studentRepo.FindByID(ctx, id)
	if err != nil {
		return &UsersError{Op: opDeleteStudent, Err: err}
	}
	if existing == nil {
		return &UsersError{Op: opDeleteStudent, Err: ErrStudentNotFound}
	}

	if err := s.studentRepo.Delete(ctx, id); err != nil {
		return &UsersError{Op: opDeleteStudent, Err: err}
	}

	return nil
}

// FindByGroupID retrieves students by their group ID
func (s *studentService) FindByGroupID(ctx context.Context, groupID int64) ([]*userModels.Student, error) {
	students, err := s.studentRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return nil, &UsersError{Op: opFindStudentsByGroup, Err: err}
	}
	return students, nil
}

// FindByGroupIDs retrieves students by multiple group IDs
func (s *studentService) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*userModels.Student, error) {
	if len(groupIDs) == 0 {
		return []*userModels.Student{}, nil
	}

	students, err := s.studentRepo.FindByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, &UsersError{Op: opFindStudentsByGroup, Err: err}
	}
	return students, nil
}

// ListWithOptions retrieves students with query options
func (s *studentService) ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*userModels.Student, error) {
	students, err := s.studentRepo.ListWithOptions(ctx, options)
	if err != nil {
		return nil, &UsersError{Op: opListStudents, Err: err}
	}
	return students, nil
}

// CountWithOptions counts students matching the query options
func (s *studentService) CountWithOptions(ctx context.Context, options *base.QueryOptions) (int, error) {
	count, err := s.studentRepo.CountWithOptions(ctx, options)
	if err != nil {
		return 0, &UsersError{Op: opCountStudents, Err: err}
	}
	return count, nil
}

// GetPrivacyConsent retrieves privacy consents for a student
func (s *studentService) GetPrivacyConsent(ctx context.Context, studentID int64) ([]*userModels.PrivacyConsent, error) {
	consents, err := s.privacyConsentRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &UsersError{Op: opGetPrivacyConsent, Err: err}
	}
	return consents, nil
}

// CreatePrivacyConsent creates a new privacy consent
func (s *studentService) CreatePrivacyConsent(ctx context.Context, consent *userModels.PrivacyConsent) error {
	if err := consent.Validate(); err != nil {
		return &UsersError{Op: opCreatePrivacyConsent, Err: err}
	}

	if err := s.privacyConsentRepo.Create(ctx, consent); err != nil {
		return &UsersError{Op: opCreatePrivacyConsent, Err: err}
	}

	return nil
}

// UpdatePrivacyConsent updates an existing privacy consent
func (s *studentService) UpdatePrivacyConsent(ctx context.Context, consent *userModels.PrivacyConsent) error {
	if err := consent.Validate(); err != nil {
		return &UsersError{Op: opUpdatePrivacyConsent, Err: err}
	}

	if err := s.privacyConsentRepo.Update(ctx, consent); err != nil {
		return &UsersError{Op: opUpdatePrivacyConsent, Err: err}
	}

	return nil
}
