package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

type WeeklyApprovalPeople = ports.WeeklyApprovalPeople
type WeeklyApprovalArrivals = ports.WeeklyApprovalArrivals
type WeeklyApprovalPickups = ports.WeeklyApprovalPickups

func NewWeeklyApprovals(people WeeklyApprovalPeople, arrivals WeeklyApprovalArrivals, pickups WeeklyApprovalPickups) (carerequests.WeeklyApprovals, error) {
	if people == nil || arrivals == nil || pickups == nil {
		return nil, errors.New("schedule: care request apply dependencies not configured")
	}
	return &application.WeeklyApprovals{People: people, Arrivals: arrivals, Pickups: pickups}, nil
}
