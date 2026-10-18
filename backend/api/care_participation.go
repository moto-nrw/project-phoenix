package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CareParticipationOf binds the People Directory group read to the Care Plan
// participation decision the root composed before the person directory.
func CareParticipationOf(participation careplan.CareParticipation) peoplePorts.CareParticipationResolver {
	return func(ctx context.Context, studentIDs []int64, on, today timezone.Date) (map[int64]bool, error) {
		resolution, err := participation.ResolveListParticipation(ctx, studentIDs, on, today, careplan.GroupRosterIncludesPendingWithdrawals)
		if err != nil {
			return nil, err
		}
		return resolution.ParticipatingIDs, nil
	}
}
