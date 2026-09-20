package services

import (
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/uptrace/bun"
)

// NewPickupSchedules binds supplied native capabilities without constructing
// a repository or service factory.
func NewPickupSchedules(db *bun.DB, records careplanCompose.PickupScheduleRecords, people peopledirectory.Capability, baselines careplan.PickupBaselineReader, auto careplan.PickupAutoExcusal, logger *slog.Logger) (careplan.PickupScheduleService, error) {
	if people == nil {
		return nil, errors.New("pickup schedules: people directory is required")
	}
	return careplanCompose.NewPickupSchedules(db, records, baselines, auto, pickupPlanStudents{people}, logger)
}
