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

	// ListPrivacyConsents retrieves a student's privacy consents.
	ListPrivacyConsents(ctx context.Context, studentID int64) ([]*userModels.PrivacyConsent, error)

	// CreatePrivacyConsent persists a new privacy consent.
	CreatePrivacyConsent(ctx context.Context, consent *userModels.PrivacyConsent) error

	// UpdatePrivacyConsent persists changes to a privacy consent.
	UpdatePrivacyConsent(ctx context.Context, consent *userModels.PrivacyConsent) error

	// ListCompanions returns the children this child walks home with
	// ("läuft mit"), folded per companion with their weekdays and names.
	ListCompanions(ctx context.Context, studentID int64) ([]userModels.CompanionLink, error)

	// ReplaceCompanions makes the given links the child's complete companion
	// set, validating each companion before writing.
	ReplaceCompanions(ctx context.Context, studentID int64, links []userModels.CompanionLink) error

	// CompanionIDsForWeekday bulk-resolves, per student, who they walk home
	// with on the given weekday (1..5). Drives the Kindersuche grouping.
	CompanionIDsForWeekday(ctx context.Context, studentIDs []int64, weekday int) (map[int64][]int64, error)
}

type studentService struct {
	studentRepo        userModels.StudentRepository
	privacyConsentRepo userModels.PrivacyConsentRepository
	companionRepo      userModels.StudentCompanionRepository
}

// NewStudentService creates a StudentService backed by the student-domain
// repositories.
func NewStudentService(
	studentRepo userModels.StudentRepository,
	privacyConsentRepo userModels.PrivacyConsentRepository,
	companionRepo userModels.StudentCompanionRepository,
) StudentService {
	return &studentService{
		studentRepo:        studentRepo,
		privacyConsentRepo: privacyConsentRepo,
		companionRepo:      companionRepo,
	}
}

func (s *studentService) ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*userModels.Student, error) {
	return s.studentRepo.ListWithOptions(ctx, options)
}

func (s *studentService) CountWithOptions(ctx context.Context, options *base.QueryOptions) (int, error) {
	return s.studentRepo.CountWithOptions(ctx, options)
}

func (s *studentService) ListSchoolClasses(ctx context.Context) ([]string, error) {
	return s.studentRepo.ListSchoolClasses(ctx)
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

// MaxStudentCompanions caps how many children one child may be linked to. A
// Laufgemeinschaft is a handful of neighbours walking home together; a request
// with more than this is a client bug or an attempt to use the field as a group
// list, and both should fail loudly rather than produce an unreadable grouping.
const MaxStudentCompanions = 10

func (s *studentService) ListCompanions(ctx context.Context, studentID int64) ([]userModels.CompanionLink, error) {
	if studentID <= 0 {
		return nil, ErrStudentNotFound
	}
	return s.companionRepo.ListLinksForStudent(ctx, studentID)
}

// ReplaceCompanions validates the submitted "läuft mit" list and writes it as
// the child's complete companion set.
//
// The write is symmetric by construction: each link becomes one undirected edge,
// so adding Tom to Lina's card is the same row that shows Lina on Tom's card.
// The flip side is that replacing Lina's list only touches edges that TOUCH
// Lina — a link between Tom and Mia is left alone.
func (s *studentService) ReplaceCompanions(ctx context.Context, studentID int64, links []userModels.CompanionLink) error {
	if studentID <= 0 {
		return ErrStudentNotFound
	}
	if len(links) > MaxStudentCompanions {
		return ErrTooManyCompanions
	}

	// The child itself must exist in this tenant before we hang links off it.
	subject, err := s.studentRepo.FindByID(ctx, studentID)
	if err != nil {
		return err
	}
	if subject == nil {
		return ErrStudentNotFound
	}

	edges := make([]*userModels.StudentCompanion, 0, len(links)*len(userModels.PickupDayOrder))
	companionIDs := make([]int64, 0, len(links))
	seen := make(map[int64]bool, len(links))

	for _, link := range links {
		if link.CompanionStudentID == studentID {
			return userModels.ErrCompanionSelfLink
		}
		if seen[link.CompanionStudentID] {
			return ErrDuplicateCompanion
		}
		seen[link.CompanionStudentID] = true
		companionIDs = append(companionIDs, link.CompanionStudentID)

		if len(link.Weekdays) == 0 {
			return ErrCompanionWeekdayRequired
		}
		seenDays := make(map[int]bool, len(link.Weekdays))
		for _, day := range link.Weekdays {
			weekday, err := userModels.CompanionWeekdayNumber(day)
			if err != nil {
				return err
			}
			if seenDays[weekday] {
				continue
			}
			seenDays[weekday] = true

			edge, err := userModels.NewStudentCompanion(studentID, link.CompanionStudentID, weekday)
			if err != nil {
				return err
			}
			edges = append(edges, edge)
		}
	}

	// Every companion has to be a real child of this tenant. FindByID/FindByIDs
	// are tenant-filtered, so an id from another school simply does not come
	// back — which is exactly the check we want before persisting a link.
	if len(companionIDs) > 0 {
		found, err := s.studentRepo.FindByIDs(ctx, companionIDs)
		if err != nil {
			return err
		}
		for _, id := range companionIDs {
			if found[id] == nil {
				return ErrCompanionNotFound
			}
		}
	}

	return s.companionRepo.ReplaceForStudent(ctx, studentID, edges)
}

func (s *studentService) CompanionIDsForWeekday(ctx context.Context, studentIDs []int64, weekday int) (map[int64][]int64, error) {
	return s.companionRepo.CompanionIDsForWeekday(ctx, studentIDs, weekday)
}
