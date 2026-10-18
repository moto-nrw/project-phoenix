package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	parentRepo "github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/parentpostgres"
	careplanPostgres "github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	parentModels "github.com/moto-nrw/project-phoenix/modules/careplan/internal/rows"
	"github.com/uptrace/bun"
)

// The retained guardian-portal repositories (database/repositories/parent)
// list an account's children, the enrollment phases it may apply to and its
// applications. They name the parent models, so they are built by the Care
// Plan composition (#2743). The People Directory and Organisation & Tenancy
// facts they read reach them through the consumer-owned ports below; the
// composition root binds those owners.

// ParentPortalDependencies bind the guardian-portal repositories.
type ParentPortalDependencies struct {
	DB        *bun.DB
	Directory careplan.ParentPortalDirectory
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
	phases := parentRepo.NewEnrollablePhaseRepository(deps.Phases, deps.AccountSchools, parentStudentDirectory{deps.Directory}, parentGuardianDirectory{deps.Directory})
	requests := parentRepo.NewEnrollmentRequestRepository(parentRepo.RuntimeFunc(TenantAmbientDatabase(deps.DB)), deps.Applications, deps.Accounts, parentGuardianDirectory{deps.Directory})
	return ParentPortalRepositories{
		ParentChild:             NewParentChildRepository(deps.AccountSchools, deps.Directory),
		ParentEnrollablePhase:   application.WithParentPhasePersons(phases, deps.Directory),
		ParentEnrollmentRequest: requests,
	}
}

// NewParentChildRepository lists a guardian account's children across its
// active schools, with the students, guardian links and person names read
// through the directory. The school names are bound by WithSchools.
func NewParentChildRepository(accountSchools func(context.Context, int64) ([]int64, error), directory careplan.ParentPortalDirectory) parentModels.ChildRepository {
	child := parentRepo.NewChildRepository(accountSchools, parentStudentDirectory{directory}, parentGuardianDirectory{directory})
	return application.WithParentChildPersons(child, directory)
}

type parentStudentDirectory struct {
	directory careplan.ParentPortalDirectory
}

func (d parentStudentDirectory) ListStudentsByID(ctx context.Context, ids []int64) ([]parentRepo.DirectoryStudent, error) {
	students, err := d.directory.ListParentPortalStudents(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]parentRepo.DirectoryStudent, 0, len(students))
	for _, student := range students {
		result = append(result, parentRepo.DirectoryStudent{ID: student.ID, TenantID: student.TenantID, PersonID: student.PersonID, SchoolClass: student.SchoolClass, Status: student.Status, EnrolledFrom: student.EnrolledFrom.String(), EnrolledUntil: student.EnrolledUntil.String()})
	}
	return result, nil
}

type parentGuardianDirectory struct {
	directory careplan.ParentPortalDirectory
}

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

// ParentChildLink is one child a guardian account reaches, with the school it
// attends, as the parent calendar reads it.
type ParentChildLink struct {
	StudentID         int64
	GuardianProfileID int64
	TenantID          int64
	SchoolName        string
}

// ParentChildLinks lists the children an account reaches through a guardian
// child repository.
type ParentChildLinks struct{ children parentModels.ChildRepository }

// NewParentChildLinks reads the links through children.
func NewParentChildLinks(children parentModels.ChildRepository) ParentChildLinks {
	return ParentChildLinks{children: children}
}

// ListByAccount returns the account's children; the slice is nil when the
// repository returns none.
func (l ParentChildLinks) ListByAccount(ctx context.Context, accountID int64) ([]ParentChildLink, error) {
	children, err := l.children.ListByAccount(ctx, accountID)
	if children == nil {
		return nil, err
	}
	result := make([]ParentChildLink, 0, len(children))
	for _, child := range children {
		result = append(result, ParentChildLink{
			StudentID: child.StudentID, GuardianProfileID: child.GuardianProfileID, TenantID: child.TenantID, SchoolName: child.SchoolName,
		})
	}
	return result, err
}

// NewParentChildQueries exposes the retained projection behind the owner's
// public read capability and the supplied security policy.
func NewParentChildQueries(source parentModels.ChildRepository, granted func(any) bool) careplan.ParentPortalChildQueries {
	return careplanPostgres.NewParentChildQueries(source, granted)
}
func NewParentPhaseQueries(source parentModels.EnrollablePhaseRepository) careplan.ParentPortalPhaseQueries {
	return careplanPostgres.NewParentPhaseQueries(source)
}
func NewParentEnrollmentQueries(source parentModels.EnrollmentRequestRepository) careplan.ParentPortalEnrollmentQueries {
	return careplanPostgres.NewParentEnrollmentQueries(source)
}
