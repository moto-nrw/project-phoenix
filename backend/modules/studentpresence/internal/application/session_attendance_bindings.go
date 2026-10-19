package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

// plannedRoster adapts the public roster port to the application port.
type plannedRoster struct{ query studentpresence.PlannedRoster }

func (r plannedRoster) ListPlannedParticipants(ctx context.Context, filter ports.PlannedRosterFilter) ([]ports.PlannedParticipant, error) {
	values, err := r.query.ListPlannedParticipants(ctx, studentpresence.PlannedRosterFilter(filter))
	if err != nil {
		return nil, err
	}
	result := make([]ports.PlannedParticipant, 0, len(values))
	for _, value := range values {
		result = append(result, ports.PlannedParticipant(value))
	}
	return result, nil
}

type carePlanDirectory struct {
	query studentpresence.SessionCarePlanDirectory
}

func (d carePlanDirectory) FindPickupException(ctx context.Context, id int64) (*ports.PickupException, error) {
	value, err := d.query.FindPickupException(ctx, id)
	if value == nil || err != nil {
		return nil, err
	}
	result := ports.PickupException(*value)
	return &result, nil
}

func (d carePlanDirectory) ListPickupExceptions(ctx context.Context, filter ports.PickupExceptionFilter) ([]ports.PickupException, error) {
	values, err := d.query.ListPickupExceptions(ctx, studentpresence.SessionPickupExceptionFilter(filter))
	result := make([]ports.PickupException, 0, len(values))
	for _, value := range values {
		result = append(result, ports.PickupException(value))
	}
	return result, err
}

func (d carePlanDirectory) FindStudentStatusDay(ctx context.Context, id int64, activeOnly bool) (*ports.StudentStatusDay, error) {
	value, err := d.query.FindStudentStatusDay(ctx, id, activeOnly)
	if value == nil || err != nil {
		return nil, err
	}
	result := ports.StudentStatusDay(*value)
	return &result, nil
}

func (d carePlanDirectory) ListStudentStatusDays(ctx context.Context, filter ports.StudentStatusDayFilter) ([]ports.StudentStatusDay, error) {
	values, err := d.query.ListStudentStatusDays(ctx, studentpresence.SessionStudentStatusDayFilter(filter))
	result := make([]ports.StudentStatusDay, 0, len(values))
	for _, value := range values {
		result = append(result, ports.StudentStatusDay(value))
	}
	return result, err
}

func NewPlannedRoster(query studentpresence.PlannedRoster) ports.PlannedRoster {
	return plannedRoster{query: query}
}
func NewCarePlanDirectory(query studentpresence.SessionCarePlanDirectory) ports.CarePlanDirectory {
	return carePlanDirectory{query: query}
}
