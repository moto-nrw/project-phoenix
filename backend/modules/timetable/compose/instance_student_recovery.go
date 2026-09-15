package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func (e engine) LockInstanceStudentAssignments(ctx context.Context, instanceID int64) error {
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		return errors.New("timetable: assignment locks require a transaction")
	}
	return mapError(e.service.LockInstanceStudentAssignments(ctx, instanceID))
}
func (e engine) RestoreInstanceStudentAttendance(ctx context.Context, instanceID int64, rows []timetable.CompletionAttendance) error {
	values := make([]domain.CompletionAttendance, 0, len(rows))
	for _, row := range rows {
		values = append(values, domain.CompletionAttendance(row))
	}
	return mapError(e.service.RestoreInstanceStudentAttendance(ctx, instanceID, values))
}
