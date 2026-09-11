package services

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/services/education"
)

// substitutionCapability serves workforce.Substitutions from the retained
// substitution module while that module's own move into the Workforce owner is
// pending (#2688). It only maps the storage-neutral contract types; every
// access rule and write stays in the module.
type substitutionCapability struct{ module education.SubstitutionModule }

// SubstitutionCapability binds the /api/substitutions capability to the
// substitution module.
func SubstitutionCapability(module education.SubstitutionModule) workforce.Substitutions {
	if module == nil {
		panic("substitution capability: substitution module is required")
	}
	return substitutionCapability{module: module}
}

func (c substitutionCapability) Overview(ctx context.Context, caller workforce.SubstitutionCaller, query workforce.SubstitutionOverviewQuery) (*workforce.SubstitutionOverviewResult, error) {
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

func (c substitutionCapability) Assign(ctx context.Context, caller workforce.SubstitutionCaller, assignment workforce.SubstitutionAssignment) (*workforce.SubstitutionAssignmentResult, error) {
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

func (c substitutionCapability) End(ctx context.Context, caller workforce.SubstitutionCaller, request workforce.SubstitutionEndRequest) error {
	err := c.module.End(ctx, substitutionCallerToModule(caller), education.EndRequest{
		Type: education.TargetType(request.Type), ID: request.ID,
	})
	return mapSubstitutionError(err)
}

// --- mapping ---

func substitutionCallerToModule(caller workforce.SubstitutionCaller) education.SubstitutionCaller {
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
		return nil, &workforce.SubstitutionOperationError{
			Target: workforce.ErrSubstitutionInvalidPeriod, Code: "invalid_period", Message: "Der Zeitraum ist ungültig.", Cause: err,
		}
	}
	return &date, nil
}

func assignmentToModule(assignment workforce.SubstitutionAssignment) (education.Assignment, error) {
	result := education.Assignment{Type: education.TargetType(assignment.Type)}
	if handover := assignment.GroupHandover; handover != nil {
		start, err := substitutionDate(handover.StartDate)
		if err != nil {
			return result, err
		}
		end, err := substitutionDate(handover.EndDate)
		if err != nil {
			return result, err
		}
		result.GroupHandover = &education.GroupHandoverAssignment{
			GroupID: handover.GroupID, TargetStaffID: handover.TargetStaffID, StartDate: start, EndDate: end,
		}
	}
	if supervision := assignment.AdditionalSupervision; supervision != nil {
		result.AdditionalSupervision = &education.AdditionalSupervisionAssignment{
			ActiveGroupID: supervision.ActiveGroupID, TargetStaffID: supervision.TargetStaffID,
		}
	}
	if schedule := assignment.ScheduleSubstitution; schedule != nil {
		value := &education.ScheduleSubstitutionAssignment{
			InstanceID: schedule.InstanceID, UnderstaffedAck: schedule.UnderstaffedAck, UnderstaffedNote: schedule.UnderstaffedNote,
			Absences:             make([]education.ScheduleAbsenceChange, 0, len(schedule.Absences)),
			Substitutions:        make([]education.ScheduleSubstitutionChange, 0, len(schedule.Substitutions)),
			SubstitutionRemovals: make([]education.ScheduleSubstitutionRemoval, 0, len(schedule.SubstitutionRemovals)),
			Presences:            make([]education.SchedulePresenceChange, 0, len(schedule.Presences)),
		}
		if schedule.Absences == nil {
			value.Absences = nil
		}
		for _, change := range schedule.Absences {
			value.Absences = append(value.Absences, education.ScheduleAbsenceChange{StaffID: change.StaffID, Reason: change.Reason, InstanceIDs: change.InstanceIDs})
		}
		if schedule.Substitutions == nil {
			value.Substitutions = nil
		}
		for _, change := range schedule.Substitutions {
			value.Substitutions = append(value.Substitutions, education.ScheduleSubstitutionChange{
				AbsentStaffID: change.AbsentStaffID, SubstituteStaffID: change.SubstituteStaffID, Reason: change.Reason, InstanceIDs: change.InstanceIDs,
			})
		}
		if schedule.SubstitutionRemovals == nil {
			value.SubstitutionRemovals = nil
		}
		for _, change := range schedule.SubstitutionRemovals {
			value.SubstitutionRemovals = append(value.SubstitutionRemovals, education.ScheduleSubstitutionRemoval{StaffID: change.StaffID, InstanceIDs: change.InstanceIDs})
		}
		if schedule.Presences == nil {
			value.Presences = nil
		}
		for _, change := range schedule.Presences {
			value.Presences = append(value.Presences, education.SchedulePresenceChange{StaffID: change.StaffID, InstanceIDs: change.InstanceIDs})
		}
		if wholeDays := schedule.WholeDays; wholeDays != nil {
			dates := make([]timezone.Date, 0, len(wholeDays.Dates))
			for _, raw := range wholeDays.Dates {
				date, err := substitutionDate(raw)
				if err != nil {
					return result, err
				}
				if date == nil {
					return result, &workforce.SubstitutionOperationError{
						Target: workforce.ErrSubstitutionInvalidPeriod, Code: "invalid_period", Message: "Der Zeitraum ist ungültig.",
					}
				}
				dates = append(dates, *date)
			}
			value.WholeDays = &education.ScheduleWholeDayAssignment{
				AbsentStaffID: wholeDays.AbsentStaffID, SubstituteStaffID: wholeDays.SubstituteStaffID, Dates: dates, Reason: wholeDays.Reason,
			}
		}
		result.ScheduleSubstitution = value
	}
	return result, nil
}

func overviewToCapability(result *education.OverviewResult) *workforce.SubstitutionOverviewResult {
	if result == nil {
		return nil
	}
	out := &workforce.SubstitutionOverviewResult{
		GroupHandovers:       make([]workforce.GroupHandover, 0, len(result.GroupHandovers)),
		Groups:               groupRefsToCapability(result.Groups),
		Targets:              staffRefsToCapability(result.Targets),
		ScheduleAppointments: make([]workforce.ScheduleAppointmentOverview, 0, len(result.ScheduleAppointments)),
		ScheduleTargets:      staffRefsToCapability(result.ScheduleTargets),
		RunningSupervisions:  make([]workforce.RunningSupervision, 0, len(result.RunningSupervisions)),
	}
	for _, handover := range result.GroupHandovers {
		out.GroupHandovers = append(out.GroupHandovers, workforce.GroupHandover{
			ID: handover.ID, Type: workforce.SubstitutionTargetType(handover.Type),
			Group:  workforce.GroupRef{ID: handover.Group.ID, Name: handover.Group.Name},
			Target: workforce.StaffRef{ID: handover.Target.ID, FullName: handover.Target.FullName},
			Period: workforce.Period{StartDate: handover.Period.StartDate, EndDate: handover.Period.EndDate},
			CanEnd: handover.CanEnd,
		})
	}
	for _, appointment := range result.ScheduleAppointments {
		staff := make([]workforce.ScheduleAppointmentStaff, 0, len(appointment.Staff))
		for _, entry := range appointment.Staff {
			staff = append(staff, workforce.ScheduleAppointmentStaff{
				AssignmentID: entry.AssignmentID, Staff: workforce.StaffRef{ID: entry.Staff.ID, FullName: entry.Staff.FullName},
				IsAbsent: entry.IsAbsent, IsSubstitute: entry.IsSubstitute, CanEnd: entry.CanEnd,
			})
		}
		out.ScheduleAppointments = append(out.ScheduleAppointments, workforce.ScheduleAppointmentOverview{
			ID: appointment.ID, Type: workforce.SubstitutionTargetType(appointment.Type), Date: appointment.Date.String(),
			StartTime: appointment.StartTime, EndTime: appointment.EndTime, Title: appointment.Title, Status: appointment.Status, Staff: staff,
		})
	}
	for _, supervision := range result.RunningSupervisions {
		out.RunningSupervisions = append(out.RunningSupervisions, workforce.RunningSupervision{
			ID: supervision.ID, Type: workforce.SubstitutionTargetType(supervision.Type), Name: supervision.Name, RoomName: supervision.RoomName,
			Supervisors: staffRefsToCapability(supervision.Supervisors), AvailableTargets: staffRefsToCapability(supervision.AvailableTargets),
			IsCurrentUserSupervising: supervision.IsCurrentUserSupervising, CanAssign: supervision.CanAssign,
		})
	}
	return out
}

func groupRefsToCapability(values []education.GroupRef) []workforce.GroupRef {
	result := make([]workforce.GroupRef, 0, len(values))
	for _, value := range values {
		result = append(result, workforce.GroupRef{ID: value.ID, Name: value.Name})
	}
	return result
}

func staffRefsToCapability(values []education.StaffRef) []workforce.StaffRef {
	result := make([]workforce.StaffRef, 0, len(values))
	for _, value := range values {
		result = append(result, workforce.StaffRef{ID: value.ID, FullName: value.FullName})
	}
	return result
}

func assignmentResultToCapability(result *education.AssignmentResult) *workforce.SubstitutionAssignmentResult {
	if result == nil {
		return nil
	}
	out := &workforce.SubstitutionAssignmentResult{
		ID: result.ID, Type: workforce.SubstitutionTargetType(result.Type), ActiveGroupID: result.ActiveGroupID,
		Target: workforce.StaffRef{ID: result.Target.ID, FullName: result.Target.FullName}, CanEnd: result.CanEnd,
	}
	if result.Group != nil {
		out.Group = &workforce.GroupRef{ID: result.Group.ID, Name: result.Group.Name}
	}
	if result.Period != nil {
		out.Period = &workforce.Period{StartDate: result.Period.StartDate, EndDate: result.Period.EndDate}
	}
	if result.ScheduleSubstitution != nil {
		out.ScheduleSubstitution = scheduleResultToCapability(result.ScheduleSubstitution)
	}
	return out
}

func scheduleResultToCapability(result *education.ScheduleSubstitutionResult) *workforce.ScheduleSubstitutionResult {
	out := &workforce.ScheduleSubstitutionResult{
		InstanceID: result.InstanceID, Cancelled: result.Cancelled, UnderstaffedAck: result.UnderstaffedAck,
		AffectedAppointments: affectedAppointmentsToCapability(result.AffectedAppointments),
		Warnings:             timeConflictsToCapability(result.Warnings),
		TotalAffected:        result.TotalAffected,
	}
	if result.Days != nil {
		out.Days = make([]workforce.ScheduleSubstitutionDayResult, 0, len(result.Days))
		for _, day := range result.Days {
			out.Days = append(out.Days, workforce.ScheduleSubstitutionDayResult{
				Date: day.Date.String(), AffectedAppointments: affectedAppointmentsToCapability(day.AffectedAppointments),
				Warnings: timeConflictsToCapability(day.Warnings),
			})
		}
	}
	return out
}

func affectedAppointmentsToCapability(values []education.ScheduleAffectedAppointment) []workforce.ScheduleAffectedAppointment {
	if values == nil {
		return nil
	}
	result := make([]workforce.ScheduleAffectedAppointment, 0, len(values))
	for _, value := range values {
		result = append(result, workforce.ScheduleAffectedAppointment{InstanceID: value.InstanceID, Title: value.Title, StartTime: value.StartTime, Action: value.Action})
	}
	return result
}

func timeConflictsToCapability(values []education.ScheduleTimeConflict) []workforce.ScheduleTimeConflict {
	if values == nil {
		return nil
	}
	result := make([]workforce.ScheduleTimeConflict, 0, len(values))
	for _, value := range values {
		result = append(result, workforce.ScheduleTimeConflict{Kind: value.Kind, InstanceID: value.InstanceID, OtherID: value.OtherID, Message: value.Message})
	}
	return result
}

var substitutionErrorKinds = []struct {
	module     error
	capability error
}{
	{education.ErrNotFound, workforce.ErrSubstitutionNotFound},
	{education.ErrForbidden, workforce.ErrSubstitutionForbidden},
	{education.ErrInvalidTarget, workforce.ErrSubstitutionInvalidTarget},
	{education.ErrInvalidPeriod, workforce.ErrSubstitutionInvalidPeriod},
	{education.ErrNotRunning, workforce.ErrSubstitutionNotRunning},
	{education.ErrAlreadyAssigned, workforce.ErrSubstitutionAlreadyAssigned},
	{education.ErrConflict, workforce.ErrSubstitutionConflict},
	{education.ErrSelfAssignment, workforce.ErrSubstitutionSelfAssignment},
}

// mapSubstitutionError translates the module's sentinels and operation errors
// into the capability's, keeping code, message and cause intact.
func mapSubstitutionError(err error) error {
	if err == nil {
		return nil
	}
	if operation, ok := errors.AsType[*education.OperationError](err); ok {
		mapped := &workforce.SubstitutionOperationError{Target: operation.Target, Code: operation.Code, Message: operation.Message, Cause: operation.Cause}
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
