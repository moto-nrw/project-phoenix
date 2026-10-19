package groups

import "context"

func NewPublicSubstitutionModule(deps SubstitutionDependencies) SubstitutionModule {
	return &substitutionBoundary{module: NewSubstitutionModule(deps)}
}

type substitutionBoundary struct{ module SubstitutionModule }

func (s *substitutionBoundary) Overview(ctx context.Context, caller SubstitutionCaller, query OverviewQuery) (*OverviewResult, error) {
	result, err := s.module.Overview(ctx, caller, query)
	return result, PublicGroupError(err)
}
func (s *substitutionBoundary) Assign(ctx context.Context, caller SubstitutionCaller, request Assignment) (*AssignmentResult, error) {
	result, err := s.module.Assign(ctx, caller, request)
	return result, PublicGroupError(err)
}
func (s *substitutionBoundary) End(ctx context.Context, caller SubstitutionCaller, request EndRequest) error {
	return PublicGroupError(s.module.End(ctx, caller, request))
}
