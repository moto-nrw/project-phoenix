package compose

import (
	"context"
	"errors"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/presenceprojection"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// PresenceReads serves the retained legacy reads that join the Timetable plan
// with the execution and attendance Student Presence owns since #2762. Each
// read goes through the tenant-safe timetable projection inside the caller's
// tenant transaction.
type PresenceReads struct{ database postgres.Database }

func NewPresenceReads(db *bun.DB) (PresenceReads, error) {
	if db == nil {
		return PresenceReads{}, errors.New("timetable presence reads: database is required")
	}
	return PresenceReads{database: databaseRuntime(db)}, nil
}

func (r PresenceReads) CountNonAbsentParticipants(ctx context.Context, instanceIDs []int64) (map[int64]int, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	return presenceprojection.CountNonAbsentParticipants(ctx, db, tenantID, instanceIDs)
}

func (r PresenceReads) ListParallelPresence(ctx context.Context, excludedInstanceID int64, date string, studentIDs []int64) ([]scheduleModels.ParallelPresence, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := presenceprojection.ListParallelPresence(ctx, db, tenantID, excludedInstanceID, calendar.Date(date), studentIDs)
	return convertRows(rows, func(row presenceprojection.ParallelPresence) scheduleModels.ParallelPresence {
		return scheduleModels.ParallelPresence(row)
	}), err
}

func (r PresenceReads) ListPartialAbsenceBlocks(ctx context.Context, studentID int64, date string, from time.Time, enrolled bool, autoExceptionIDs []int64) ([]scheduleModels.PartialAbsenceBlock, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := presenceprojection.ListPartialAbsenceBlocks(ctx, db, tenantID, studentID, calendar.Date(date), from, enrolled, autoExceptionIDs)
	return convertRows(rows, func(row presenceprojection.PartialAbsenceBlock) scheduleModels.PartialAbsenceBlock {
		return scheduleModels.PartialAbsenceBlock(row)
	}), err
}

func (r PresenceReads) ListScheduledInstancesForStudent(ctx context.Context, studentID int64, from, to string) ([]*scheduleModels.ScheduledInstanceRow, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := presenceprojection.ListScheduledInstancesForStudent(ctx, db, tenantID, studentID, calendar.Date(from), calendar.Date(to))
	return convertRows(rows, func(row presenceprojection.ScheduledInstance) *scheduleModels.ScheduledInstanceRow {
		return &scheduleModels.ScheduledInstanceRow{Instance: legacyInstance(row.Instance), Attendance: legacyParticipant(row.Participant)}
	}), err
}

func (r PresenceReads) HasPlannedSlotsInRange(ctx context.Context, from, to string) (bool, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return false, err
	}
	return presenceprojection.HasPlannedSlotsInRange(ctx, db, tenantID, calendar.Date(from), calendar.Date(to))
}

// LegacyInstanceFilter selects activity instances the way the retained
// repositories ask for them; see presenceprojection.LegacyInstanceFilter.
type LegacyInstanceFilter presenceprojection.LegacyInstanceFilter

// LegacyParticipantFilter selects participants the way the retained
// repositories ask for them; see presenceprojection.LegacyParticipantFilter.
type LegacyParticipantFilter presenceprojection.LegacyParticipantFilter

// ListLegacyInstances lists activity instances joined with their execution
// in one statement.
func (r PresenceReads) ListLegacyInstances(ctx context.Context, filter LegacyInstanceFilter) ([]*scheduleModels.ActivityInstance, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := presenceprojection.ListLegacyInstances(ctx, db, tenantID, presenceprojection.LegacyInstanceFilter(filter))
	return convertRows(rows, legacyInstance), err
}

// ListLegacyParticipants lists participants joined with their attendance in
// one statement.
func (r PresenceReads) ListLegacyParticipants(ctx context.Context, filter LegacyParticipantFilter) ([]*scheduleModels.InstanceStudent, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := presenceprojection.ListLegacyParticipants(ctx, db, tenantID, presenceprojection.LegacyParticipantFilter(filter))
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

func legacyInstance(row presenceprojection.Instance) *scheduleModels.ActivityInstance {
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

func legacyParticipant(row presenceprojection.Participant) *scheduleModels.InstanceStudent {
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

func mustPresenceReads(db *bun.DB) PresenceReads {
	reads, err := NewPresenceReads(db)
	if err != nil {
		panic(err)
	}
	return reads
}
