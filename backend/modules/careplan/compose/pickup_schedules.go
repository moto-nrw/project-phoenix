package compose

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	compositionBinding "github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	carePlanBindingPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
	"github.com/uptrace/bun"
)

type PickupBulkStudents = ports.PickupBulkStudents
type PickupBulkStudent = ports.PickupBulkStudent

func NewPickupSchedules(db *bun.DB, records carePlanBindingPorts.PickupScheduleRecords, baselines careplan.PickupBaselineReader, auto careplan.PickupAutoExcusal, students PickupBulkStudents, logger *slog.Logger) (careplan.PickupScheduleService, error) {
	if db == nil || records == nil || baselines == nil {
		return nil, errors.New("pickup schedules: database, records, and baselines are required")
	}
	return application.NewPickupSchedules(application.PickupScheduleRecords(records), application.PickupExceptionRecords(records),
		application.PickupNoteRecords(records), pickupScheduleTransaction{excusalTransaction: excusalTransaction{db: db}}, baselines, auto, students, records, logger), nil
}

type pickupScheduleTransaction struct{ excusalTransaction }

func (t pickupScheduleTransaction) LockStudentAndExceptionDay(ctx context.Context, id int64, date string) error {
	return careplanning.LockStudentAndExceptionDay(ctx, t.db, id, date)
}

func (t pickupScheduleTransaction) IsNotFound(err error) bool {
	return compositionBinding.IsPickupScheduleNotFound(err, t.excusalTransaction.IsNotFound)
}
