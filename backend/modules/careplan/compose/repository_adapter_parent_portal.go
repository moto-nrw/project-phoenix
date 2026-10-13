package compose

import (
	"context"
	"slices"

	parentRepo "github.com/moto-nrw/project-phoenix/database/repositories/parent"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	"github.com/uptrace/bun"
)

// The retained guardian-portal repositories (database/repositories/parent)
// list an account's children, the enrollment phases it may apply to and its
// applications. They name the parent models, so they are built by the Care
// Plan composition (#2743). The People Directory and Organisation & Tenancy
// facts they read reach them through the consumer-owned ports below; the
// composition root binds those owners.

// ParentPortalStudent is the People Directory student fact the guardian portal
// reads, alumni included.
type ParentPortalStudent struct {
	ID            int64
	TenantID      int64
	PersonID      int64
	SchoolClass   string
	Status        string
	EnrolledFrom  string
	EnrolledUntil string
}

// ParentPortalGuardianLink is one student-guardian relationship of an
// account, with the granted parents-portal permissions.
type ParentPortalGuardianLink struct {
	TenantID          int64
	StudentID         int64
	GuardianProfileID int64
	Permissions       []string
}

// ParentPortalPerson is a person the People Directory still shows; a
// soft-deleted person is absent.
type ParentPortalPerson struct {
	ID        int64
	FirstName string
	LastName  string
}

// ParentPortalDirectory is the People Directory read of the guardian portal.
// Students and guardian links are read across every tenant the admin
// transaction can see.
type ParentPortalDirectory interface {
	ListParentPortalStudents(ctx context.Context, ids []int64) ([]ParentPortalStudent, error)
	ListParentPortalGuardianLinks(ctx context.Context, accountID int64) ([]ParentPortalGuardianLink, error)
	ListParentPortalPersons(ctx context.Context, ids []int64) ([]ParentPortalPerson, error)
}

// ParentPortalDependencies bind the guardian-portal repositories.
type ParentPortalDependencies struct {
	DB        *bun.DB
	Directory ParentPortalDirectory
	// AccountSchools lists the schools an account holds an active mapping
	// at; the mapping is the only safe scope of a cross-tenant parent read.
	AccountSchools func(context.Context, int64) ([]int64, error)
	// Phases and Applications are Enrollment's open phases and the account's
	// applications.
	Phases       parentRepo.PhaseQueries
	Applications parentRepo.EnrollmentCommands
	// Accounts resolves the address behind an account.
	Accounts parentRepo.AccountDirectory
}

// ParentPortalRepositories are the guardian-portal repositories over their
// owners.
type ParentPortalRepositories struct {
	ParentChild             parentModels.ChildRepository
	ParentEnrollablePhase   parentModels.EnrollablePhaseRepository
	ParentEnrollmentRequest parentModels.EnrollmentRequestRepository
}

// NewParentPortalRepositories builds the guardian-portal repositories with
// the students, guardian links and person names read through the directory.
// The school names are bound by WithSchools.
func NewParentPortalRepositories(deps ParentPortalDependencies) ParentPortalRepositories {
	phases := parentRepo.NewEnrollablePhaseRepository(deps.Phases, deps.AccountSchools)
	requests := NewParentEnrollmentRequestRepository(deps.DB, deps.Applications, deps.Accounts)
	if repo, ok := phases.(*parentRepo.EnrollablePhaseRepository); ok {
		repo.BindStudentDirectory(parentStudentDirectory{deps.Directory})
		repo.BindGuardianDirectory(parentGuardianDirectory{deps.Directory})
	}
	if repo, ok := requests.(*parentRepo.EnrollmentRequestRepository); ok {
		repo.BindGuardianDirectory(parentGuardianDirectory{deps.Directory})
	}
	return ParentPortalRepositories{
		ParentChild:             NewParentChildRepository(deps.AccountSchools, deps.Directory),
		ParentEnrollablePhase:   personEnrollablePhaseRepository{EnrollablePhaseRepository: phases, persons: deps.Directory},
		ParentEnrollmentRequest: requests,
	}
}

// NewParentEnrollmentRequestRepository is the account's applications over the
// ambient tenant transaction, without the guardian links bound.
func NewParentEnrollmentRequestRepository(db *bun.DB, applications parentRepo.EnrollmentCommands, accounts parentRepo.AccountDirectory) parentModels.EnrollmentRequestRepository {
	return parentRepo.NewEnrollmentRequestRepository(parentRepo.RuntimeFunc(TenantAmbientDatabase(db)), applications, accounts)
}

// NewParentChildRepository lists a guardian account's children across its
// active schools, with the students, guardian links and person names read
// through the directory. The school names are bound by WithSchools.
func NewParentChildRepository(accountSchools func(context.Context, int64) ([]int64, error), directory ParentPortalDirectory) parentModels.ChildRepository {
	child := parentRepo.NewChildRepository(accountSchools)
	if repo, ok := child.(*parentRepo.ChildRepository); ok {
		repo.BindStudentDirectory(parentStudentDirectory{directory})
		repo.BindGuardianDirectory(parentGuardianDirectory{directory})
	}
	return personChildRepository{ChildRepository: child, persons: directory}
}

type parentStudentDirectory struct{ directory ParentPortalDirectory }

func (d parentStudentDirectory) ListStudentsByID(ctx context.Context, ids []int64) ([]parentRepo.DirectoryStudent, error) {
	students, err := d.directory.ListParentPortalStudents(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]parentRepo.DirectoryStudent, 0, len(students))
	for _, student := range students {
		result = append(result, parentRepo.DirectoryStudent(student))
	}
	return result, nil
}

type parentGuardianDirectory struct{ directory ParentPortalDirectory }

func (d parentGuardianDirectory) ListGuardianLinksByAccount(ctx context.Context, accountID int64) ([]parentRepo.DirectoryGuardianLink, error) {
	links, err := d.directory.ListParentPortalGuardianLinks(ctx, accountID)
	if err != nil {
		return nil, err
	}
	result := make([]parentRepo.DirectoryGuardianLink, 0, len(links))
	for _, link := range links {
		result = append(result, parentRepo.DirectoryGuardianLink(link))
	}
	return result, nil
}

// parentPortalPersonsByID resolves the persons the directory still shows for
// ids, keyed by person ID. Duplicates and unset IDs are dropped before the
// lookup.
func parentPortalPersonsByID(ctx context.Context, directory ParentPortalDirectory, ids []int64) (map[int64]ParentPortalPerson, error) {
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, found := seen[id]; !found {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return map[int64]ParentPortalPerson{}, nil
	}
	values, err := directory.ListParentPortalPersons(ctx, unique)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]ParentPortalPerson, len(values))
	for _, value := range values {
		result[value.ID] = value
	}
	return result, nil
}

// personChildRepository attaches the children's names and applies the
// deleted-person filter the cross-tenant join used to carry.
type personChildRepository struct {
	parentModels.ChildRepository
	persons ParentPortalDirectory
}

func (r personChildRepository) ListByAccount(ctx context.Context, accountID int64) ([]*parentModels.ChildSummary, error) {
	children, err := r.ChildRepository.ListByAccount(ctx, accountID)
	if err != nil || len(children) == 0 {
		return children, err
	}
	kept, err := attachChildPersons(ctx, r.persons, children)
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(kept, func(left, right *parentModels.ChildSummary) int {
		if order := compareParentPortalStrings(left.FirstName, right.FirstName); order != 0 {
			return order
		}
		return compareParentPortalStrings(left.LastName, right.LastName)
	})
	return kept, nil
}

func (r personChildRepository) FindForAccount(ctx context.Context, accountID, studentID int64) (*parentModels.ChildSummary, error) {
	child, err := r.ChildRepository.FindForAccount(ctx, accountID, studentID)
	if err != nil || child == nil {
		return child, err
	}
	kept, err := attachChildPersons(ctx, r.persons, []*parentModels.ChildSummary{child})
	if err != nil {
		return nil, err
	}
	if len(kept) == 0 {
		// A soft-deleted person hides the child, as the join filter did.
		return nil, nil
	}
	return kept[0], nil
}

func attachChildPersons(ctx context.Context, directory ParentPortalDirectory, children []*parentModels.ChildSummary) ([]*parentModels.ChildSummary, error) {
	ids := make([]int64, 0, len(children))
	for _, child := range children {
		ids = append(ids, child.PersonID)
	}
	persons, err := parentPortalPersonsByID(ctx, directory, ids)
	if err != nil {
		return nil, err
	}
	kept := make([]*parentModels.ChildSummary, 0, len(children))
	for _, child := range children {
		person, found := persons[child.PersonID]
		if !found {
			continue
		}
		child.FirstName = person.FirstName
		child.LastName = person.LastName
		kept = append(kept, child)
	}
	return kept, nil
}

// personEnrollablePhaseRepository decides the existing_students eligibility:
// the phase stays only when at least one candidate person still exists (a
// soft-deleted child does not count), mirroring the submit-time matcher.
type personEnrollablePhaseRepository struct {
	parentModels.EnrollablePhaseRepository
	persons ParentPortalDirectory
}

func (r personEnrollablePhaseRepository) ListEnrollable(ctx context.Context, accountID int64) ([]*parentModels.EnrollablePhase, error) {
	phases, err := r.EnrollablePhaseRepository.ListEnrollable(ctx, accountID)
	if err != nil || len(phases) == 0 {
		return phases, err
	}
	ids := make([]int64, 0, len(phases))
	for _, phase := range phases {
		ids = append(ids, phase.EnrolledSubmitPersonIDs...)
	}
	persons, err := parentPortalPersonsByID(ctx, r.persons, ids)
	if err != nil {
		return nil, err
	}
	result := make([]*parentModels.EnrollablePhase, 0, len(phases))
	for _, phase := range phases {
		if phase.Audience == "existing_students" && !anyParentPortalPersonExists(persons, phase.EnrolledSubmitPersonIDs) {
			continue
		}
		result = append(result, phase)
	}
	return result, nil
}

func (r personEnrollablePhaseRepository) GuardianSubmitStatus(ctx context.Context, accountID, tenantID int64) (*parentModels.GuardianSubmitStatus, error) {
	status, err := r.EnrollablePhaseRepository.GuardianSubmitStatus(ctx, accountID, tenantID)
	if err != nil || status == nil {
		return status, err
	}
	persons, err := parentPortalPersonsByID(ctx, r.persons, status.EnrolledSubmitPersonIDs)
	if err != nil {
		return nil, err
	}
	status.HasEnrolledSubmitPermission = anyParentPortalPersonExists(persons, status.EnrolledSubmitPersonIDs)
	return status, nil
}

func anyParentPortalPersonExists(persons map[int64]ParentPortalPerson, ids []int64) bool {
	for _, id := range ids {
		if _, found := persons[id]; found {
			return true
		}
	}
	return false
}

func compareParentPortalStrings(left, right string) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}
