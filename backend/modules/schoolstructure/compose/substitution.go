package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/application/groups"
)

// NewSubstitutionModule exposes only neutral results and errors from the
// private application, including failures from retained storage ports.
func NewSubstitutionModule(deps SubstitutionDependencies) SubstitutionModule {
	return &substitutionBoundary{module: groups.NewSubstitutionModule(deps)}
}

type substitutionBoundary struct{ module SubstitutionModule }

func (s *substitutionBoundary) Overview(ctx context.Context, caller SubstitutionCaller, query OverviewQuery) (*OverviewResult, error) {
	result, err := s.module.Overview(ctx, caller, query)
	return result, publicGroupError(err)
}
func (s *substitutionBoundary) Assign(ctx context.Context, caller SubstitutionCaller, request Assignment) (*AssignmentResult, error) {
	result, err := s.module.Assign(ctx, caller, request)
	return result, publicGroupError(err)
}
func (s *substitutionBoundary) End(ctx context.Context, caller SubstitutionCaller, request EndRequest) error {
	return publicGroupError(s.module.End(ctx, caller, request))
}
