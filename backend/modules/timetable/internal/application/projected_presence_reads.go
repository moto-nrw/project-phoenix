package application

import (
	"context"
	"errors"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	presenceContract "github.com/moto-nrw/project-phoenix/modules/presenceprojection/presencecontract"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PresenceReads serves the retained legacy reads that join the Timetable plan
// with the execution and attendance Student Presence owns since #2762. Each
// read goes through the tenant-safe timetable projection inside the caller's
// tenant transaction.
type ProjectedPresenceReads struct{ ports.PresenceProjection }

func NewProjectedPresenceReads(projection ports.PresenceProjection) (ProjectedPresenceReads, error) {
	if projection == nil {
		return ProjectedPresenceReads{}, errors.New("timetable presence reads: database is required")
	}
	return ProjectedPresenceReads{PresenceProjection: projection}, nil
}

func (r ProjectedPresenceReads) ListParallelPresence(ctx context.Context, excludedInstanceID int64, date string, studentIDs []int64) ([]scheduleModels.ParallelPresence, error) {
	rows, err := r.PresenceProjection.ListParallelPresence(ctx, excludedInstanceID, calendar.Date(date), studentIDs)
	return convertRows(rows, func(row presenceContract.ParallelPresence) scheduleModels.ParallelPresence {
		return scheduleModels.ParallelPresence(row)
	}), err
}

func (r ProjectedPresenceReads) ListPartialAbsenceBlocks(ctx context.Context, studentID int64, date string, from time.Time, enrolled bool, autoExceptionIDs []int64) ([]scheduleModels.PartialAbsenceBlock, error) {
	rows, err := r.PresenceProjection.ListPartialAbsenceBlocks(ctx, studentID, calendar.Date(date), from, enrolled, autoExceptionIDs)
	return convertRows(rows, func(row presenceContract.PartialAbsenceBlock) scheduleModels.PartialAbsenceBlock {
		return scheduleModels.PartialAbsenceBlock(row)
	}), err
}

func (r ProjectedPresenceReads) ListScheduledInstancesForStudent(ctx context.Context, studentID int64, from, to string) ([]*scheduleModels.ScheduledInstanceRow, error) {
	rows, err := r.PresenceProjection.ListScheduledInstancesForStudent(ctx, studentID, calendar.Date(from), calendar.Date(to))
	return convertRows(rows, func(row presenceContract.ScheduledInstance) *scheduleModels.ScheduledInstanceRow {
		return &scheduleModels.ScheduledInstanceRow{Instance: legacyInstance(row.Instance), Attendance: legacyParticipant(row.Participant)}
	}), err
}

func (r ProjectedPresenceReads) HasPlannedSlotsInRange(ctx context.Context, from, to string) (bool, error) {
	return r.PresenceProjection.HasPlannedSlotsInRange(ctx, calendar.Date(from), calendar.Date(to))
}

// ListLegacyInstances lists activity instances joined with their execution
// in one statement.
func (r ProjectedPresenceReads) ListLegacyInstances(ctx context.Context, filter ports.LegacyInstanceFilter) ([]*scheduleModels.ActivityInstance, error) {
	rows, err := r.PresenceProjection.ListLegacyInstances(ctx, presenceContract.LegacyInstanceFilter(filter))
	return convertRows(rows, legacyInstance), err
}

// ListLegacyParticipants lists participants joined with their attendance in
// one statement.
func (r ProjectedPresenceReads) ListLegacyParticipants(ctx context.Context, filter ports.LegacyParticipantFilter) ([]*scheduleModels.InstanceStudent, error) {
	rows, err := r.PresenceProjection.ListLegacyParticipants(ctx, presenceContract.LegacyParticipantFilter(filter))
	return convertRows(rows, legacyParticipant), err
}

// convertRows maps the projection's rows onto the retained DTOs; a failed
// read returns nil.
func convertRows[Row, DTO any](rows []Row, convert func(Row) DTO) []DTO {
	if rows == nil {
		return nil
	}
	result := make([]DTO, 0, len(rows))
	for _, row := range rows {
		result = append(result, convert(row))
	}
	return result
}

func legacyInstance(row presenceContract.Instance) *scheduleModels.ActivityInstance {
	instance := &scheduleModels.ActivityInstance{
		Date: scheduleModels.Date(row.Date), ActivityGroupID: row.ActivityGroupID, CalendarPeriodID: row.CalendarPeriodID,
		Title: row.Title, Description: row.Description, StartTime: row.StartTime, EndTime: row.EndTime, RoomID: row.RoomID,
		RequiredStaff: row.RequiredStaff, Status: row.Status, ActiveGroupID: row.ActiveGroupID, ListKind: row.ListKind,
		IsSpontaneous: row.IsSpontaneous, UnderstaffedAck: row.UnderstaffedAck, UnderstaffedNote: row.UnderstaffedNote,
		CancelReason: row.CancelReason, Notes: row.Notes, IdempotencyKey: row.IdempotencyKey,
		IdempotencyFingerprint: row.IdempotencyFingerprint, CreatedBy: row.CreatedBy, TemplateType: row.TemplateType,
		TemplateRequiredStaff: row.TemplateRequiredStaff, StartedBy: row.StartedBy, StartedAt: row.StartedAt,
		CompletedAt: row.CompletedAt, CompletedBy: row.CompletedBy, ReopenUntil: row.ReopenUntil, CompletionSnapshot: row.CompletionSnapshot,
	}
	instance.ID, instance.CreatedAt, instance.UpdatedAt = row.ID, row.CreatedAt, row.UpdatedAt
	instance.SetTenantID(row.TenantID)
	return instance
}

func legacyParticipant(row presenceContract.Participant) *scheduleModels.InstanceStudent {
	participant := &scheduleModels.InstanceStudent{
		InstanceID: row.InstanceID, StudentID: row.StudentID, RoomID: row.RoomID, Status: row.Status,
		Substatus: row.Substatus, Note: row.Note, CheckedInAt: row.CheckedInAt, CheckedOutAt: row.CheckedOutAt,
		IsUnplanned: row.IsUnplanned, NotScheduled: row.NotScheduled, ManualStatusAt: row.ManualStatusAt,
		StudentStatusDayID: row.StudentStatusDayID, PickupExceptionID: row.PickupExceptionID,
	}
	participant.ID, participant.CreatedAt, participant.UpdatedAt = row.ID, row.CreatedAt, row.UpdatedAt
	participant.SetTenantID(row.TenantID)
	return participant
}
