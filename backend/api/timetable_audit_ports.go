package api

import (
	"context"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// The Audit Platform's trails the Timetable owner writes through
// consumer-owned ports (#3424 slice S3): the Änderungsprotokoll of the
// deviation writes and the attendance correction trail.

// The Timetable owner may not name the Audit Platform, so it mirrors the
// vocabulary these trails store, and the bindings below pass it through
// unchanged. A drift must not compile: a false comparison repeats the false
// key of this map literal.
var _ = map[bool]struct{}{
	false: {},
	auditModels.DeviationEventAbsence == timetablePorts.DeviationEventAbsence &&
		auditModels.DeviationEventReturnToPresence == timetablePorts.DeviationEventReturnToPresence &&
		auditModels.DeviationEventSubstitution == timetablePorts.DeviationEventSubstitution &&
		auditModels.DeviationEventSubstituteRemoved == timetablePorts.DeviationEventSubstituteRemoved &&
		auditModels.DeviationEventSickReported == timetablePorts.DeviationEventSickReported &&
		auditModels.DeviationEventSickCleared == timetablePorts.DeviationEventSickCleared &&
		auditModels.AttendanceFieldStatus == timetable.AttendanceCorrectionFieldStatus &&
		auditModels.AttendanceFieldSubstatus == timetable.AttendanceCorrectionFieldSubstatus &&
		auditModels.AttendanceFieldNote == timetable.AttendanceCorrectionFieldNote &&
		auditModels.CorrectionReasonMaxLength == timetable.AttendanceCorrectionReasonMaxLength: {},
}

// TimetableAttendanceCorrectionTrail serves the correction trail from the
// Audit Platform's attendance corrections; nil stays nil, so the correction
// fails closed.
func TimetableAttendanceCorrectionTrail(corrections attendanceCorrectionLedger) timetablePorts.AttendanceCorrectionTrail {
	if corrections == nil {
		return nil
	}
	return timetableCorrectionTrail{corrections: corrections}
}

// attendanceCorrectionLedger is the Audit Platform's correction trail as the
// attendance correction appends and lists it.
type attendanceCorrectionLedger interface {
	CreateBatch(ctx context.Context, corrections []*auditModels.AttendanceCorrection) error
	ListByInstanceAndStudent(ctx context.Context, instanceID, studentID int64) ([]*auditModels.AttendanceCorrection, error)
}

type timetableCorrectionTrail struct {
	corrections attendanceCorrectionLedger
}

func (t timetableCorrectionTrail) RecordAttendanceCorrections(ctx context.Context, records []timetablePorts.AttendanceCorrectionRecord) error {
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
