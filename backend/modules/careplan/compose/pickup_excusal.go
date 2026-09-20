package compose

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type PartialAbsenceBlocks = ports.PartialAbsenceBlocks
type PartialAbsencePreview = ports.PartialAbsencePreview
type PickupExtensions = ports.PickupExtensions
type PickupDayExtension = ports.PickupDayExtension
type PickupWeekdayExtension = ports.PickupWeekdayExtension

// PickupExcusalRecords is the existing owner persistence capability needed by
// the derived and manual excusal commands.
type PickupExcusalRecords interface {
	FindPickupException(context.Context, int64, bool) (careplan.PickupException, error)
	ListPickupExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error)
	CreatePickupException(context.Context, careplan.PickupException) (careplan.PickupException, error)
	UpdatePickupException(context.Context, careplan.PickupException) error
	DeletePickupException(context.Context, int64) error
	ListStudentStatusDays(context.Context, careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, error)
	ListExcusedAbsenceRequests(context.Context, careplan.ExcusedAbsenceRequestFilter) ([]careplan.ExcusedAbsenceRequest, error)
}

type PickupExcusalDependencies struct {
	DB         *bun.DB
	Records    PickupExcusalRecords
	Baselines  careplan.PickupBaselineReader
	Blocks     PartialAbsenceBlocks
	Preview    PartialAbsencePreview
	Extensions PickupExtensions
}

func NewPickupAutoExcusal(deps PickupExcusalDependencies) (careplan.PickupAutoExcusal, error) {
	if deps.DB == nil || deps.Records == nil || deps.Baselines == nil || deps.Blocks == nil {
		return nil, errors.New("pickup excusal: database, records, baselines, and block commands are required")
	}
	return application.NewPickupAutoExcusal(pickupExcusalRecords{deps.Records}, deps.Baselines,
		deps.Blocks, deps.Preview, deps.Extensions, excusalTransaction{deps.DB}), nil
}

func NewPartialAbsences(db *bun.DB, records PickupExcusalRecords, blocks PartialAbsenceBlocks, syncer careplan.PickupAutoExcusal) (careplan.PartialAbsenceService, error) {
	if db == nil || records == nil || blocks == nil {
		return nil, errors.New("partial absences: database, records, and block commands are required")
	}
	adapter := pickupExcusalRecords{records}
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

type pickupExcusalRecords struct{ PickupExcusalRecords }

func (s pickupExcusalRecords) FindByID(ctx context.Context, id int64) (*careplan.PickupException, error) {
	row, err := s.FindPickupException(ctx, id, false)
	if errors.Is(err, careplan.ErrStudentScheduleNotFound) {
		return nil, careplan.ErrPartialAbsenceNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}
func (s pickupExcusalRecords) FindByStudentIDAndDate(ctx context.Context, id int64, date calendar.Date) (*careplan.PickupException, error) {
	rows, err := s.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, Date: careplan.Date(date)})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}
func (s pickupExcusalRecords) FindUpcomingByStudentID(ctx context.Context, id int64) ([]*careplan.PickupException, error) {
	return s.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, UpcomingFrom: careplan.Date(calendar.TodayDate())})
}
func (s pickupExcusalRecords) FindByStudentIDAndDateRange(ctx context.Context, id int64, from, to calendar.Date) ([]*careplan.PickupException, error) {
	return s.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, From: careplan.Date(from), To: careplan.Date(to)})
}
func (s pickupExcusalRecords) list(ctx context.Context, filter careplan.StudentScheduleFilter) ([]*careplan.PickupException, error) {
	rows, err := s.ListPickupExceptions(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]*careplan.PickupException, 0, len(rows))
	for i := range rows {
		out = append(out, &rows[i])
	}
	return out, nil
}
func (s pickupExcusalRecords) Create(ctx context.Context, row *careplan.PickupException) error {
	row.NormalizeWallClockTimes()
	result, err := s.CreatePickupException(ctx, *row)
	if err == nil {
		*row = result
	}
	return err
}
func (s pickupExcusalRecords) Update(ctx context.Context, row *careplan.PickupException) error {
	row.NormalizeWallClockTimes()
	return s.UpdatePickupException(ctx, *row)
}
func (s pickupExcusalRecords) Delete(ctx context.Context, id int64) error {
	return s.DeletePickupException(ctx, id)
}
func (s pickupExcusalRecords) HasFullDayStatus(ctx context.Context, id int64, date calendar.Date) (bool, error) {
	rows, err := s.ListStudentStatusDays(ctx, careplan.StudentStatusDayFilter{
		StudentIDs: []int64{id}, From: careplan.Date(date), To: careplan.Date(date), ActiveOnly: true,
	})
	return len(rows) > 0, err
}
func (s pickupExcusalRecords) PendingExcusedDates(ctx context.Context, id int64) ([]calendar.Date, error) {
	rows, err := s.ListExcusedAbsenceRequests(ctx, careplan.ExcusedAbsenceRequestFilter{StudentID: id, Statuses: []string{"pending"}})
	if err != nil {
		return nil, err
	}
	var dates []calendar.Date
	for _, row := range rows {
		for _, date := range row.Dates {
			dates = append(dates, calendar.Date(date))
		}
	}
	return dates, nil
}
