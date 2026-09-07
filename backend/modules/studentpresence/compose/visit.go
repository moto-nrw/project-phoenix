package compose

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func visitToPublic(row *ports.Visit) studentpresence.Visit {
	if row == nil {
		return studentpresence.Visit{}
	}
	return studentpresence.Visit{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StudentID: row.StudentID, ActiveGroupID: row.ActiveGroupID, EntryTime: row.EntryTime, ExitTime: row.ExitTime,
	}
}

func visitFromPublic(value studentpresence.Visit) *ports.Visit {
	row := &ports.Visit{StudentID: value.StudentID, ActiveGroupID: value.ActiveGroupID, EntryTime: value.EntryTime, ExitTime: value.ExitTime}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}

func visitRowsToPublic(rows []*ports.Visit) []studentpresence.Visit {
	result := make([]studentpresence.Visit, 0, len(rows))
	for _, row := range rows {
		result = append(result, visitToPublic(row))
	}
	return result
}

func (e engine) FindVisit(ctx context.Context, id int64) (*studentpresence.Visit, error) {
	row, err := e.Service.FindVisit(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, studentpresence.ErrVisitNotFound
	}
	if err != nil {
		return nil, err
	}
	result := visitToPublic(row)
	return &result, nil
}

func (e engine) ListVisits(ctx context.Context, filter studentpresence.VisitFilter) ([]studentpresence.Visit, error) {
	rows, err := e.Service.ListVisits(ctx, ports.VisitFilter(filter))
	if err != nil {
		return nil, err
	}
	return visitRowsToPublic(rows), nil
}

func (e engine) RecordVisit(ctx context.Context, value studentpresence.Visit) (studentpresence.Visit, error) {
	row := visitFromPublic(value)
	err := e.Service.RecordVisit(ctx, row)
	return visitToPublic(row), err
}

func (e engine) ReviseVisit(ctx context.Context, value studentpresence.Visit) (studentpresence.Visit, error) {
	row := visitFromPublic(value)
	err := e.Service.ReviseVisit(ctx, row)
	return visitToPublic(row), err
}

func (e engine) CloseVisits(ctx context.Context, ids []int64, at time.Time) ([]studentpresence.Visit, error) {
	rows, err := e.Service.CloseVisits(ctx, ids, at)
	if err != nil {
		return nil, err
	}
	return visitRowsToPublic(rows), nil
}
