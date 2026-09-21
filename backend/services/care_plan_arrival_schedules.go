package services

import (
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/uptrace/bun"
)

// NewArrivalSchedules binds supplied native capabilities without constructing
// a repository or service factory.
func NewArrivalSchedules(db *bun.DB, records careplanCompose.ArrivalScheduleRecords, people peopledirectory.Capability, baselines careplan.ArrivalBaselineReader, classes careplanCompose.ArrivalClassPlans, exceptions careplan.ClassArrivalExceptions, logger *slog.Logger) (careplan.ArrivalScheduleService, error) {
	if people == nil {
		return nil, errors.New("arrival schedules: people directory is required")
	}
	students := arrivalPlanStudents{people}
	rules := careplanCompose.NewArrivalScheduleRules(students, classes)
	return careplanCompose.NewArrivalSchedules(db, records, baselines, rules, careplanCompose.ArrivalScheduleDependencies{
		Students: students, Classes: classes, ClassExceptions: exceptions, Logger: logger,
	})
}
