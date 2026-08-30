package services

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/services/schedule"
)

type scheduleSubstitutionBridge struct {
	adapter *schedule.SubstitutionAdapter
}

func newScheduleSubstitutionBridge(adapter *schedule.SubstitutionAdapter) education.ScheduleSubstitutionAdapter {
	return &scheduleSubstitutionBridge{adapter: adapter}
}

func (b *scheduleSubstitutionBridge) Overview(
	ctx context.Context,
	from, to timezone.Date,
	includeTargets, canManage bool,
) (*education.ScheduleOverview, error) {
	overview, err := b.adapter.Overview(ctx, from, to, includeTargets, canManage)
	if err != nil {
		return nil, mapScheduleSubstitutionError(err)
	}
	return mapScheduleOverview(overview), nil
}

func (b *scheduleSubstitutionBridge) Assign(
	ctx context.Context,
	assignment education.ScheduleSubstitutionAssignment,
	actorAccountID int64,
) (*education.ScheduleSubstitutionResult, error) {
	mutation, err := b.apply(ctx, assignment, actorAccountID)
	if err != nil {
		return nil, mapScheduleSubstitutionError(err)
	}
	return mapScheduleMutation(mutation)
}

func (b *scheduleSubstitutionBridge) apply(
	ctx context.Context,
	assignment education.ScheduleSubstitutionAssignment,
	actorAccountID int64,
) (*schedule.SubstitutionMutation, error) {
	if assignment.WholeDays == nil {
		return b.adapter.ApplyAppointment(
			ctx,
			assignment.InstanceID,
			mapScheduleDeviationInput(assignment, actorAccountID),
		)
	}
	if hasAppointmentChanges(assignment) {
		return nil, &education.OperationError{
			Target:  education.ErrInvalidTarget,
			Code:    "invalid_target",
			Message: "Eine Sammelvertretung kann nicht mit Terminänderungen verbunden werden.",
		}
	}
	wholeDays := assignment.WholeDays
	return b.adapter.ApplyWholeDays(ctx, schedule.BulkSubstitutionInput{
		AbsentStaffID: wholeDays.AbsentStaffID, SubstituteStaffID: wholeDays.SubstituteStaffID,
		Dates: wholeDays.Dates, Reason: wholeDays.Reason, ActorAccountID: &actorAccountID,
	})
}

func (b *scheduleSubstitutionBridge) End(
	ctx context.Context,
	substitutionID, actorAccountID int64,
) (*education.ScheduleSubstitutionResult, error) {
	mutation, err := b.adapter.End(ctx, substitutionID, actorAccountID)
	if err != nil {
		return nil, mapScheduleSubstitutionError(err)
	}
	return mapScheduleMutation(mutation)
}

func hasAppointmentChanges(assignment education.ScheduleSubstitutionAssignment) bool {
	return assignment.InstanceID != 0 || assignment.UnderstaffedAck != nil || assignment.UnderstaffedNote != nil ||
		len(assignment.Absences) > 0 || len(assignment.Substitutions) > 0 ||
		len(assignment.SubstitutionRemovals) > 0 || len(assignment.Presences) > 0
}

func mapScheduleDeviationInput(
	assignment education.ScheduleSubstitutionAssignment,
	actorAccountID int64,
) schedule.ApplyDeviationsInput {
	input := schedule.ApplyDeviationsInput{
		UnderstaffedAck:  assignment.UnderstaffedAck,
		UnderstaffedNote: assignment.UnderstaffedNote,
		ActorAccountID:   &actorAccountID,
	}
	input.Absences = mapScheduleAbsences(assignment.Absences)
	input.Substitutions = mapScheduleSubstitutions(assignment.Substitutions)
	input.SubstitutionRemovals = mapScheduleSubstitutionRemovals(assignment.SubstitutionRemovals)
	input.Presences = mapSchedulePresences(assignment.Presences)
	return input
}

func mapScheduleAbsences(source []education.ScheduleAbsenceChange) []schedule.DeviationAbsenceInput {
	result := make([]schedule.DeviationAbsenceInput, 0, len(source))
	for _, change := range source {
		result = append(result, schedule.DeviationAbsenceInput{
			StaffID: change.StaffID, Reason: change.Reason, InstanceIDs: change.InstanceIDs,
		})
	}
	return result
}

func mapScheduleSubstitutions(source []education.ScheduleSubstitutionChange) []schedule.DeviationSubstitutionInput {
	result := make([]schedule.DeviationSubstitutionInput, 0, len(source))
	for _, change := range source {
		result = append(result, schedule.DeviationSubstitutionInput{
			AbsentStaffID: change.AbsentStaffID, SubstituteStaffID: change.SubstituteStaffID,
			Reason: change.Reason, InstanceIDs: change.InstanceIDs,
		})
	}
	return result
}

func mapScheduleSubstitutionRemovals(source []education.ScheduleSubstitutionRemoval) []schedule.DeviationSubstitutionRemovalInput {
	result := make([]schedule.DeviationSubstitutionRemovalInput, 0, len(source))
	for _, change := range source {
		result = append(result, schedule.DeviationSubstitutionRemovalInput{
			StaffID: change.StaffID, InstanceIDs: change.InstanceIDs,
		})
	}
	return result
}

func mapSchedulePresences(source []education.SchedulePresenceChange) []schedule.DeviationPresenceInput {
	result := make([]schedule.DeviationPresenceInput, 0, len(source))
	for _, change := range source {
		result = append(result, schedule.DeviationPresenceInput{
			StaffID: change.StaffID, InstanceIDs: change.InstanceIDs,
		})
	}
	return result
}

func mapScheduleOverview(source *schedule.SubstitutionOverview) *education.ScheduleOverview {
	result := &education.ScheduleOverview{
		Appointments: make([]education.ScheduleAppointmentOverview, 0, len(source.Appointments)),
		Targets:      mapScheduleStaffRefs(source.Targets),
	}
	for _, appointment := range source.Appointments {
		result.Appointments = append(result.Appointments, education.ScheduleAppointmentOverview{
			ID: appointment.ID, Type: education.TargetScheduleSubstitution,
			Date: appointment.Date, StartTime: appointment.StartTime, EndTime: appointment.EndTime,
			Title: appointment.Title, Status: appointment.Status,
			Staff: mapScheduleAppointmentStaff(appointment.Staff),
		})
	}
	return result
}

func mapScheduleStaffRefs(source []schedule.SubstitutionStaffRef) []education.StaffRef {
	result := make([]education.StaffRef, 0, len(source))
	for _, staff := range source {
		result = append(result, education.StaffRef{ID: staff.ID, FullName: staff.FullName})
	}
	return result
}

func mapScheduleAppointmentStaff(source []schedule.SubstitutionAppointmentStaff) []education.ScheduleAppointmentStaff {
	result := make([]education.ScheduleAppointmentStaff, 0, len(source))
	for _, staff := range source {
		result = append(result, education.ScheduleAppointmentStaff{
			AssignmentID: staff.AssignmentID,
			Staff:        education.StaffRef{ID: staff.Staff.ID, FullName: staff.Staff.FullName},
			IsAbsent:     staff.IsAbsent, IsSubstitute: staff.IsSubstitute, CanEnd: staff.CanEnd,
		})
	}
	return result
}

func mapScheduleMutation(source *schedule.SubstitutionMutation) (*education.ScheduleSubstitutionResult, error) {
	if source == nil {
		return nil, education.ErrInvalidTarget
	}
	if source.Appointment != nil {
		return mapAppointmentMutation(source), nil
	}
	if source.WholeDays != nil {
		return mapWholeDayMutation(source), nil
	}
	return nil, education.ErrInvalidTarget
}

func mapAppointmentMutation(source *schedule.SubstitutionMutation) *education.ScheduleSubstitutionResult {
	result := source.Appointment
	return &education.ScheduleSubstitutionResult{
		InstanceID: result.InstanceID, UnderstaffedAck: result.UnderstaffedAck,
		AffectedAppointments: mapScheduleAffected(result.Affected),
		Warnings:             mapScheduleWarnings(result.Warnings), TotalAffected: result.AppliedWrites,
		AfterCommit: source.AfterCommit,
	}
}

func mapWholeDayMutation(source *schedule.SubstitutionMutation) *education.ScheduleSubstitutionResult {
	result := source.WholeDays
	days := make([]education.ScheduleSubstitutionDayResult, 0, len(result.Days))
	for _, day := range result.Days {
		days = append(days, education.ScheduleSubstitutionDayResult{
			Date: day.Date, AffectedAppointments: mapScheduleAffected(day.Affected),
			Warnings: mapScheduleWarnings(day.Warnings),
		})
	}
	return &education.ScheduleSubstitutionResult{
		Days: days, TotalAffected: result.AppliedWrites, AfterCommit: source.AfterCommit,
	}
}

func mapScheduleAffected(source []schedule.DeviationAffected) []education.ScheduleAffectedAppointment {
	result := make([]education.ScheduleAffectedAppointment, 0, len(source))
	for _, item := range source {
		result = append(result, education.ScheduleAffectedAppointment{
			InstanceID: item.InstanceID, Title: item.Title,
			StartTime: item.StartTime.Format("15:04"), Action: item.Action,
		})
	}
	return result
}

func mapScheduleWarnings(source []schedule.SubstituteTimeConflict) []education.ScheduleTimeConflict {
	result := make([]education.ScheduleTimeConflict, 0, len(source))
	for _, item := range source {
		result = append(result, education.ScheduleTimeConflict{
			Kind: item.Kind, InstanceID: item.InstanceID,
			OtherID: item.OtherID, Message: item.Message,
		})
	}
	return result
}

func mapScheduleSubstitutionError(err error) error {
	switch {
	case errors.Is(err, schedule.ErrSubstitutionInvalidPeriod):
		return &education.OperationError{
			Target: education.ErrInvalidPeriod, Code: "invalid_period", Message: "Der Zeitraum ist ungültig.",
		}
	case errors.Is(err, schedule.ErrSubstitutionNotFound):
		return education.ErrNotFound
	case errors.Is(err, schedule.ErrSubstitutionNotRunning):
		return education.ErrNotRunning
	}
	var deviation *schedule.DeviationError
	if !errors.As(err, &deviation) || deviation.Status == 500 {
		return err
	}
	target, code := education.ErrInvalidTarget, "invalid_target"
	switch deviation.Status {
	case 404:
		target, code = education.ErrNotFound, "not_found"
	case 409:
		target, code = education.ErrConflict, deviation.Code
		if code == "" {
			code = "conflict"
		}
	}
	return &education.OperationError{
		Target: target, Code: code, Message: deviation.ClientMsg, Cause: deviation.Cause,
	}
}
