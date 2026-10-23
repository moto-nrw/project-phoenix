package api

import (
	"context"
	"errors"

	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// capabilityError reports a Workforce sentinel through errors.Is while keeping
// the wording of the legacy service error, which is what the HTTP envelope
// renders. Unwrap keeps the original chain reachable.
type capabilityError struct {
	kind  error
	cause error
}

func (e *capabilityError) Error() string        { return e.cause.Error() }
func (e *capabilityError) Is(target error) bool { return target == e.kind }
func (e *capabilityError) Unwrap() error        { return e.cause }

// substitutionCapability serves workforceModule.Substitutions from the retained
// substitution module while that module's own move into the Workforce owner is
// pending (#2688). It only maps the storage-neutral contract types; every
// access rule and write stays in the module.
type substitutionCapability struct{ module education.SubstitutionModule }

// SubstitutionCapability binds the /api/substitutions capability to the
// substitution module.
func SubstitutionCapability(module education.SubstitutionModule) workforceModule.Substitutions {
	if module == nil {
		panic("substitution capability: substitution module is required")
	}
	return substitutionCapability{module: module}
}

func (c substitutionCapability) Overview(ctx context.Context, caller workforceModule.SubstitutionCaller, query workforceModule.SubstitutionOverviewQuery) (*workforceModule.SubstitutionOverviewResult, error) {
	on, err := substitutionDate(query.On)
	if err != nil {
		return nil, err
	}
	from, err := substitutionDate(query.ScheduleFrom)
	if err != nil {
		return nil, err
	}
	to, err := substitutionDate(query.ScheduleTo)
	if err != nil {
		return nil, err
	}
	result, err := c.module.Overview(ctx, substitutionCallerToModule(caller), education.OverviewQuery{
		GroupID: query.GroupID, ActiveGroupID: query.ActiveGroupID, On: on, IncludeTargets: query.IncludeTargets,
		ScheduleFrom: from, ScheduleTo: to, IncludeScheduleTargets: query.IncludeScheduleTargets,
	})
	if err != nil {
		return nil, mapSubstitutionError(err)
	}
	return overviewToCapability(result), nil
}

func (c substitutionCapability) Assign(ctx context.Context, caller workforceModule.SubstitutionCaller, assignment workforceModule.SubstitutionAssignment) (*workforceModule.SubstitutionAssignmentResult, error) {
	request, err := assignmentToModule(assignment)
	if err != nil {
		return nil, err
	}
	result, err := c.module.Assign(ctx, substitutionCallerToModule(caller), request)
	if err != nil {
		return nil, mapSubstitutionError(err)
	}
	return assignmentResultToCapability(result), nil
}

func (c substitutionCapability) End(ctx context.Context, caller workforceModule.SubstitutionCaller, request workforceModule.SubstitutionEndRequest) error {
	err := c.module.End(ctx, substitutionCallerToModule(caller), education.EndRequest{
		Type: education.TargetType(request.Type), ID: request.ID,
	})
	return mapSubstitutionError(err)
}

// --- mapping ---

func substitutionCallerToModule(caller workforceModule.SubstitutionCaller) education.SubstitutionCaller {
	return education.SubstitutionCaller{
		AccountID: caller.AccountID, TenantID: caller.TenantID, Scope: caller.Scope, Roles: caller.Roles,
		HasPermission: caller.HasPermission, Admin: caller.Admin,
	}
}

// substitutionDate parses an optional calendar day; an unset day stays nil
// and a malformed one is the module's invalid-period rejection.
func substitutionDate(value string) (*timezone.Date, error) {
	if value == "" {
		return nil, nil
	}
	date, err := timezone.ParseDate(value)
	if err != nil {
		return nil, workforceModule.InvalidSubstitutionPeriod(err)
	}
	return &date, nil
}

func assignmentToModule(assignment workforceModule.SubstitutionAssignment) (education.Assignment, error) {
	result := education.Assignment{Type: education.TargetType(assignment.Type)}
	if handover := assignment.GroupHandover; handover != nil {
		value, err := groupHandoverToModule(*handover)
		if err != nil {
			return result, err
		}
		result.GroupHandover = value
	}
	if supervision := assignment.AdditionalSupervision; supervision != nil {
		result.AdditionalSupervision = &education.AdditionalSupervisionAssignment{
			ActiveGroupID: supervision.ActiveGroupID, TargetStaffID: supervision.TargetStaffID,
		}
	}
	if schedule := assignment.ScheduleSubstitution; schedule != nil {
		value, err := scheduleSubstitutionToModule(*schedule)
		if err != nil {
			return result, err
		}
		result.ScheduleSubstitution = value
	}
	return result, nil
}

func groupHandoverToModule(handover workforceModule.GroupHandoverAssignment) (*education.GroupHandoverAssignment, error) {
	start, err := substitutionDate(handover.StartDate)
	if err != nil {
		return nil, err
	}
	end, err := substitutionDate(handover.EndDate)
	if err != nil {
		return nil, err
	}
	return &education.GroupHandoverAssignment{
		GroupID: handover.GroupID, TargetStaffID: handover.TargetStaffID, StartDate: start, EndDate: end,
	}, nil
}

func scheduleSubstitutionToModule(schedule workforceModule.ScheduleSubstitutionAssignment) (*education.ScheduleSubstitutionAssignment, error) {
	value := &education.ScheduleSubstitutionAssignment{
		InstanceID: schedule.InstanceID, UnderstaffedAck: schedule.UnderstaffedAck, UnderstaffedNote: schedule.UnderstaffedNote,
		Absences:             scheduleAbsencesToModule(schedule.Absences),
		Substitutions:        scheduleSubstitutionsToModule(schedule.Substitutions),
		SubstitutionRemovals: scheduleRemovalsToModule(schedule.SubstitutionRemovals),
		Presences:            schedulePresencesToModule(schedule.Presences),
	}
	if wholeDays := schedule.WholeDays; wholeDays != nil {
		days, err := wholeDaysToModule(*wholeDays)
		if err != nil {
			return nil, err
		}
		value.WholeDays = days
	}
	return value, nil
}

// The change lists keep their nil-ness: an absent list stays nil, an empty
// one stays empty.

func scheduleAbsencesToModule(changes []workforceModule.ScheduleAbsenceChange) []education.ScheduleAbsenceChange {
	if changes == nil {
		return nil
	}
	result := make([]education.ScheduleAbsenceChange, 0, len(changes))
	for _, change := range changes {
		result = append(result, education.ScheduleAbsenceChange{StaffID: change.StaffID, Reason: change.Reason, InstanceIDs: change.InstanceIDs})
	}
	return result
}

func scheduleSubstitutionsToModule(changes []workforceModule.ScheduleSubstitutionChange) []education.ScheduleSubstitutionChange {
	if changes == nil {
		return nil
	}
	result := make([]education.ScheduleSubstitutionChange, 0, len(changes))
	for _, change := range changes {
		result = append(result, education.ScheduleSubstitutionChange{
			AbsentStaffID: change.AbsentStaffID, SubstituteStaffID: change.SubstituteStaffID, Reason: change.Reason, InstanceIDs: change.InstanceIDs,
		})
	}
	return result
}

func scheduleRemovalsToModule(changes []workforceModule.ScheduleSubstitutionRemoval) []education.ScheduleSubstitutionRemoval {
	if changes == nil {
		return nil
	}
	result := make([]education.ScheduleSubstitutionRemoval, 0, len(changes))
	for _, change := range changes {
		result = append(result, education.ScheduleSubstitutionRemoval{StaffID: change.StaffID, InstanceIDs: change.InstanceIDs})
	}
	return result
}

func schedulePresencesToModule(changes []workforceModule.SchedulePresenceChange) []education.SchedulePresenceChange {
	if changes == nil {
		return nil
	}
	result := make([]education.SchedulePresenceChange, 0, len(changes))
	for _, change := range changes {
		result = append(result, education.SchedulePresenceChange{StaffID: change.StaffID, InstanceIDs: change.InstanceIDs})
	}
	return result
}

// wholeDaysToModule maps the whole-day assignment with the days Workforce
// parsed.
func wholeDaysToModule(wholeDays workforceModule.ScheduleWholeDayAssignment) (*education.ScheduleWholeDayAssignment, error) {
	dates, err := wholeDays.Days()
	if err != nil {
		return nil, err
	}
	return &education.ScheduleWholeDayAssignment{
		AbsentStaffID: wholeDays.AbsentStaffID, SubstituteStaffID: wholeDays.SubstituteStaffID, Dates: dates, Reason: wholeDays.Reason,
	}, nil
}

func overviewToCapability(result *education.OverviewResult) *workforceModule.SubstitutionOverviewResult {
	if result == nil {
		return nil
	}
	out := &workforceModule.SubstitutionOverviewResult{
		GroupHandovers:       make([]workforceModule.GroupHandover, 0, len(result.GroupHandovers)),
		Groups:               groupRefsToCapability(result.Groups),
		Targets:              staffRefsToCapability(result.Targets),
		ScheduleAppointments: make([]workforceModule.ScheduleAppointmentOverview, 0, len(result.ScheduleAppointments)),
		ScheduleTargets:      staffRefsToCapability(result.ScheduleTargets),
		RunningSupervisions:  make([]workforceModule.RunningSupervision, 0, len(result.RunningSupervisions)),
	}
	for _, handover := range result.GroupHandovers {
		out.GroupHandovers = append(out.GroupHandovers, workforceModule.GroupHandover{
			ID: handover.ID, Type: workforceModule.SubstitutionTargetType(handover.Type),
			Group:  workforceModule.GroupRef{ID: handover.Group.ID, Name: handover.Group.Name},
			Target: workforceModule.StaffRef{ID: handover.Target.ID, FullName: handover.Target.FullName},
			Period: workforceModule.Period{StartDate: handover.Period.StartDate, EndDate: handover.Period.EndDate},
			CanEnd: handover.CanEnd,
		})
	}
	for _, appointment := range result.ScheduleAppointments {
		staff := make([]workforceModule.ScheduleAppointmentStaff, 0, len(appointment.Staff))
		for _, entry := range appointment.Staff {
			staff = append(staff, workforceModule.ScheduleAppointmentStaff{
				AssignmentID: entry.AssignmentID, Staff: workforceModule.StaffRef{ID: entry.Staff.ID, FullName: entry.Staff.FullName},
				IsAbsent: entry.IsAbsent, IsSubstitute: entry.IsSubstitute, CanEnd: entry.CanEnd,
			})
		}
		out.ScheduleAppointments = append(out.ScheduleAppointments, workforceModule.ScheduleAppointmentOverview{
			ID: appointment.ID, Type: workforceModule.SubstitutionTargetType(appointment.Type), Date: appointment.Date.String(),
			StartTime: appointment.StartTime, EndTime: appointment.EndTime, Title: appointment.Title, Status: appointment.Status, Staff: staff,
		})
	}
	for _, supervision := range result.RunningSupervisions {
		out.RunningSupervisions = append(out.RunningSupervisions, workforceModule.RunningSupervision{
			ID: supervision.ID, Type: workforceModule.SubstitutionTargetType(supervision.Type), Name: supervision.Name, RoomName: supervision.RoomName,
			Supervisors: staffRefsToCapability(supervision.Supervisors), AvailableTargets: staffRefsToCapability(supervision.AvailableTargets),
			IsCurrentUserSupervising: supervision.IsCurrentUserSupervising, CanAssign: supervision.CanAssign,
		})
	}
	return out
}

func groupRefsToCapability(values []education.GroupRef) []workforceModule.GroupRef {
	result := make([]workforceModule.GroupRef, 0, len(values))
	for _, value := range values {
		result = append(result, workforceModule.GroupRef{ID: value.ID, Name: value.Name})
	}
	return result
}

func staffRefsToCapability(values []education.StaffRef) []workforceModule.StaffRef {
	result := make([]workforceModule.StaffRef, 0, len(values))
	for _, value := range values {
		result = append(result, workforceModule.StaffRef{
			ID: value.ID, FullName: value.FullName, IsExternal: value.IsExternal,
			ExternalAccountStatus: value.ExternalAccountStatus, ExternalOrganization: value.ExternalOrganization,
		})
	}
	return result
}

func assignmentResultToCapability(result *education.AssignmentResult) *workforceModule.SubstitutionAssignmentResult {
	if result == nil {
		return nil
	}
	out := &workforceModule.SubstitutionAssignmentResult{
		ID: result.ID, Type: workforceModule.SubstitutionTargetType(result.Type), ActiveGroupID: result.ActiveGroupID,
		Target: workforceModule.StaffRef{ID: result.Target.ID, FullName: result.Target.FullName}, CanEnd: result.CanEnd,
	}
	if result.Group != nil {
		out.Group = &workforceModule.GroupRef{ID: result.Group.ID, Name: result.Group.Name}
	}
	if result.Period != nil {
		out.Period = &workforceModule.Period{StartDate: result.Period.StartDate, EndDate: result.Period.EndDate}
	}
	if result.ScheduleSubstitution != nil {
		out.ScheduleSubstitution = scheduleResultToCapability(result.ScheduleSubstitution)
	}
	return out
}

func scheduleResultToCapability(result *education.ScheduleSubstitutionResult) *workforceModule.ScheduleSubstitutionResult {
	out := &workforceModule.ScheduleSubstitutionResult{
		InstanceID: result.InstanceID, Cancelled: result.Cancelled, UnderstaffedAck: result.UnderstaffedAck,
		AffectedAppointments: affectedAppointmentsToCapability(result.AffectedAppointments),
		Warnings:             timeConflictsToCapability(result.Warnings),
		TotalAffected:        result.TotalAffected,
	}
	if result.Days != nil {
		out.Days = make([]workforceModule.ScheduleSubstitutionDayResult, 0, len(result.Days))
		for _, day := range result.Days {
			out.Days = append(out.Days, workforceModule.ScheduleSubstitutionDayResult{
				Date: day.Date.String(), AffectedAppointments: affectedAppointmentsToCapability(day.AffectedAppointments),
				Warnings: timeConflictsToCapability(day.Warnings),
			})
		}
	}
	return out
}

func affectedAppointmentsToCapability(values []education.ScheduleAffectedAppointment) []workforceModule.ScheduleAffectedAppointment {
	if values == nil {
		return nil
	}
	result := make([]workforceModule.ScheduleAffectedAppointment, 0, len(values))
	for _, value := range values {
		result = append(result, workforceModule.ScheduleAffectedAppointment{InstanceID: value.InstanceID, Title: value.Title, StartTime: value.StartTime, Action: value.Action})
	}
	return result
}

func timeConflictsToCapability(values []education.ScheduleTimeConflict) []workforceModule.ScheduleTimeConflict {
	if values == nil {
		return nil
	}
	result := make([]workforceModule.ScheduleTimeConflict, 0, len(values))
	for _, value := range values {
		result = append(result, workforceModule.ScheduleTimeConflict{Kind: value.Kind, InstanceID: value.InstanceID, OtherID: value.OtherID, Message: value.Message})
	}
	return result
}

var substitutionErrorKinds = []struct {
	module     error
	capability error
}{
	{education.ErrNotFound, workforceModule.ErrSubstitutionNotFound},
	{education.ErrForbidden, workforceModule.ErrSubstitutionForbidden},
	{education.ErrInvalidTarget, workforceModule.ErrSubstitutionInvalidTarget},
	{education.ErrInvalidPeriod, workforceModule.ErrSubstitutionInvalidPeriod},
	{education.ErrNotRunning, workforceModule.ErrSubstitutionNotRunning},
	{education.ErrAlreadyAssigned, workforceModule.ErrSubstitutionAlreadyAssigned},
	{education.ErrConflict, workforceModule.ErrSubstitutionConflict},
	{education.ErrSelfAssignment, workforceModule.ErrSubstitutionSelfAssignment},
}

// mapSubstitutionError translates the module's sentinels and operation errors
// into the capability's, keeping code, message and cause intact.
func mapSubstitutionError(err error) error {
	if err == nil {
		return nil
	}
	if operation, ok := errors.AsType[*education.OperationError](err); ok {
		mapped := &workforceModule.SubstitutionOperationError{
			Target: operation.Target, Code: operation.Code, Message: operation.Message, Cause: operation.Cause,
			Details: substitutionRefusalValues(operation.Details), Field: operation.Field,
		}
		for _, kind := range substitutionErrorKinds {
			if errors.Is(operation.Target, kind.module) {
				mapped.Target = kind.capability
				break
			}
		}
		return mapped
	}
	for _, kind := range substitutionErrorKinds {
		if errors.Is(err, kind.module) {
			return &capabilityError{kind: kind.capability, cause: err}
		}
	}
	return err
}

// substitutionRefusalValues keeps the values a schedule refusal names (#2516).
func substitutionRefusalValues(details education.SubstitutionRefusalValues) workforceModule.SubstitutionRefusalValues {
	return workforceModule.SubstitutionRefusalValues{Date: details.Date, Max: details.Max}
}
