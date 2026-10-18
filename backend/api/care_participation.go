package api

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CareParticipationResolver binds the People Directory group read to the Care
// Plan participation decision (#3350). The directory selects the children of a
// group; which of them still take part in care on a given day is the owner's
// answer, so only the resolved set crosses the seam. Care Plan is composed
// after the person directory, so current reaches it at call time; a graph that
// never bound it reports a configuration error.
func CareParticipationResolver(current func() careplan.CareParticipation) peoplePorts.CareParticipationResolver {
	return func(ctx context.Context, studentIDs []int64, on, today timezone.Date) (map[int64]bool, error) {
		lifecycle := current()
		if lifecycle == nil {
			return nil, errors.New("person service: care participation resolver is not configured")
		}
		resolution, err := lifecycle.ResolveListParticipation(ctx, studentIDs, on, today, careplan.GroupRosterIncludesPendingWithdrawals)
		if err != nil {
			return nil, err
		}
		return resolution.ParticipatingIDs, nil
	}
}
