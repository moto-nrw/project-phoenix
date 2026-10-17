package compose

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/uptrace/bun"
)

// PickupScheduleRecords is the owner's storage capability used by pickup commands.
type PickupScheduleRecords interface {
	FindPickupSchedule(context.Context, int64) (careplan.PickupSchedule, error)
	ListPickupSchedules(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupSchedule, error)
	CreatePickupSchedule(context.Context, careplan.PickupSchedule) (careplan.PickupSchedule, error)
	DeletePickupSchedule(context.Context, int64) error
	DeletePickupSchedulesByStudent(context.Context, int64) error
	UpsertPickupSchedule(context.Context, careplan.PickupSchedule) (careplan.PickupSchedule, error)
	FindPickupException(context.Context, int64, bool) (careplan.PickupException, error)
	ListPickupExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error)
	CreatePickupException(context.Context, careplan.PickupException) (careplan.PickupException, error)
	DeletePickupException(context.Context, int64) error
	DeletePickupExceptionsByStudent(context.Context, int64) error
	UpdatePickupException(context.Context, careplan.PickupException) error
	FindPickupNote(context.Context, int64) (careplan.PickupNote, error)
	ListPickupNotes(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupNote, error)
	CreatePickupNote(context.Context, careplan.PickupNote) (careplan.PickupNote, error)
	DeletePickupNote(context.Context, int64) error
	DeletePickupNotesByStudent(context.Context, int64) error
	UpdatePickupNote(context.Context, careplan.PickupNote) error
	careplan.DepartureExceptions
}

type PickupBulkStudents = ports.PickupBulkStudents
type PickupBulkStudent = ports.PickupBulkStudent

func NewPickupSchedules(db *bun.DB, records PickupScheduleRecords, baselines careplan.PickupBaselineReader, auto careplan.PickupAutoExcusal, students PickupBulkStudents, logger *slog.Logger) (careplan.PickupScheduleService, error) {
	if db == nil || records == nil || baselines == nil {
		return nil, errors.New("pickup schedules: database, records, and baselines are required")
	}
	return application.NewPickupSchedules(pickupScheduleRecords(records), pickupExceptionRecords(records),
		pickupNoteRecords(records), pickupScheduleTransaction{excusalTransaction{db}}, baselines, auto, students, records, logger), nil
}

type pickupScheduleTransaction struct{ excusalTransaction }

func (t pickupScheduleTransaction) LockStudentAndExceptionDay(ctx context.Context, id int64, date string) error {
	return careplanning.LockStudentAndExceptionDay(ctx, t.db, id, date)
}
func (t pickupScheduleTransaction) IsNotFound(err error) bool {
	return errors.Is(err, careplan.ErrStudentScheduleNotFound) || t.excusalTransaction.IsNotFound(err)
}

func pickupScheduleRecords(r PickupScheduleRecords) application.EffectiveRecords[careplan.PickupSchedule, *careplan.PickupSchedule] {
	return application.NewEffectiveRecords[careplan.PickupSchedule, *careplan.PickupSchedule](application.RecordFuncs[careplan.PickupSchedule]{Find: application.Unlocked(r.FindPickupSchedule), List: r.ListPickupSchedules,
		Create: r.CreatePickupSchedule, Upsert: r.UpsertPickupSchedule, Remove: r.DeletePickupSchedule, RemoveByStudent: r.DeletePickupSchedulesByStudent})
}

func pickupExceptionRecords(r PickupScheduleRecords) application.ExceptionRecords[careplan.PickupException, *careplan.PickupException] {
	return application.NewExceptionRecords[careplan.PickupException, *careplan.PickupException](application.RecordFuncs[careplan.PickupException]{Find: r.FindPickupException, List: r.ListPickupExceptions,
		Create: r.CreatePickupException, Update: r.UpdatePickupException, Remove: r.DeletePickupException, RemoveByStudent: r.DeletePickupExceptionsByStudent})
}

func pickupNoteRecords(r PickupScheduleRecords) application.NoteRecords[careplan.PickupNote, *careplan.PickupNote] {
	return application.NewNoteRecords[careplan.PickupNote, *careplan.PickupNote](application.RecordFuncs[careplan.PickupNote]{Find: application.Unlocked(r.FindPickupNote), List: r.ListPickupNotes,
		Create: r.CreatePickupNote, Update: r.UpdatePickupNote, Remove: r.DeletePickupNote, RemoveByStudent: r.DeletePickupNotesByStudent})
}
