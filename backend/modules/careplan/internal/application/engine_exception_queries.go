package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

// StatusQueries are the schedule, exception, note and status-day reads and
// the departure exception writes the exception queries serve directly over
// the Care Plan store.
type StatusQueries interface {
	CountStudentScheduleRows(ctx context.Context, studentID int64) (int, domain.OperationStats, error)
	DeleteDepartureException(ctx context.Context, id int64) (domain.OperationStats, error)
	FindArrivalException(ctx context.Context, id int64, lock bool) (careplan.ArrivalException, bool, domain.OperationStats, error)
	FindArrivalNote(ctx context.Context, id int64) (careplan.ArrivalNote, bool, domain.OperationStats, error)
	FindArrivalSchedule(ctx context.Context, id int64) (careplan.ArrivalSchedule, bool, domain.OperationStats, error)
	FindPickupException(ctx context.Context, id int64, lock bool) (careplan.PickupException, bool, domain.OperationStats, error)
	FindPickupNote(ctx context.Context, id int64) (careplan.PickupNote, bool, domain.OperationStats, error)
	FindPickupSchedule(ctx context.Context, id int64) (careplan.PickupSchedule, bool, domain.OperationStats, error)
	FindStudentStatusDay(ctx context.Context, id int64, activeOnly bool) (careplan.StudentStatusDay, bool, domain.RequestStoreStats, error)
	ListArrivalExceptions(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.ArrivalException, domain.OperationStats, error)
	ListArrivalNotes(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.ArrivalNote, domain.OperationStats, error)
	ListArrivalSchedules(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.ArrivalSchedule, domain.OperationStats, error)
	ListDepartureExceptions(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.DepartureException, domain.OperationStats, error)
	ListPickupExceptions(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.PickupException, domain.OperationStats, error)
	ListPickupNotes(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.PickupNote, domain.OperationStats, error)
	ListPickupSchedules(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.PickupSchedule, domain.OperationStats, error)
	ListStudentStatusDays(ctx context.Context, filter careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, domain.RequestStoreStats, error)
	SaveDepartureException(ctx context.Context, v careplan.DepartureException) (careplan.DepartureException, domain.OperationStats, error)
}

// ExceptionStore is the Care Plan store behind the exception queries: the
// application service's store and the direct reads.
type ExceptionStore interface {
	ports.Store
	StatusQueries
}

// Owner reads keep transaction acquisition in the application layer. The raw
// records receive the same ambient unit of work as their calling workflow.
func scopedRecordRead[T any](ctx context.Context, read func(context.Context) (T, domain.OperationStats, error)) (value T, stats domain.OperationStats, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error { value, stats, err = read(txCtx); return err })
	return
}
func scopedRecordFind[T any](ctx context.Context, read func(context.Context) (T, bool, domain.OperationStats, error)) (value T, found bool, stats domain.OperationStats, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error { value, found, stats, err = read(txCtx); return err })
	return
}
