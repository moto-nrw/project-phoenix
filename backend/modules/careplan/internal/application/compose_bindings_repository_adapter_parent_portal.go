package application

import (
	"context"

	parentModels "github.com/moto-nrw/project-phoenix/modules/careplan/internal/rows"
	carePlanBindingPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
)

// ParentPortalRepositories are the guardian-portal repositories over their
// owners.
type ParentPortalRepositories struct {
	ParentChild             parentModels.ChildRepository
	ParentEnrollablePhase   parentModels.EnrollablePhaseRepository
	ParentEnrollmentRequest parentModels.EnrollmentRequestRepository
}

// ParentChildLinks lists the children an account reaches through a guardian
// child repository.
type ParentChildLinks struct{ Children parentModels.ChildRepository }

// ListByAccount returns the account's children; the slice is nil when the
// repository returns none.
func (l ParentChildLinks) ListByAccount(ctx context.Context, accountID int64) ([]carePlanBindingPorts.ParentChildLink, error) {
	children, err := l.Children.ListByAccount(ctx, accountID)
	if children == nil {
		return nil, err
	}
	result := make([]carePlanBindingPorts.ParentChildLink, 0, len(children))
	for _, child := range children {
		result = append(result, carePlanBindingPorts.ParentChildLink{
			StudentID: child.StudentID, GuardianProfileID: child.GuardianProfileID, TenantID: child.TenantID, SchoolName: child.SchoolName,
		})
	}
	return result, err
}
