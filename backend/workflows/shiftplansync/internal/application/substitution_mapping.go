package application

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/services/education"
)

// Registered error codes (error-registry.json) of a rejected substitution.
// This package may not import api/common, so it names them once here.
const (
	codeSubstitutionConflict      = "substitutions.conflict"
	codeSubstitutionInvalidPeriod = "substitutions.invalid_period"
	codeSubstitutionInvalidTarget = "substitutions.invalid_target"
	codeSubstitutionNotFound      = "substitutions.not_found"
	codeSubstitutionNotRunning    = "substitutions.not_running"
)

// The mapping between the substitution module's storage-neutral vocabulary and
// the Timetable owner's deviation inputs and results. It used to live as a
// bridge in the composition root; it belongs to the workflow that owns the
// write (#3418).

func hasAppointmentChanges(assignment education.ScheduleSubstitutionAssignment) bool {
	return assignment.InstanceID != 0 || assignment.UnderstaffedAck != nil || assignment.UnderstaffedNote != nil ||
		len(assignment.Absences) > 0 || len(assignment.Substitutions) > 0 ||
		len(assignment.SubstitutionRemovals) > 0 || len(assignment.Presences) > 0
}

func mapScheduleDeviationInput(
	assignment education.ScheduleSubstitutionAssignment,
	actorAccountID int64,
) timetable.ApplyDeviationsInput {
	input := timetable.ApplyDeviationsInput{
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

func mapScheduleAbsences(source []education.ScheduleAbsenceChange) []timetable.DeviationAbsenceInput {
	result := make([]timetable.DeviationAbsenceInput, 0, len(source))
	for _, change := range source {
		result = append(result, timetable.DeviationAbsenceInput{
			StaffID: change.StaffID, Reason: change.Reason, InstanceIDs: change.InstanceIDs,
		})
	}
	return result
}

func mapScheduleSubstitutions(source []education.ScheduleSubstitutionChange) []timetable.DeviationSubstitutionInput {
	result := make([]timetable.DeviationSubstitutionInput, 0, len(source))
	for _, change := range source {
		result = append(result, timetable.DeviationSubstitutionInput{
			AbsentStaffID: change.AbsentStaffID, SubstituteStaffID: change.SubstituteStaffID,
			Reason: change.Reason, InstanceIDs: change.InstanceIDs,
		})
	}
	return result
}

func mapScheduleSubstitutionRemovals(source []education.ScheduleSubstitutionRemoval) []timetable.DeviationSubstitutionRemovalInput {
	result := make([]timetable.DeviationSubstitutionRemovalInput, 0, len(source))
	for _, change := range source {
		result = append(result, timetable.DeviationSubstitutionRemovalInput{
			StaffID: change.StaffID, InstanceIDs: change.InstanceIDs,
		})
	}
	return result
}

func mapSchedulePresences(source []education.SchedulePresenceChange) []timetable.DeviationPresenceInput {
	result := make([]timetable.DeviationPresenceInput, 0, len(source))
	for _, change := range source {
		result = append(result, timetable.DeviationPresenceInput{
			StaffID: change.StaffID, InstanceIDs: change.InstanceIDs,
		})
	}
	return result
}

func mapScheduleMutation(source *substitutionMutation) (*education.ScheduleSubstitutionResult, error) {
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

func mapAppointmentMutation(source *substitutionMutation) *education.ScheduleSubstitutionResult {
	result := source.Appointment
	return &education.ScheduleSubstitutionResult{
		InstanceID: result.InstanceID, UnderstaffedAck: result.UnderstaffedAck,
		AffectedAppointments: mapScheduleAffected(result.Affected),
		Warnings:             mapScheduleWarnings(result.Warnings), TotalAffected: result.AppliedWrites,
		AfterCommit: source.AfterCommit,
	}
}

func mapWholeDayMutation(source *substitutionMutation) *education.ScheduleSubstitutionResult {
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

func mapScheduleAffected(source []timetable.DeviationAffected) []education.ScheduleAffectedAppointment {
	result := make([]education.ScheduleAffectedAppointment, 0, len(source))
	for _, item := range source {
		result = append(result, education.ScheduleAffectedAppointment{
			InstanceID: item.InstanceID, Title: item.Title,
			StartTime: item.StartTime.Format("15:04"), Action: item.Action,
		})
	}
	return result
}

func mapScheduleWarnings(source []timetable.SubstituteTimeConflict) []education.ScheduleTimeConflict {
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
	case errors.Is(err, errSubstitutionInvalidPeriod):
		return &education.OperationError{
			Target: education.ErrInvalidPeriod, Code: codeSubstitutionInvalidPeriod, Message: "Der Zeitraum ist ungültig.",
		}
	case errors.Is(err, errSubstitutionNotFound):
		return &education.OperationError{
			Target: education.ErrNotFound, Code: codeSubstitutionNotFound, Message: "Die Terminvertretung wurde nicht gefunden.",
		}
	case errors.Is(err, errSubstitutionNotRunning):
		return &education.OperationError{
			Target: education.ErrNotRunning, Code: codeSubstitutionNotRunning, Message: "Die Terminvertretung ist nicht mehr aktiv.",
		}
	}
	var deviation *timetable.DeviationError
	if !errors.As(err, &deviation) || deviation.Status == 500 {
		return err
	}
	target, code, message := education.ErrInvalidTarget, codeSubstitutionInvalidTarget, "Die Angaben für die Vertretung sind ungültig."
	switch deviation.Status {
	case 404:
		target, code, message = education.ErrNotFound, codeSubstitutionNotFound, "Der Termin oder die Person wurde nicht gefunden."
	case 409:
		target, code, message = education.ErrConflict, codeSubstitutionConflict, deviation.ClientMsg
	}
	// The refusal's own code and the values it names reach the client, so it
	// can say why (#2516); the class code only covers an uncoded refusal.
	if deviation.Code != "" {
		code = deviation.Code
	}
	return &education.OperationError{
		Target: target, Code: code, Message: message, Cause: deviation.Cause,
		Details: refusalDetails(deviation.Details), Field: deviation.Field,
	}
}

// refusalDetails puts the values a refusal names into wire form; nil when it
// names none.
func refusalDetails(values timetable.RefusalValues) map[string]any {
	details := map[string]any{}
	if values.Date != "" {
		details["date"] = values.Date
	}
	if values.Max > 0 {
		details["max"] = values.Max
	}
	if len(details) == 0 {
		return nil
	}
	return details
}
