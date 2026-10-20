package application

import (
	context "context"
	errors "errors"
	time "time"

	domain "github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/domain"
)

// ErrDashboardNotComposed is returned by every dashboard collaborator a
// device-only graph left unset. It fails loudly instead of rendering a
// partial screen from missing facts.
var ErrDashboardNotComposed = errors.New("devicefleet: display dashboard sources are not composed")

type UncomposedDashboard struct{}

func (UncomposedDashboard) ListActiveSessions(context.Context) ([]domain.ActiveSession, error) {
	return nil, ErrDashboardNotComposed
}
func (UncomposedDashboard) ListActivityTemplates(context.Context) ([]domain.ActivityTemplate, error) {
	return nil, ErrDashboardNotComposed
}
func (UncomposedDashboard) ListPlannedActivities(context.Context, time.Time) ([]domain.PlannedActivity, error) {
	return nil, ErrDashboardNotComposed
}
func (UncomposedDashboard) ListPickupTimes(context.Context, []int64, time.Time) ([]domain.PickupTime, error) {
	return nil, ErrDashboardNotComposed
}
func (UncomposedDashboard) ListOpenVisits(context.Context) ([]domain.PresentVisit, error) {
	return nil, ErrDashboardNotComposed
}
func (UncomposedDashboard) ListPresentStudents(context.Context, time.Time) ([]int64, error) {
	return nil, ErrDashboardNotComposed
}
func (UncomposedDashboard) School(context.Context, int64) (domain.School, error) {
	return domain.School{}, ErrDashboardNotComposed
}
func (UncomposedDashboard) DisplayEnabled(context.Context, int64) (bool, error) {
	return false, ErrDashboardNotComposed
}
