package postgres

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/domain"
	"github.com/uptrace/bun"
)

func (s *Store) CreateArrivalExceptions(ctx context.Context, values []careplan.ArrivalException) (domain.OperationStats, error) {
	if len(values) == 0 {
		return domain.OperationStats{}, nil
	}
	db, tid, err := s.databaseForWrite(ctx, "create arrival exceptions")
	if err != nil {
		return domain.OperationStats{}, err
	}
	rows := make([]arrivalExceptionRow, len(values))
	for i, value := range values {
		rows[i] = arrivalExceptionFromPublic(value)
		rows[i].TenantID = tid
	}
	return execAny(ctx, db.NewInsert().Model(&rows).ModelTableExpr(`schedule.student_arrival_exceptions`).Returning("NULL"), "create arrival exceptions")
}

func (s *Store) DeleteArrivalExceptions(ctx context.Context, ids []int64) (domain.OperationStats, error) {
	if len(ids) == 0 {
		return domain.OperationStats{}, nil
	}
	db, tid, err := s.databaseForWrite(ctx, "delete arrival exceptions")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewDelete().TableExpr(`schedule.student_arrival_exceptions`).Where("tenant_id = ?", tid).Where("id IN (?)", bun.List(ids)), "delete arrival exceptions")
}

func (s *Store) CreatePickupExceptions(ctx context.Context, values []careplan.PickupException) (domain.OperationStats, error) {
	if len(values) == 0 {
		return domain.OperationStats{}, nil
	}
	db, tid, err := s.databaseForWrite(ctx, "create pickup exceptions")
	if err != nil {
		return domain.OperationStats{}, err
	}
	rows := make([]pickupExceptionRow, len(values))
	for i, value := range values {
		rows[i] = pickupExceptionFromPublic(value)
		rows[i].TenantID = tid
	}
	return execAny(ctx, db.NewInsert().Model(&rows).ModelTableExpr(`schedule.student_pickup_exceptions`).Returning("NULL"), "create pickup exceptions")
}

func (s *Store) DeletePickupExceptions(ctx context.Context, ids []int64) (domain.OperationStats, error) {
	if len(ids) == 0 {
		return domain.OperationStats{}, nil
	}
	db, tid, err := s.databaseForWrite(ctx, "delete pickup exceptions")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewDelete().TableExpr(`schedule.student_pickup_exceptions`).Where("tenant_id = ?", tid).Where("id IN (?)", bun.List(ids)), "delete pickup exceptions")
}
