package postgres

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

func (s *Store) CreatePickupException(ctx context.Context, v careplan.PickupException) (careplan.PickupException, domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "create pickup exception")
	if err != nil {
		return careplan.PickupException{}, domain.OperationStats{}, err
	}
	row := pickupExceptionFromPublic(v)
	row.TenantID = tid
	stats, err := createStudentScheduleRow(ctx, db.NewInsert().Model(&row).ModelTableExpr(`schedule.student_pickup_exceptions`), "create pickup exception")
	return pickupExceptionToPublic(row), stats, err
}
func (s *Store) UpdatePickupException(ctx context.Context, v careplan.PickupException) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "update pickup exception")
	if err != nil {
		return domain.OperationStats{}, err
	}
	row := pickupExceptionFromPublic(v)
	row.TenantID = tid
	row.UpdatedAt = time.Now()
	return execGuarded(ctx, db.NewUpdate().Model(&row).ModelTableExpr(`schedule.student_pickup_exceptions AS "student_pickup_exception"`).Where(`"student_pickup_exception".id = ?`, row.ID).Where(`"student_pickup_exception".tenant_id = ?`, tid), "update pickup exception", careplan.ErrStudentScheduleNotFound)
}
func (s *Store) DeletePickupException(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "delete pickup exception")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewDelete().TableExpr(`schedule.student_pickup_exceptions AS "student_pickup_exception"`).Where(`"student_pickup_exception".id = ?`, id).Where(`"student_pickup_exception".tenant_id = ?`, tid), "delete pickup exception")
}
func (s *Store) DeletePickupExceptionsByStudent(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "delete pickup exceptions by student")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewDelete().TableExpr(`schedule.student_pickup_exceptions AS "student_pickup_exception"`).Where(`"student_pickup_exception".tenant_id = ?`, tid).Where(`"student_pickup_exception".student_id = ?`, id), "delete pickup exceptions by student")
}
func (s *Store) DeletePickupExceptionsBefore(ctx context.Context, d careplan.Date) (domain.OperationStats, error) {
	db, tid, err := s.databaseForWrite(ctx, "delete old pickup exceptions")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewDelete().TableExpr(`schedule.student_pickup_exceptions AS "student_pickup_exception"`).Where(`"student_pickup_exception".tenant_id = ?`, tid).Where(`"student_pickup_exception".exception_date < ?`, calendarDate(d)), "delete past pickup exceptions")
}

func pickupExceptionFromPublic(v careplan.PickupException) pickupExceptionRow {
	v.NormalizeWallClockTimes()
	return pickupExceptionRow{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, ExceptionDate: calendarDate(v.ExceptionDate), PickupTime: v.PickupTime, Reason: v.Reason, ExcusedFrom: v.ExcusedFrom, ExcusedReason: v.ExcusedReason, ExcusedCreatedBy: v.ExcusedCreatedBy, ExcusedOwnsPickupTime: v.ExcusedOwnsPickupTime, ExcusedAuto: v.ExcusedAuto, Source: v.Source, CreatedBy: v.CreatedBy, CreatedByGuardian: v.CreatedByGuardian}
}
func pickupExceptionToPublic(v pickupExceptionRow) careplan.PickupException {
	return careplan.PickupException{ID: v.ID, TenantID: v.TenantID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, StudentID: v.StudentID, ExceptionDate: careplan.Date(v.ExceptionDate), PickupTime: v.PickupTime, Reason: v.Reason, ExcusedFrom: v.ExcusedFrom, ExcusedReason: v.ExcusedReason, ExcusedCreatedBy: v.ExcusedCreatedBy, ExcusedOwnsPickupTime: v.ExcusedOwnsPickupTime, ExcusedAuto: v.ExcusedAuto, Source: v.Source, CreatedBy: v.CreatedBy, CreatedByGuardian: v.CreatedByGuardian}
}
