package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
)

// errDashboardNotComposed is returned by every dashboard collaborator a
// device-only graph left unset. It fails loudly instead of rendering a
// partial screen from missing facts.
var errDashboardNotComposed = errors.New("devicefleet: display dashboard sources are not composed")

type uncomposedDashboard struct{}

func (uncomposedDashboard) ListActiveSessions(context.Context) ([]domain.ActiveSession, error) {
	return nil, errDashboardNotComposed
}

func (uncomposedDashboard) ListActivityTemplates(context.Context) ([]domain.ActivityTemplate, error) {
	return nil, errDashboardNotComposed
}

func (uncomposedDashboard) ListPlannedActivities(context.Context, time.Time) ([]domain.PlannedActivity, error) {
	return nil, errDashboardNotComposed
}

func (uncomposedDashboard) ListPickupTimes(context.Context, []int64, time.Time) ([]domain.PickupTime, error) {
	return nil, errDashboardNotComposed
}

func (uncomposedDashboard) ListOpenVisits(context.Context) ([]domain.PresentVisit, error) {
	return nil, errDashboardNotComposed
}

func (uncomposedDashboard) ListPresentStudents(context.Context, time.Time) ([]int64, error) {
	return nil, errDashboardNotComposed
}

func (uncomposedDashboard) School(context.Context, int64) (domain.School, error) {
	return domain.School{}, errDashboardNotComposed
}

func (uncomposedDashboard) DisplayEnabled(context.Context, int64) (bool, error) {
	return false, errDashboardNotComposed
}
