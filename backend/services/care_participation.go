package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan/legacy/carelifecycle"
	"github.com/moto-nrw/project-phoenix/services/users"
)

// StudentServices is the child record's two halves. They have different owners
// since #3350 — People Directory keeps the rows and their locks, Care Plan owns
// the "läuft mit" graph — but the students API holds both, so they travel
// together and are built once.
type StudentServices struct {
	Directory  users.StudentService
	Companions carelifecycle.StudentCompanionService
}

// careParticipationResolver binds the People Directory group read to the Care
// Plan participation decision (#3350). The directory selects the children of a
// group; which of them still take part in care on a given day is the owner's
// answer, so only the resolved set crosses the seam.
func careParticipationResolver(lifecycle carelifecycle.CareLifecycleService) users.CareParticipationResolver {
	return func(ctx context.Context, studentIDs []int64, on, today timezone.Date) (map[int64]bool, error) {
		resolution, err := lifecycle.ResolveListParticipation(ctx, studentIDs, on, today, false)
		if err != nil {
			return nil, err
		}
		return resolution.ParticipatingIDs, nil
	}
}
