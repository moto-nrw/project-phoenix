package schedule

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	substitution "github.com/moto-nrw/project-phoenix/services/substitution"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type SubstitutionAdapterDependencies struct {
	Instances     scheduleModel.ActivityInstanceRepository
	InstanceStaff scheduleModel.InstanceStaffRepository
	Staff         userModels.StaffRepository
	Engine        InstanceService
	Broadcaster   realtime.Broadcaster
	Logger        *slog.Logger
}

type SubstitutionAdapter struct {
	deps SubstitutionAdapterDependencies
}

func NewSubstitutionAdapter(deps SubstitutionAdapterDependencies) *SubstitutionAdapter {
	return &SubstitutionAdapter{deps: deps}
}

func (a *SubstitutionAdapter) Overview(ctx context.Context, from, to timezone.Date, includeTargets, canManage bool) (*substitution.ScheduleOverview, error) {
	if from.IsZero() || to.IsZero() || to.Before(from) || from.DaysUntil(to) >= 56 {
		return nil, &substitution.OperationError{
			Target: substitution.ErrInvalidPeriod, Code: "invalid_period", Message: "Der Zeitraum ist ungültig.",
		}
	}
	instances, err := a.deps.Instances.FindByTenantAndDateRange(ctx, from, to)
	if err != nil {
		return nil, err
	}
	deviations, staffByID, err := a.loadOverviewStaff(ctx, instances)
	if err != nil {
		return nil, err
	}
	targets, err := a.loadTargets(ctx, includeTargets)
	if err != nil {
		return nil, err
	}
	return &substitution.ScheduleOverview{
		Appointments: projectScheduleAppointments(instances, deviations, staffByID, canManage),
		Targets:      targets,
	}, nil
}

func (a *SubstitutionAdapter) loadOverviewStaff(ctx context.Context, instances []*scheduleModel.ActivityInstance) (map[int64][]*scheduleModel.InstanceStaff, map[int64]*userModels.Staff, error) {
	instanceIDs := make([]int64, 0, len(instances))
	for _, instance := range instances {
		if instance == nil {
			continue
		}
		instanceIDs = append(instanceIDs, instance.ID)
	}
	rows, err := a.deps.InstanceStaff.FindByInstanceIDs(ctx, instanceIDs)
	if err != nil {
		return nil, nil, err
	}
	staffIDs := make([]int64, 0, len(rows))
	seenStaff := make(map[int64]bool, len(rows))
	deviationsByInstance := make(map[int64][]*scheduleModel.InstanceStaff)
	for _, row := range rows {
		if row == nil || (!row.IsAbsent && !row.IsSubstitute) {
			continue
		}
		deviationsByInstance[row.InstanceID] = append(deviationsByInstance[row.InstanceID], row)
		if !seenStaff[row.StaffID] {
			seenStaff[row.StaffID] = true
			staffIDs = append(staffIDs, row.StaffID)
		}
	}
	staffByID, err := a.deps.Staff.FindWithPersonByIDs(ctx, staffIDs)
	if err != nil {
		return nil, nil, err
	}
	return deviationsByInstance, staffByID, nil
}

func projectScheduleAppointments(instances []*scheduleModel.ActivityInstance, deviations map[int64][]*scheduleModel.InstanceStaff, staffByID map[int64]*userModels.Staff, canManage bool) []substitution.ScheduleAppointmentOverview {
	appointments := make([]substitution.ScheduleAppointmentOverview, 0, len(deviations))
	for _, instance := range instances {
		rows := deviations[instance.ID]
		if len(rows) == 0 {
			continue
		}
		staff := make([]substitution.ScheduleAppointmentStaff, 0, len(rows))
		canChange := canManage && !instance.Date.Before(timezone.TodayDate()) && isPlannableInstance(instance)
		for _, row := range rows {
			member := staffByID[row.StaffID]
			name := ""
			if member != nil {
				name = member.GetFullName()
			}
			staff = append(staff, substitution.ScheduleAppointmentStaff{
				AssignmentID: row.ID, Staff: substitution.StaffRef{ID: row.StaffID, FullName: name},
				IsAbsent: row.IsAbsent, IsSubstitute: row.IsSubstitute,
				CanEnd: canChange && row.IsSubstitute && !row.IsAbsent,
			})
		}
		appointments = append(appointments, substitution.ScheduleAppointmentOverview{
			ID: instance.ID, Type: substitution.TargetScheduleSubstitution,
			Date: instance.Date, StartTime: instance.StartTime.Format("15:04"),
			EndTime: instance.EndTime.Format("15:04"), Title: instance.Title,
			Status: instance.Status, Staff: staff,
		})
	}
	return appointments
}

func (a *SubstitutionAdapter) loadTargets(ctx context.Context, include bool) ([]substitution.StaffRef, error) {
	targets := []substitution.StaffRef{}
	if !include {
		return targets, nil
	}
	members, err := a.deps.Staff.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	for _, member := range members {
		if member != nil {
			targets = append(targets, substitution.StaffRef{ID: member.ID, FullName: member.GetFullName()})
		}
	}
	return targets, nil
}

func (a *SubstitutionAdapter) Assign(ctx context.Context, assignment substitution.ScheduleSubstitutionAssignment, actorAccountID int64) (*substitution.ScheduleSubstitutionResult, error) {
	if assignment.WholeDays != nil {
		return a.assignWholeDays(ctx, assignment, actorAccountID)
	}
	result, err := a.deps.Engine.ApplyDeviations(ctx, assignment.InstanceID, deviationInput(assignment, actorAccountID))
	if err != nil {
		return nil, operationError(err)
	}
	return a.appointmentResult(result), nil
}

func deviationInput(assignment substitution.ScheduleSubstitutionAssignment, actorAccountID int64) ApplyDeviationsInput {
	input := ApplyDeviationsInput{
		UnderstaffedAck: assignment.UnderstaffedAck, UnderstaffedNote: assignment.UnderstaffedNote,
		ActorAccountID: &actorAccountID,
	}
	for _, change := range assignment.Absences {
		input.Absences = append(input.Absences, DeviationAbsenceInput{
			StaffID: change.StaffID, Reason: change.Reason, InstanceIDs: change.InstanceIDs,
		})
	}
	for _, change := range assignment.Substitutions {
		input.Substitutions = append(input.Substitutions, DeviationSubstitutionInput{
			AbsentStaffID: change.AbsentStaffID, SubstituteStaffID: change.SubstituteStaffID,
			Reason: change.Reason, InstanceIDs: change.InstanceIDs,
		})
	}
	for _, change := range assignment.SubstitutionRemovals {
		input.SubstitutionRemovals = append(input.SubstitutionRemovals, DeviationSubstitutionRemovalInput{
			StaffID: change.StaffID, InstanceIDs: change.InstanceIDs,
		})
	}
	for _, change := range assignment.Presences {
		input.Presences = append(input.Presences, DeviationPresenceInput{
			StaffID: change.StaffID, InstanceIDs: change.InstanceIDs,
		})
	}
	return input
}

func (a *SubstitutionAdapter) appointmentResult(result *ApplyDeviationsResult) *substitution.ScheduleSubstitutionResult {
	return &substitution.ScheduleSubstitutionResult{
		InstanceID: result.InstanceID, UnderstaffedAck: result.UnderstaffedAck,
		AffectedAppointments: mapAffected(result.Affected), Warnings: mapWarnings(result.Warnings),
		TotalAffected: result.AppliedWrites,
		AfterCommit:   a.afterCommit(result.ActiveTouched, result.AppliedWrites > 0 || result.AckChanged || result.ClearedAcks > 0),
	}
}

func (a *SubstitutionAdapter) assignWholeDays(ctx context.Context, assignment substitution.ScheduleSubstitutionAssignment, actorAccountID int64) (*substitution.ScheduleSubstitutionResult, error) {
	if assignment.InstanceID != 0 || assignment.UnderstaffedAck != nil || assignment.UnderstaffedNote != nil ||
		len(assignment.Absences) > 0 || len(assignment.Substitutions) > 0 ||
		len(assignment.SubstitutionRemovals) > 0 || len(assignment.Presences) > 0 {
		return nil, &substitution.OperationError{
			Target: substitution.ErrInvalidTarget, Code: "invalid_target",
			Message: "Eine Sammelvertretung kann nicht mit Terminänderungen verbunden werden.",
		}
	}
	wholeDays := assignment.WholeDays
	result, err := a.deps.Engine.ApplyBulkSubstitution(ctx, BulkSubstitutionInput{
		AbsentStaffID: wholeDays.AbsentStaffID, SubstituteStaffID: wholeDays.SubstituteStaffID,
		Dates: wholeDays.Dates, Reason: wholeDays.Reason, ActorAccountID: &actorAccountID,
	})
	if err != nil {
		return nil, operationError(err)
	}
	return &substitution.ScheduleSubstitutionResult{
		Days: mapBulkDays(result.Days), TotalAffected: result.AppliedWrites,
		AfterCommit: a.afterCommit(result.ActiveTouched, result.AppliedWrites > 0 || result.ClearedAcks > 0),
	}, nil
}

func mapBulkDays(source []BulkSubstitutionDay) []substitution.ScheduleSubstitutionDayResult {
	days := make([]substitution.ScheduleSubstitutionDayResult, 0, len(source))
	for _, day := range source {
		days = append(days, substitution.ScheduleSubstitutionDayResult{
			Date: day.Date, AffectedAppointments: mapAffected(day.Affected), Warnings: mapWarnings(day.Warnings),
		})
	}
	return days
}

func mapAffected(source []DeviationAffected) []substitution.ScheduleAffectedAppointment {
	result := make([]substitution.ScheduleAffectedAppointment, 0, len(source))
	for _, item := range source {
		result = append(result, substitution.ScheduleAffectedAppointment{
			InstanceID: item.InstanceID, Title: item.Title, StartTime: item.StartTime.Format("15:04"), Action: item.Action,
		})
	}
	return result
}

func mapWarnings(source []SubstituteTimeConflict) []substitution.ScheduleTimeConflict {
	result := make([]substitution.ScheduleTimeConflict, 0, len(source))
	for _, item := range source {
		result = append(result, substitution.ScheduleTimeConflict{
			Kind: item.Kind, InstanceID: item.InstanceID, OtherID: item.OtherID, Message: item.Message,
		})
	}
	return result
}

func (a *SubstitutionAdapter) End(ctx context.Context, substitutionID, actorAccountID int64) (*substitution.ScheduleSubstitutionResult, error) {
	row, err := a.deps.InstanceStaff.FindByID(ctx, substitutionID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, substitution.ErrNotFound
		}
		return nil, err
	}
	if row == nil || !row.IsSubstitute {
		return nil, substitution.ErrNotFound
	}
	if row.IsAbsent {
		return nil, substitution.ErrNotRunning
	}
	selected := []int64{row.InstanceID}
	return a.Assign(ctx, substitution.ScheduleSubstitutionAssignment{
		InstanceID: row.InstanceID,
		SubstitutionRemovals: []substitution.ScheduleSubstitutionRemoval{{
			StaffID: row.StaffID, InstanceIDs: &selected,
		}},
	}, actorAccountID)
}

func operationError(err error) error {
	var deviation *DeviationError
	if !errors.As(err, &deviation) {
		return err
	}
	target := substitution.ErrInvalidTarget
	code := "invalid_target"
	switch deviation.Status {
	case 404:
		target, code = substitution.ErrNotFound, "not_found"
	case 409:
		target, code = substitution.ErrConflict, deviation.Code
	case 500:
		return err
	}
	if code == "" {
		code = "conflict"
	}
	return &substitution.OperationError{
		Target: target, Code: code, Message: deviation.ClientMsg, Cause: deviation.Cause,
	}
}

func (a *SubstitutionAdapter) afterCommit(activeTouched map[int64]*scheduleModel.ActivityInstance, notifyStaffing bool) func(context.Context) {
	return func(ctx context.Context) {
		a.deps.Engine.QueueActivityUpdates(ctx, activeTouched)
		if notifyStaffing {
			a.broadcastStaffingChanged(ctx)
		}
	}
}

func (a *SubstitutionAdapter) broadcastStaffingChanged(ctx context.Context) {
	if a.deps.Broadcaster == nil {
		return
	}
	logger := a.deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	source := "schedule_substitution"
	tenantID := tenant.FromContext(ctx)
	event := realtime.NewEvent(realtime.EventStaffingDeviationChanged, "", realtime.EventData{Source: &source})
	tenant.RegisterAfterCommit(ctx, func() {
		if err := a.deps.Broadcaster.BroadcastToTenant(tenantID, event); err != nil {
			logger.Warn(
				"SSE schedule substitution broadcast failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
		}
	})
}
