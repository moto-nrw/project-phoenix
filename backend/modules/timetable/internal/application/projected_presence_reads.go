package application

import (
	"context"
	"errors"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PresenceReads serves the retained legacy reads that join the Timetable plan
// with the execution and attendance Student Presence owns since #2762. Each
// read goes through the tenant-safe timetable projection inside the caller's
// tenant transaction.
type ProjectedPresenceReads struct {
	ports.PresenceReader
	runInTx func(context.Context, func(context.Context) error) error
}

func NewProjectedPresenceReads(projection ports.PresenceReader, runInTx func(context.Context, func(context.Context) error) error) (ProjectedPresenceReads, error) {
	if projection == nil {
		return ProjectedPresenceReads{}, errors.New("timetable presence reads: database is required")
	}
	return ProjectedPresenceReads{PresenceReader: projection, runInTx: runInTx}, nil
}

func (r ProjectedPresenceReads) ListParallelPresence(ctx context.Context, excludedInstanceID int64, date string, studentIDs []int64) (value0 []scheduleModels.ParallelPresence, err error) {
	err = r.runInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.inUnitListParallelPresence(txCtx, excludedInstanceID, date, studentIDs)
		return err
	})
	return
}

func (r ProjectedPresenceReads) inUnitListParallelPresence(ctx context.Context, excludedInstanceID int64, date string, studentIDs []int64) ([]scheduleModels.ParallelPresence, error) {
	rows, err := r.PresenceReader.ListParallelPresence(ctx, excludedInstanceID, calendar.Date(date), studentIDs)
	return convertRows(rows, func(row ports.ParallelPresence) scheduleModels.ParallelPresence {
		return scheduleModels.ParallelPresence(row)
	}), err
}

func (r ProjectedPresenceReads) ListPartialAbsenceBlocks(ctx context.Context, studentID int64, date string, from time.Time, enrolled bool, autoExceptionIDs []int64) (value0 []scheduleModels.PartialAbsenceBlock, err error) {
	err = r.runInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.inUnitListPartialAbsenceBlocks(txCtx, studentID, date, from, enrolled, autoExceptionIDs)
		return err
	})
	return
}

func (r ProjectedPresenceReads) inUnitListPartialAbsenceBlocks(ctx context.Context, studentID int64, date string, from time.Time, enrolled bool, autoExceptionIDs []int64) ([]scheduleModels.PartialAbsenceBlock, error) {
	rows, err := r.PresenceReader.ListPartialAbsenceBlocks(ctx, studentID, calendar.Date(date), from, enrolled, autoExceptionIDs)
	return convertRows(rows, func(row ports.PartialAbsenceBlock) scheduleModels.PartialAbsenceBlock {
		return scheduleModels.PartialAbsenceBlock(row)
	}), err
}

func (r ProjectedPresenceReads) ListScheduledInstancesForStudent(ctx context.Context, studentID int64, from, to string) (value0 []*scheduleModels.ScheduledInstanceRow, err error) {
	err = r.runInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.inUnitListScheduledInstancesForStudent(txCtx, studentID, from, to)
		return err
	})
	return
}

func (r ProjectedPresenceReads) inUnitListScheduledInstancesForStudent(ctx context.Context, studentID int64, from, to string) ([]*scheduleModels.ScheduledInstanceRow, error) {
	rows, err := r.PresenceReader.ListScheduledInstancesForStudent(ctx, studentID, calendar.Date(from), calendar.Date(to))
	return convertRows(rows, func(row ports.ScheduledInstance) *scheduleModels.ScheduledInstanceRow {
		return &scheduleModels.ScheduledInstanceRow{Instance: legacyInstance(row.Instance), Attendance: legacyParticipant(row.Participant)}
	}), err
}

func (r ProjectedPresenceReads) HasPlannedSlotsInRange(ctx context.Context, from, to string) (value0 bool, err error) {
	err = r.runInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.inUnitHasPlannedSlotsInRange(txCtx, from, to)
		return err
	})
	return
}

func (r ProjectedPresenceReads) inUnitHasPlannedSlotsInRange(ctx context.Context, from, to string) (bool, error) {
	return r.PresenceReader.HasPlannedSlotsInRange(ctx, calendar.Date(from), calendar.Date(to))
}

// ListLegacyInstances lists activity instances joined with their execution
// in one statement.
func (r ProjectedPresenceReads) ListLegacyInstances(ctx context.Context, filter ports.LegacyInstanceFilter) (value0 []*scheduleModels.ActivityInstance, err error) {
	err = r.runInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.inUnitListLegacyInstances(txCtx, filter)
		return err
	})
	return
}

func (r ProjectedPresenceReads) inUnitListLegacyInstances(ctx context.Context, filter ports.LegacyInstanceFilter) ([]*scheduleModels.ActivityInstance, error) {
	rows, err := r.ListInstances(ctx, filter)
	return convertRows(rows, legacyInstance), err
}

// ListLegacyParticipants lists participants joined with their attendance in
// one statement.
func (r ProjectedPresenceReads) ListLegacyParticipants(ctx context.Context, filter ports.LegacyParticipantFilter) (value0 []*scheduleModels.InstanceStudent, err error) {
	err = r.runInTx(ctx, func(txCtx context.Context) error {
		value0, err = r.inUnitListLegacyParticipants(txCtx, filter)
		return err
	})
	return
}

func (r ProjectedPresenceReads) inUnitListLegacyParticipants(ctx context.Context, filter ports.LegacyParticipantFilter) ([]*scheduleModels.InstanceStudent, error) {
	rows, err := r.ListParticipants(ctx, filter)
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

func legacyInstance(row ports.ActivityInstance) *scheduleModels.ActivityInstance {
	instance := &scheduleModels.ActivityInstance{
		Date: calendar.Date(row.Date), ActivityGroupID: row.ActivityGroupID, CalendarPeriodID: row.CalendarPeriodID,
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

func legacyParticipant(row ports.ParticipantAttendance) *scheduleModels.InstanceStudent {
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
