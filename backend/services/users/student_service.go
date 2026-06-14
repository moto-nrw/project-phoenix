package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// StudentService exposes student persistence operations to the api layer
// (issue #584: handlers must not hold repositories). CONTRACT: repository
// results and errors are returned VERBATIM — the handlers keep their existing
// transaction wrappers, validation, and error-to-status mapping, so responses
// stay byte-identical. See the rule-8 deviation note in the PR description.
type StudentService interface {
	// ListWithOptions retrieves students matching the query options.
	ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*userModels.Student, error)

	// CountWithOptions counts students matching the query options.
	CountWithOptions(ctx context.Context, options *base.QueryOptions) (int, error)

	// GetByIDForUpdate retrieves a student with SELECT … FOR UPDATE row locking.
	GetByIDForUpdate(ctx context.Context, id int64) (*userModels.Student, error)

	// Create persists a new student.
	Create(ctx context.Context, student *userModels.Student) error

	// Update persists changes to a student.
	Update(ctx context.Context, student *userModels.Student) error

	// Delete removes a student.
	Delete(ctx context.Context, id int64) error

	// LockPhotoFeature acquires the per-tenant photo-feature advisory lock.
	LockPhotoFeature(ctx context.Context) error

	// ListPrivacyConsents retrieves a student's privacy consents.
	ListPrivacyConsents(ctx context.Context, studentID int64) ([]*userModels.PrivacyConsent, error)

	// CreatePrivacyConsent persists a new privacy consent.
	CreatePrivacyConsent(ctx context.Context, consent *userModels.PrivacyConsent) error

	// UpdatePrivacyConsent persists changes to a privacy consent.
	UpdatePrivacyConsent(ctx context.Context, consent *userModels.PrivacyConsent) error

	// ListParentNotes returns a student's parent notes, newest first.
	ListParentNotes(ctx context.Context, studentID int64, limit int) ([]*userModels.StudentParentNote, error)
}

type studentService struct {
	studentRepo        userModels.StudentRepository
	privacyConsentRepo userModels.PrivacyConsentRepository
	parentNoteRepo     userModels.StudentParentNoteRepository
}

// NewStudentService creates a StudentService backed by the student-domain
// repositories.
func NewStudentService(studentRepo userModels.StudentRepository, privacyConsentRepo userModels.PrivacyConsentRepository, parentNoteRepo userModels.StudentParentNoteRepository) StudentService {
	return &studentService{
		studentRepo:        studentRepo,
		privacyConsentRepo: privacyConsentRepo,
		parentNoteRepo:     parentNoteRepo,
	}
}

func (s *studentService) ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*userModels.Student, error) {
	return s.studentRepo.ListWithOptions(ctx, options)
}

func (s *studentService) CountWithOptions(ctx context.Context, options *base.QueryOptions) (int, error) {
	return s.studentRepo.CountWithOptions(ctx, options)
}

func (s *studentService) GetByIDForUpdate(ctx context.Context, id int64) (*userModels.Student, error) {
	return s.studentRepo.FindByIDForUpdate(ctx, id)
}

func (s *studentService) Create(ctx context.Context, student *userModels.Student) error {
	return s.studentRepo.Create(ctx, student)
}

func (s *studentService) Update(ctx context.Context, student *userModels.Student) error {
	return s.studentRepo.Update(ctx, student)
}

func (s *studentService) Delete(ctx context.Context, id int64) error {
	return s.studentRepo.Delete(ctx, id)
}

func (s *studentService) LockPhotoFeature(ctx context.Context) error {
	return s.studentRepo.LockPhotoFeature(ctx)
}

func (s *studentService) ListPrivacyConsents(ctx context.Context, studentID int64) ([]*userModels.PrivacyConsent, error) {
	return s.privacyConsentRepo.FindByStudentID(ctx, studentID)
}

func (s *studentService) CreatePrivacyConsent(ctx context.Context, consent *userModels.PrivacyConsent) error {
	return s.privacyConsentRepo.Create(ctx, consent)
}

func (s *studentService) UpdatePrivacyConsent(ctx context.Context, consent *userModels.PrivacyConsent) error {
	return s.privacyConsentRepo.Update(ctx, consent)
}

func (s *studentService) ListParentNotes(ctx context.Context, studentID int64, limit int) ([]*userModels.StudentParentNote, error) {
	return s.parentNoteRepo.ListByStudent(ctx, studentID, limit)
}
