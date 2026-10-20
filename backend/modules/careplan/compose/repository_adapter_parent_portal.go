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

// NewParentPortalRepositories builds the guardian-portal repositories with
// the students, guardian links and person names read through the directory.
// The school names are bound by WithSchools.
func NewParentPortalRepositories(deps ParentPortalDependencies) ParentPortalRepositories {
	phases := parentRepo.NewEnrollablePhaseRepository(deps.Phases, deps.AccountSchools, parentRepo.ParentStudentDirectory{Directory: deps.Directory}, parentRepo.ParentGuardianDirectory{Directory: deps.Directory})
	requests := parentRepo.NewEnrollmentRequestRepository(parentRepo.RuntimeFunc(careplanPostgres.TenantAmbientDatabase(deps.DB)), deps.Applications, deps.Accounts, parentRepo.ParentGuardianDirectory{Directory: deps.Directory})
	return ParentPortalRepositories{ParentPortalRepositories: application.ParentPortalRepositories{
		ParentChild:             NewParentChildRepository(deps.AccountSchools, deps.Directory),
		ParentEnrollablePhase:   application.WithParentPhasePersons(phases, deps.Directory),
		ParentEnrollmentRequest: requests,
	}}
}

// NewParentChildRepository lists a guardian account's children across its
// active schools, with the students, guardian links and person names read
// through the directory. The school names are bound by WithSchools.
func NewParentChildRepository(accountSchools func(context.Context, int64) ([]int64, error), directory careplan.ParentPortalDirectory) parentModels.ChildRepository {
	child := parentRepo.NewChildRepository(accountSchools, parentRepo.ParentStudentDirectory{Directory: directory}, parentRepo.ParentGuardianDirectory{Directory: directory})
	return application.WithParentChildPersons(child, directory)
}

// NewParentChildLinks reads the links through children.
func NewParentChildLinks(children parentModels.ChildRepository) ParentChildLinks {
	return ParentChildLinks{ParentChildLinks: application.ParentChildLinks{Children: children}}
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

type ParentChildLinks struct {
	application.ParentChildLinks
}

type ParentPortalRepositories struct {
	application.ParentPortalRepositories
}
