package repositories

import (
	"context"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/uptrace/bun"
)

type recoveryAssignments struct {
	capability timetable.InstanceStudentCapability
}

// NewActivityRecoveryRepository composes the remaining presence recovery
// adapter with the same Timetable provider used by normal attendance writes.
func NewActivityRecoveryRepository(db *bun.DB, assignments scheduleModels.InstanceStudentRepository) scheduleModels.ActivityRecoveryRepository {
	owner, ok := assignments.(timetableInstanceStudentRepository)
	if !ok {
		panic("activity recovery: timetable assignment adapter is required")
	}
	return scheduleRepo.NewActivityRecoveryRepository(db, recoveryAssignments{capability: owner.timetable})
}

func (r recoveryAssignments) LockAttendance(ctx context.Context, instanceID int64) error {
	return r.capability.LockInstanceStudentAssignments(ctx, instanceID)
}
func (r recoveryAssignments) RestoreAttendance(ctx context.Context, instanceID int64, rows []scheduleModels.CompletionAttendanceSnapshot) error {
	values := make([]timetable.CompletionAttendance, 0, len(rows))
	for _, row := range rows {
		values = append(values, timetable.CompletionAttendance(row))
	}
	return r.capability.RestoreInstanceStudentAttendance(ctx, instanceID, values)
}
