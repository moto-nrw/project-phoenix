package compose

import (
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	carePlanBindingPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
	"github.com/uptrace/bun"
)

type ArrivalScheduleRules = ports.ArrivalScheduleRules
type ArrivalRuleStudents = ports.ArrivalRuleStudents
type ArrivalBulkStudents = ports.ArrivalBulkStudents
type ArrivalBulkStudent = ports.ArrivalBulkStudent
type ArrivalClassPlans = ports.ArrivalClassPlans
type ArrivalClassPlan = ports.ArrivalClassPlan

func NewArrivalScheduleRules(students ArrivalRuleStudents, classes ArrivalClassPlans) ArrivalScheduleRules {
	return application.NewArrivalScheduleRules(students, classes)
}

type ArrivalScheduleDependencies struct {
	Students        ArrivalBulkStudents
	Classes         ArrivalClassPlans
	ClassExceptions careplan.ClassArrivalExceptions
	Logger          *slog.Logger
}

func NewArrivalSchedules(db *bun.DB, records carePlanBindingPorts.ArrivalScheduleRecords, baselines careplan.ArrivalBaselineReader, rules ArrivalScheduleRules, deps ArrivalScheduleDependencies) (careplan.ArrivalScheduleService, error) {
	if db == nil || records == nil {
		return nil, errors.New("arrival schedules: database and records are required")
	}
	return application.NewArrivalSchedules(application.ArrivalScheduleRecords(records), application.ArrivalExceptionRecords(records), application.ArrivalNoteRecords(records), baselines, rules, pickupScheduleTransaction{excusalTransaction: excusalTransaction{db: db}}, deps.Students, deps.Classes, deps.ClassExceptions, deps.Logger), nil
}
