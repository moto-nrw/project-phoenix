package services

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
)

// StudentServices is the child record's two halves. They have different owners
// since #3350 — People Directory keeps the rows and their locks, Care Plan owns
// the "läuft mit" graph — but the students API holds both, so they travel
// together and are built once.
type StudentServices struct {
	Directory  peopleCompose.StudentService
	Companions careplan.StudentCompanions
}

// careParticipationResolver binds the People Directory group read to the Care
// Plan participation decision (#3350). The directory selects the children of a
// group; which of them still take part in care on a given day is the owner's
// answer, so only the resolved set crosses the seam. Care Plan is composed
// after the person directory, so current reaches it at call time; a graph that
// never bound it reports a configuration error.
func careParticipationResolver(current func() careplan.CareParticipation) peopleCompose.CareParticipationResolver {
	return func(ctx context.Context, studentIDs []int64, on, today timezone.Date) (map[int64]bool, error) {
		lifecycle := current()
		if lifecycle == nil {
			return nil, errors.New("person service: care participation resolver is not configured")
		}
		resolution, err := lifecycle.ResolveListParticipation(ctx, studentIDs, on, today, false)
		if err != nil {
			return nil, err
		}
		return resolution.ParticipatingIDs, nil
	}
}
