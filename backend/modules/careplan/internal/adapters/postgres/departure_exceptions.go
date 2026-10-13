package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/uptrace/bun"
)

type departureExceptionRow struct {
	bun.BaseModel `bun:"table:student_departure_exceptions,alias:student_departure_exception"`
	ID            int64        `bun:"id,pk,autoincrement"`
	TenantID      int64        `bun:"tenant_id,notnull"`
	CreatedAt     time.Time    `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time    `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID     int64        `bun:"student_id,notnull"`
	ExceptionDate calendarDate `bun:"exception_date,notnull,type:date"`
	DepartureMode string       `bun:"departure_mode,notnull"`
	Reason        *string      `bun:"reason"`
	CreatedBy     *int64       `bun:"created_by"`
}

func (s *Store) ListDepartureExceptions(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.DepartureException, domain.OperationStats, error) {
	db, tid, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []departureExceptionRow{}
	if f.IDs != nil && len(f.IDs) == 0 || f.StudentIDs != nil && len(f.StudentIDs) == 0 {
		return []careplan.DepartureException{}, domain.OperationStats{}, nil
	}
	query := db.NewSelect().Model(&rows).ModelTableExpr(`schedule.student_departure_exceptions AS "student_departure_exception"`).Where(`"student_departure_exception".tenant_id = ?`, tid)
	if f.IDs != nil {
		query = query.Where(`"student_departure_exception".id IN (?)`, bun.List(f.IDs))
	}
	if f.StudentIDs != nil {
		query = query.Where(`"student_departure_exception".student_id IN (?)`, bun.List(f.StudentIDs))
	}
	if !f.Date.IsZero() {
		query = query.Where(`"student_departure_exception".exception_date = ?`, calendarDate(f.Date))
	}
	if !f.From.IsZero() {
		query = query.Where(`"student_departure_exception".exception_date >= ?`, calendarDate(f.From))
	}
	if !f.To.IsZero() {
		query = query.Where(`"student_departure_exception".exception_date <= ?`, calendarDate(f.To))
	}
	if !f.UpcomingFrom.IsZero() {
		query = query.Where(`"student_departure_exception".exception_date >= ?`, calendarDate(f.UpcomingFrom))
	}
	query = query.OrderExpr(`"student_departure_exception".exception_date ASC, "student_departure_exception".student_id ASC`)
	if f.LockForUpdate {
		query = query.For("UPDATE")
	}
	stats, err := finishStudentScheduleList(ctx, query, &rows, "list departure exceptions")
	return mapRows(rows, departureExceptionToPublic), stats, err
}

// SaveDepartureException writes the one value of the child's day. The caller
// holds the owner's care-day lock, so the upsert never races another writer
// of the same day; ON CONFLICT only spares a separate read.
func (s *Store) SaveDepartureException(ctx context.Context, v careplan.DepartureException) (careplan.DepartureException, domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "save departure exception")
	if err != nil {
		return careplan.DepartureException{}, domain.OperationStats{}, err
	}
	row := departureExceptionRow{
		TenantID: tid, StudentID: v.StudentID, ExceptionDate: calendarDate(v.ExceptionDate),
		DepartureMode: v.DepartureMode, Reason: v.Reason, CreatedBy: v.CreatedBy,
	}
	query := db.NewInsert().Model(&row).ModelTableExpr(`schedule.student_departure_exceptions`).
		On(`CONFLICT (tenant_id, student_id, exception_date) DO UPDATE`).
		Set(`departure_mode = EXCLUDED.departure_mode`).
		Set(`reason = EXCLUDED.reason`).
		Set(`created_by = EXCLUDED.created_by`).
		Set(`updated_at = NOW()`)
	stats, err := createStudentScheduleRow(ctx, query, "save departure exception")
	return departureExceptionToPublic(row), stats, err
}

func (s *Store) DeleteDepartureException(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "delete departure exception")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewDelete().TableExpr(`schedule.student_departure_exceptions AS "student_departure_exception"`).Where(`"student_departure_exception".id = ?`, id).Where(`"student_departure_exception".tenant_id = ?`, tid), "delete departure exception")
}

func departureExceptionToPublic(v departureExceptionRow) careplan.DepartureException {
	return careplan.DepartureException{
		ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID,
		ExceptionDate: careplan.Date(v.ExceptionDate), DepartureMode: v.DepartureMode, Reason: v.Reason, CreatedBy: v.CreatedBy,
	}
}
