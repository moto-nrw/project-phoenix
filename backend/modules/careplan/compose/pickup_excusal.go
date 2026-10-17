package compose

import (
	"context"
	"database/sql"
	"errors"

	carePlanPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type PartialAbsenceBlocks = ports.PartialAbsenceBlocks
type PartialAbsencePreview = ports.PartialAbsencePreview
type PickupExtensions = ports.PickupExtensions
type PickupDayExtension = ports.PickupDayExtension
type PickupWeekdayExtension = ports.PickupWeekdayExtension
type PickupStudentLocks = ports.PickupStudentLocks

type PickupExcusalDependencies struct {
	DB         *bun.DB
	Records    carePlanPorts.PickupExcusalRecords
	Baselines  careplan.PickupBaselineReader
	Blocks     PartialAbsenceBlocks
	Preview    PartialAbsencePreview
	Extensions PickupExtensions
}

func NewPickupAutoExcusal(deps PickupExcusalDependencies) (careplan.PickupAutoExcusal, error) {
	if deps.DB == nil || deps.Records == nil || deps.Baselines == nil || deps.Blocks == nil {
		return nil, errors.New("pickup excusal: database, records, baselines, and block commands are required")
	}
	return application.NewPickupAutoExcusal(application.NewPickupExcusalRows(deps.Records), deps.Baselines,
		deps.Blocks, deps.Preview, deps.Extensions, excusalTransaction{deps.DB}), nil
}

// LockWeeklyPickupStudents binds Care Plan's lock order of a weekly pickup
// approval to the student locks of the People Directory.
func LockWeeklyPickupStudents(students PickupStudentLocks) func(context.Context, []int64) error {
	return func(ctx context.Context, studentIDs []int64) error {
		return application.LockWeeklyPickupStudents(ctx, students, studentIDs)
	}
}

func NewPartialAbsences(db *bun.DB, records carePlanPorts.PickupExcusalRecords, blocks PartialAbsenceBlocks, syncer careplan.PickupAutoExcusal) (careplan.PartialAbsenceService, error) {
	if db == nil || records == nil || blocks == nil {
		return nil, errors.New("partial absences: database, records, and block commands are required")
	}
	adapter := application.NewPickupExcusalRows(records)
	return application.NewPartialAbsences(adapter, adapter, blocks, syncer, excusalTransaction{db}), nil
}

type excusalTransaction struct{ db *bun.DB }

func (t excusalTransaction) TenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }
func (t excusalTransaction) WithinTenant(ctx context.Context, fn func(context.Context) error) error {
	return tenant.WithTenantTx(ctx, t.db, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error { return fn(txCtx) })
}
func (t excusalTransaction) LockStudent(ctx context.Context, id int64) error {
	return careplanning.LockStudent(ctx, t.db, id)
}
func (t excusalTransaction) LockStudentAndExceptionDay(ctx context.Context, id int64, date string) error {
	err := careplanning.LockStudentAndExceptionDay(ctx, t.db, id, date)
	if errors.Is(err, careplanning.ErrStudentNotFound) {
		return careplan.ErrPartialAbsenceNotFound
	}
	return err
}
func (t excusalTransaction) IsNotFound(err error) bool {
	return errors.Is(err, careplanning.ErrStudentNotFound) || errors.Is(err, sql.ErrNoRows)
}
