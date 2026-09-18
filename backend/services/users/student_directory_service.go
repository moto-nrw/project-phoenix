package users

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// StudentService is what the api layer holds (issue #584: handlers must not
// hold repositories). It is the two halves of the child record together: the
// directory rows People Directory owns, and the companion graph Care Plan
// owns. CONTRACT: results and errors are returned VERBATIM — the handlers keep
// their existing transaction wrappers, validation, and error-to-status
// mapping, so responses stay byte-identical.
type StudentService interface {
	StudentDirectoryService
	StudentCompanionService
}

// StudentDirectoryService is the People Directory half: the child row, its
// locks and the two per-tenant gates. The reads and the locks reach the owner;
// Create, Update and Delete are the remainder still served by the retained
// repository, which #3349 is moving.
type StudentDirectoryService interface {
	// StudentDirectoryReader is the owner's filtered, paginated directory
	// read (#3349); this service only carries it to the handlers.
	StudentDirectoryReader

	// ListSchoolClasses retrieves all distinct non-empty school classes.
	ListSchoolClasses(ctx context.Context) ([]string, error)

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

	// LockClassWritesShared acquires the shared per-tenant class-writes gate.
	// Needed only by callers that take another tenant-wide gate (recurrence)
	// before their first student row lock: the gate has to come first to keep
	// the project-wide order acyclic against a grade transition.
	LockClassWritesShared(ctx context.Context) error

	// GetByIDs retrieves several students in one query, keyed by id.
	GetByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Student, error)
}

type studentService struct {
	// StudentDirectoryReader is the owner capability behind the directory
	// read; embedding it keeps this service out of the filter's way.
	StudentDirectoryReader
	directory     StudentDirectoryLocker
	studentRepo   userModels.StudentRepository
	companionRepo userModels.StudentCompanionRepository
	studentAudit  StudentChangeRecorder
	classes       StudentClassReader
}

// StudentClassReader lists the distinct classes of the tenant's non-alumni
// children; the owner answers it.
type StudentClassReader interface {
	ListSchoolClasses(ctx context.Context) ([]string, error)
}

// NewStudentService creates a StudentService backed by the student-domain
// repositories.
func NewStudentService(
	directory StudentDirectoryAccess,
	classes StudentClassReader,
	studentRepo userModels.StudentRepository,
	companionRepo userModels.StudentCompanionRepository,
	studentAudit StudentChangeRecorder,
) StudentService {
	return &studentService{
		StudentDirectoryReader: directory,
		directory:              directory,
		studentRepo:            studentRepo,
		companionRepo:          companionRepo,
		studentAudit:           studentAudit,
		classes:                classes,
	}
}

func (s *studentService) ListSchoolClasses(ctx context.Context) ([]string, error) {
	return s.classes.ListSchoolClasses(ctx)
}

func (s *studentService) GetByIDForUpdate(ctx context.Context, id int64) (*userModels.Student, error) {
	student, err := s.directory.LockStudent(ctx, id)
	return student, translateMissingStudent("find_by_id_for_update", err)
}

// translateMissingStudent restates the owner's missing child in the error shape
// this service's handlers have always branched on: a DatabaseError wrapping
// both base.ErrNotFound and sql.ErrNoRows. The mapping lives here because the
// composition seam that observes the owner may import neither of those.
func translateMissingStudent(op string, err error) error {
	if !errors.Is(err, userModels.ErrStudentRowMissing) {
		return err
	}
	return &base.DatabaseError{Op: op, Err: errors.Join(base.ErrNotFound, sql.ErrNoRows)}
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
	return s.directory.LockPhotoFeature(ctx)
}

func (s *studentService) LockClassWritesShared(ctx context.Context) error {
	return s.directory.LockClassWritesShared(ctx)
}

func (s *studentService) GetByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Student, error) {
	if len(ids) == 0 {
		return map[int64]*userModels.Student{}, nil
	}
	students, err := s.directory.GetStudentsByID(ctx, ids)
	if err != nil {
		return nil, translateMissingStudent("find_by_ids", err)
	}
	result := make(map[int64]*userModels.Student, len(students))
	for _, student := range students {
		result[student.ID] = student
	}
	return result, nil
}
