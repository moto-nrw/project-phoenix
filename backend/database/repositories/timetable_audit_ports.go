package repositories

import (
	"context"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

// The Audit Platform's trails the Timetable owner writes through
// consumer-owned ports (#3424 slice S3): the Änderungsprotokoll of the
// deviation writes and the attendance correction trail.

// TimetableDeviationProtocol appends the owner's Änderungsprotokoll entries
// to the Audit Platform's deviation events.
func TimetableDeviationProtocol(events auditModels.DeviationEventRepository) timetableCompose.DeviationProtocol {
	return timetableDeviationProtocol{events: events}
}

// TimetableAttendanceCorrectionTrail serves the correction trail from the
// Audit Platform's attendance corrections; nil stays nil, so the correction
// fails closed.
func TimetableAttendanceCorrectionTrail(corrections auditModels.AttendanceCorrectionRepository) timetableCompose.AttendanceCorrectionTrail {
	if corrections == nil {
		return nil
	}
	return timetableCorrectionTrail{corrections: corrections}
}

type timetableDeviationProtocol struct {
	events auditModels.DeviationEventRepository
}

func (p timetableDeviationProtocol) RecordDeviationEvent(ctx context.Context, event timetableCompose.DeviationEventRecord) error {
	return p.events.Create(ctx, &auditModels.DeviationEvent{
		ActivityGroupID: event.ActivityGroupID,
		OccurrenceDate:  auditModels.Date(event.OccurrenceDate),
		StartTime:       event.StartTime,
		InstanceID:      event.InstanceID,
		SubjectStaffID:  event.SubjectStaffID,
		RelatedStaffID:  event.RelatedStaffID,
		EventType:       event.EventType,
		OldValue:        event.OldValue,
		NewValue:        event.NewValue,
		Reason:          event.Reason,
		ActorAccountID:  event.ActorAccountID,
	})
}

type timetableCorrectionTrail struct {
	corrections auditModels.AttendanceCorrectionRepository
}

func (t timetableCorrectionTrail) RecordAttendanceCorrections(ctx context.Context, records []timetableCompose.AttendanceCorrectionRecord) error {
	rows := make([]*auditModels.AttendanceCorrection, 0, len(records))
	for _, record := range records {
		rows = append(rows, &auditModels.AttendanceCorrection{
			InstanceID:        record.InstanceID,
			StudentID:         record.StudentID,
			ActorAccountID:    record.ActorAccountID,
			ActorNameSnapshot: record.ActorNameSnapshot,
			FieldName:         record.FieldName,
			OldValue:          record.OldValue,
			NewValue:          record.NewValue,
			Reason:            record.Reason,
		})
	}
	return t.corrections.CreateBatch(ctx, rows)
}

func (t timetableCorrectionTrail) ListAttendanceCorrections(ctx context.Context, instanceID, studentID int64) ([]timetable.AttendanceCorrectionEntry, error) {
	rows, err := t.corrections.ListByInstanceAndStudent(ctx, instanceID, studentID)
	if err != nil {
		return nil, err
	}
	entries := make([]timetable.AttendanceCorrectionEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, timetable.AttendanceCorrectionEntry{
			FieldName: row.FieldName, OldValue: row.OldValue, NewValue: row.NewValue,
			Reason: row.Reason, ActorName: row.ActorNameSnapshot, CorrectedAt: row.CreatedAt,
		})
	}
	return entries, nil
}
