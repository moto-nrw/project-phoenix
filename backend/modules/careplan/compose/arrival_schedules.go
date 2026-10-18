package compose

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/uptrace/bun"
)

// ArrivalScheduleRecords is the owner's storage capability used by arrival commands.
type ArrivalScheduleRecords interface {
	FindArrivalSchedule(context.Context, int64) (careplan.ArrivalSchedule, error)
	ListArrivalSchedules(context.Context, careplan.StudentScheduleFilter) ([]careplan.ArrivalSchedule, error)
	CreateArrivalSchedule(context.Context, careplan.ArrivalSchedule) (careplan.ArrivalSchedule, error)
	DeleteArrivalSchedule(context.Context, int64) error
	DeleteArrivalSchedulesByStudent(context.Context, int64) error
	UpsertArrivalSchedule(context.Context, careplan.ArrivalSchedule) (careplan.ArrivalSchedule, error)
	FindArrivalException(context.Context, int64, bool) (careplan.ArrivalException, error)
	ListArrivalExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.ArrivalException, error)
	CreateArrivalException(context.Context, careplan.ArrivalException) (careplan.ArrivalException, error)
	DeleteArrivalException(context.Context, int64) error
	DeleteArrivalExceptionsByStudent(context.Context, int64) error
	UpdateArrivalException(context.Context, careplan.ArrivalException) error
	FindArrivalNote(context.Context, int64) (careplan.ArrivalNote, error)
	ListArrivalNotes(context.Context, careplan.StudentScheduleFilter) ([]careplan.ArrivalNote, error)
	CreateArrivalNote(context.Context, careplan.ArrivalNote) (careplan.ArrivalNote, error)
	DeleteArrivalNote(context.Context, int64) error
	DeleteArrivalNotesByStudent(context.Context, int64) error
	UpdateArrivalNote(context.Context, careplan.ArrivalNote) error
}

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

func NewArrivalSchedules(db *bun.DB, records ArrivalScheduleRecords, baselines careplan.ArrivalBaselineReader, rules ArrivalScheduleRules, deps ArrivalScheduleDependencies) (careplan.ArrivalScheduleService, error) {
	if db == nil || records == nil {
		return nil, errors.New("arrival schedules: database and records are required")
	}
	return application.NewArrivalSchedules(arrivalScheduleRecords(records), arrivalExceptionRecords(records), arrivalNoteRecords(records), baselines, rules, pickupScheduleTransaction{excusalTransaction{db}}, deps.Students, deps.Classes, deps.ClassExceptions, deps.Logger), nil
}

func arrivalScheduleRecords(r ArrivalScheduleRecords) application.EffectiveRecords[careplan.ArrivalSchedule, *careplan.ArrivalSchedule] {
	return application.NewEffectiveRecords[careplan.ArrivalSchedule, *careplan.ArrivalSchedule](application.RecordFuncs[careplan.ArrivalSchedule]{Find: application.Unlocked(r.FindArrivalSchedule), List: r.ListArrivalSchedules,
		Create: r.CreateArrivalSchedule, Upsert: r.UpsertArrivalSchedule, Remove: r.DeleteArrivalSchedule, RemoveByStudent: r.DeleteArrivalSchedulesByStudent})
}

func arrivalExceptionRecords(r ArrivalScheduleRecords) application.ExceptionRecords[careplan.ArrivalException, *careplan.ArrivalException] {
	return application.NewExceptionRecords[careplan.ArrivalException, *careplan.ArrivalException](application.RecordFuncs[careplan.ArrivalException]{Find: r.FindArrivalException, List: r.ListArrivalExceptions,
		Create: r.CreateArrivalException, Update: r.UpdateArrivalException, Remove: r.DeleteArrivalException, RemoveByStudent: r.DeleteArrivalExceptionsByStudent})
}

func arrivalNoteRecords(r ArrivalScheduleRecords) application.NoteRecords[careplan.ArrivalNote, *careplan.ArrivalNote] {
	return application.NewNoteRecords[careplan.ArrivalNote, *careplan.ArrivalNote](application.RecordFuncs[careplan.ArrivalNote]{Find: application.Unlocked(r.FindArrivalNote), List: r.ListArrivalNotes,
		Create: r.CreateArrivalNote, Update: r.UpdateArrivalNote, Remove: r.DeleteArrivalNote, RemoveByStudent: r.DeleteArrivalNotesByStudent})
}
