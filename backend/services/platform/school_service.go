package platform

import (
	"context"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
)

// SchoolService exposes school lookups to the api layer (issue #584: handlers
// must not hold repositories). CONTRACT: repository results and errors are
// returned VERBATIM — handlers keep their existing nil/IsDeleted gates and
// error mapping, so responses stay byte-identical.
type SchoolService interface {
	// GetSchoolByID retrieves a school by ID.
	GetSchoolByID(ctx context.Context, id int64) (*platformModels.School, error)

	// GetSchoolBySlug retrieves a school by tenant slug.
	GetSchoolBySlug(ctx context.Context, slug string) (*platformModels.School, error)

	// GetSchoolBySubdomain retrieves a school by subdomain.
	GetSchoolBySubdomain(ctx context.Context, subdomain string) (*platformModels.School, error)

	// ListPublicSchools lists schools visible on public tenant pickers.
	ListPublicSchools(ctx context.Context) ([]platformModels.School, error)

	// ListActiveSchoolsByAccountID lists the active schools an account belongs to.
	ListActiveSchoolsByAccountID(ctx context.Context, accountID int64) ([]platformModels.School, error)
}

type schoolService struct {
	repo platformModels.SchoolRepository
}

// NewSchoolService creates a SchoolService backed by the school repository.
func NewSchoolService(repo platformModels.SchoolRepository) SchoolService {
	return &schoolService{repo: repo}
}

func (s *schoolService) GetSchoolByID(ctx context.Context, id int64) (*platformModels.School, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *schoolService) GetSchoolBySlug(ctx context.Context, slug string) (*platformModels.School, error) {
	return s.repo.FindBySlug(ctx, slug)
}

func (s *schoolService) GetSchoolBySubdomain(ctx context.Context, subdomain string) (*platformModels.School, error) {
	return s.repo.FindBySubdomain(ctx, subdomain)
}

func (s *schoolService) ListPublicSchools(ctx context.Context) ([]platformModels.School, error) {
	return s.repo.ListPublic(ctx)
}

func (s *schoolService) ListActiveSchoolsByAccountID(ctx context.Context, accountID int64) ([]platformModels.School, error) {
	return s.repo.FindActiveByAccountID(ctx, accountID)
}
