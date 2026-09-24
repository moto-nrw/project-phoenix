package application

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The sentinels the schedule half classifies on. They stay inside the
// workflow: every caller receives the mapped education.OperationError.
var (
	errSubstitutionInvalidPeriod = errors.New("invalid substitution period")
	errSubstitutionNotFound      = errors.New("schedule substitution not found")
	errSubstitutionNotRunning    = errors.New("schedule substitution is not running")
)

// ScheduleDeviations is the Timetable owner's deviation command surface a
// Terminvertretung writes through (timetable.StaffDeviations).
type ScheduleDeviations interface {
	ApplyDeviations(ctx context.Context, instanceID int64, in timetable.ApplyDeviationsInput) (*timetable.ApplyDeviationsResult, error)
	ApplyBulkSubstitution(ctx context.Context, in timetable.BulkSubstitutionInput) (*timetable.BulkSubstitutionResult, error)
	QueueActivityUpdates(ctx context.Context, touched timetable.TouchedActivities)
}

type SubstitutionAdapterDependencies struct {
	Instances     scheduleModel.ActivityInstanceRepository
	InstanceStaff scheduleModel.InstanceStaffRepository
	Staff         userModels.StaffRepository
	Engine        ScheduleDeviations
	Broadcaster   realtime.Broadcaster
	Logger        *slog.Logger
}

// SubstitutionAdapter serves the schedule half of the substitution module: the
// appointment overview, the deviation writes and ending one substitution. It
// implements education.ScheduleAdapter directly, so the composition root needs
// no mapping bridge of its own.
type SubstitutionAdapter struct {
	deps SubstitutionAdapterDependencies
}

func NewSubstitutionAdapter(deps SubstitutionAdapterDependencies) *SubstitutionAdapter {
	return &SubstitutionAdapter{deps: deps}
}

// substitutionMutation is what one deviation write changed: either one
// appointment or a set of whole days, plus the after-commit notification.
type substitutionMutation struct {
	Appointment *timetable.ApplyDeviationsResult
	WholeDays   *timetable.BulkSubstitutionResult
	AfterCommit func(context.Context)
}

func (a *SubstitutionAdapter) Overview(
	ctx context.Context,
	from, to timezone.Date,
	includeTargets, canManage bool,
) (*education.ScheduleOverview, error) {
	overview, err := a.overview(ctx, from, to, includeTargets, canManage)
	if err != nil {
		return nil, mapScheduleSubstitutionError(err)
	}
	return overview, nil
}

func (a *SubstitutionAdapter) overview(
	ctx context.Context,
	from, to timezone.Date,
	includeTargets, canManage bool,
) (*education.ScheduleOverview, error) {
	if from.IsZero() || to.IsZero() || to.Before(from) || from.DaysUntil(to) >= 56 {
		return nil, errSubstitutionInvalidPeriod
	}
	instances, err := a.deps.Instances.FindByTenantAndDateRange(ctx, scheduleModel.Date(from), scheduleModel.Date(to))
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
	return &education.ScheduleOverview{
		Appointments: projectSubstitutionAppointments(instances, deviations, staffByID, canManage),
		Targets:      targets,
	}, nil
}

func (a *SubstitutionAdapter) loadOverviewStaff(ctx context.Context, instances []*scheduleModel.ActivityInstance) (map[int64][]*scheduleModel.InstanceStaff, map[int64]*userModels.Staff, error) {
	instanceIDs := make([]int64, 0, len(instances))
	for _, instance := range instances {
		if instance != nil {
			instanceIDs = append(instanceIDs, instance.ID)
		}
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

func projectSubstitutionAppointments(instances []*scheduleModel.ActivityInstance, deviations map[int64][]*scheduleModel.InstanceStaff, staffByID map[int64]*userModels.Staff, canManage bool) []education.ScheduleAppointmentOverview {
	appointments := make([]education.ScheduleAppointmentOverview, 0, len(deviations))
	for _, instance := range instances {
		rows := deviations[instance.ID]
		if len(rows) == 0 {
			continue
		}
		staff := make([]education.ScheduleAppointmentStaff, 0, len(rows))
		canChange := canManage && !instance.Date.Before(timezone.TodayDate()) && isPlannableInstance(instance)
		for _, row := range rows {
			member := staffByID[row.StaffID]
			name := ""
			if member != nil {
				name = member.GetFullName()
			}
			staff = append(staff, education.ScheduleAppointmentStaff{
				AssignmentID: row.ID,
				Staff:        education.StaffRef{ID: row.StaffID, FullName: name},
				IsAbsent:     row.IsAbsent,
				IsSubstitute: row.IsSubstitute,
				CanEnd:       canChange && row.IsSubstitute && !row.IsAbsent,
			})
		}
		appointments = append(appointments, education.ScheduleAppointmentOverview{
			ID: instance.ID, Type: education.TargetScheduleSubstitution,
			Date:      timezone.Date(instance.Date),
			StartTime: instance.StartTime.Format("15:04"), EndTime: instance.EndTime.Format("15:04"),
			Title: instance.Title, Status: instance.Status, Staff: staff,
		})
	}
	return appointments
}

func (a *SubstitutionAdapter) loadTargets(ctx context.Context, include bool) ([]education.StaffRef, error) {
	targets := []education.StaffRef{}
	if !include {
		return targets, nil
	}
	members, err := a.deps.Staff.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	for _, member := range members {
		if member != nil {
			targets = append(targets, education.StaffRef{ID: member.ID, FullName: member.GetFullName()})
		}
	}
	return targets, nil
}

func (a *SubstitutionAdapter) Assign(
	ctx context.Context,
	assignment education.ScheduleSubstitutionAssignment,
	actorAccountID int64,
) (*education.ScheduleSubstitutionResult, error) {
	mutation, err := a.apply(ctx, assignment, actorAccountID)
	if err != nil {
		return nil, mapScheduleSubstitutionError(err)
	}
	return mapScheduleMutation(mutation)
}

func (a *SubstitutionAdapter) apply(
	ctx context.Context,
	assignment education.ScheduleSubstitutionAssignment,
	actorAccountID int64,
) (*substitutionMutation, error) {
	if assignment.WholeDays == nil {
		return a.applyAppointment(
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
	return a.applyWholeDays(ctx, timetable.BulkSubstitutionInput{
		AbsentStaffID: wholeDays.AbsentStaffID, SubstituteStaffID: wholeDays.SubstituteStaffID,
		Dates: wholeDays.Dates, Reason: wholeDays.Reason, ActorAccountID: &actorAccountID,
	})
}

func (a *SubstitutionAdapter) applyAppointment(ctx context.Context, instanceID int64, input timetable.ApplyDeviationsInput) (*substitutionMutation, error) {
	result, err := a.deps.Engine.ApplyDeviations(ctx, instanceID, input)
	if err != nil {
		return nil, err
	}
	return &substitutionMutation{
		Appointment: result,
		AfterCommit: a.afterCommit(result.ActiveTouched, result.AppliedWrites > 0 || result.AckChanged || result.ClearedAcks > 0),
	}, nil
}

func (a *SubstitutionAdapter) applyWholeDays(ctx context.Context, input timetable.BulkSubstitutionInput) (*substitutionMutation, error) {
	result, err := a.deps.Engine.ApplyBulkSubstitution(ctx, input)
	if err != nil {
		return nil, err
	}
	return &substitutionMutation{
		WholeDays:   result,
		AfterCommit: a.afterCommit(result.ActiveTouched, result.AppliedWrites > 0 || result.ClearedAcks > 0),
	}, nil
}

func (a *SubstitutionAdapter) End(
	ctx context.Context,
	substitutionID, actorAccountID int64,
) (*education.ScheduleSubstitutionResult, error) {
	mutation, err := a.end(ctx, substitutionID, actorAccountID)
	if err != nil {
		return nil, mapScheduleSubstitutionError(err)
	}
	return mapScheduleMutation(mutation)
}

func (a *SubstitutionAdapter) end(ctx context.Context, substitutionID, actorAccountID int64) (*substitutionMutation, error) {
	row, err := a.deps.InstanceStaff.FindByID(ctx, substitutionID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, errSubstitutionNotFound
		}
		return nil, err
	}
	if row == nil || !row.IsSubstitute {
		return nil, errSubstitutionNotFound
	}
	if row.IsAbsent {
		return nil, errSubstitutionNotRunning
	}
	selected := []int64{row.InstanceID}
	return a.applyAppointment(ctx, row.InstanceID, timetable.ApplyDeviationsInput{
		ActorAccountID: &actorAccountID,
		SubstitutionRemovals: []timetable.DeviationSubstitutionRemovalInput{{
			StaffID: row.StaffID, InstanceIDs: &selected,
		}},
	})
}

func (a *SubstitutionAdapter) afterCommit(activeTouched timetable.TouchedActivities, notifyStaffing bool) func(context.Context) {
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
