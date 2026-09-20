package compose

import (
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

type RequestDiffPeople = ports.RequestDiffPeople
type RequestDiffArrivals = ports.RequestDiffArrivals
type RequestDiffPickups = ports.RequestDiffPickups

func NewRequestDiffs(people RequestDiffPeople, arrivals RequestDiffArrivals, pickups RequestDiffPickups, logger *slog.Logger) (carerequests.Diffs, error) {
	if people == nil || arrivals == nil || pickups == nil {
		return nil, errors.New("care request diffs: people, arrivals, and pickups are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &application.RequestDiffs{People: people, Arrivals: arrivals, Pickups: pickups, Logger: logger}, nil
}
