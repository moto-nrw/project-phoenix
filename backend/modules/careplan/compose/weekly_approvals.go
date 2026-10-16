package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

func NewWeeklyApprovals(people ports.WeeklyApprovalPeople, arrivals ports.WeeklyApprovalArrivals, pickups ports.WeeklyApprovalPickups) (carerequests.WeeklyApprovals, error) {
	if people == nil || arrivals == nil || pickups == nil {
		return nil, errors.New("schedule: care request apply dependencies not configured")
	}
	return &application.WeeklyApprovals{People: people, Arrivals: arrivals, Pickups: pickups}, nil
}
