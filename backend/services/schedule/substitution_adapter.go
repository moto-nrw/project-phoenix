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
	"github.com/moto-nrw/project-phoenix/tenant"
)

var (
	ErrSubstitutionInvalidPeriod = errors.New("invalid substitution period")
	ErrSubstitutionNotFound      = errors.New("schedule substitution not found")
	ErrSubstitutionNotRunning    = errors.New("schedule substitution is not running")
)

type SubstitutionAdapterDependencies struct {
	Instances     scheduleModel.ActivityInstanceRepository
	InstanceStaff scheduleModel.InstanceStaffRepository
	Staff         userModels.StaffRepository
	Engine        InstanceService
	Broadcaster   realtime.Broadcaster
	Logger        *slog.Logger
}

type SubstitutionStaffRef struct {
	ID       int64
	FullName string
}

type SubstitutionAppointmentStaff struct {
	AssignmentID int64
	Staff        SubstitutionStaffRef
	IsAbsent     bool
	IsSubstitute bool
	CanEnd       bool
}

type SubstitutionAppointment struct {
	ID        int64
	Date      timezone.Date
	StartTime string
	EndTime   string
	Title     string
	Status    string
	Staff     []SubstitutionAppointmentStaff
}

type SubstitutionOverview struct {
	Appointments []SubstitutionAppointment
	Targets      []SubstitutionStaffRef
}

type SubstitutionMutation struct {
	Appointment *ApplyDeviationsResult
	WholeDays   *BulkSubstitutionResult
	AfterCommit func(context.Context)
}

type SubstitutionAdapter struct {
	deps SubstitutionAdapterDependencies
}

func NewSubstitutionAdapter(deps SubstitutionAdapterDependencies) *SubstitutionAdapter {
	return &SubstitutionAdapter{deps: deps}
}

func (a *SubstitutionAdapter) Overview(ctx context.Context, from, to timezone.Date, includeTargets, canManage bool) (*SubstitutionOverview, error) {
	if from.IsZero() || to.IsZero() || to.Before(from) || from.DaysUntil(to) >= 56 {
		return nil, ErrSubstitutionInvalidPeriod
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
	return &SubstitutionOverview{
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

func projectSubstitutionAppointments(instances []*scheduleModel.ActivityInstance, deviations map[int64][]*scheduleModel.InstanceStaff, staffByID map[int64]*userModels.Staff, canManage bool) []SubstitutionAppointment {
	appointments := make([]SubstitutionAppointment, 0, len(deviations))
	for _, instance := range instances {
		rows := deviations[instance.ID]
		if len(rows) == 0 {
			continue
		}
		staff := make([]SubstitutionAppointmentStaff, 0, len(rows))
		canChange := canManage && !instance.Date.Before(timezone.TodayDate()) && isPlannableInstance(instance)
		for _, row := range rows {
			member := staffByID[row.StaffID]
			name := ""
			if member != nil {
				name = member.GetFullName()
			}
			staff = append(staff, SubstitutionAppointmentStaff{
				AssignmentID: row.ID,
				Staff:        SubstitutionStaffRef{ID: row.StaffID, FullName: name},
				IsAbsent:     row.IsAbsent,
				IsSubstitute: row.IsSubstitute,
				CanEnd:       canChange && row.IsSubstitute && !row.IsAbsent,
			})
		}
		appointments = append(appointments, SubstitutionAppointment{
			ID: instance.ID, Date: instance.Date,
			StartTime: instance.StartTime.Format("15:04"), EndTime: instance.EndTime.Format("15:04"),
			Title: instance.Title, Status: instance.Status, Staff: staff,
		})
	}
	return appointments
}

func (a *SubstitutionAdapter) loadTargets(ctx context.Context, include bool) ([]SubstitutionStaffRef, error) {
	targets := []SubstitutionStaffRef{}
	if !include {
		return targets, nil
	}
	members, err := a.deps.Staff.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	for _, member := range members {
		if member != nil {
			targets = append(targets, SubstitutionStaffRef{ID: member.ID, FullName: member.GetFullName()})
		}
	}
	return targets, nil
}

func (a *SubstitutionAdapter) ApplyAppointment(ctx context.Context, instanceID int64, input ApplyDeviationsInput) (*SubstitutionMutation, error) {
	result, err := a.deps.Engine.ApplyDeviations(ctx, instanceID, input)
	if err != nil {
		return nil, err
	}
	return &SubstitutionMutation{
		Appointment: result,
		AfterCommit: a.afterCommit(result.ActiveTouched, result.AppliedWrites > 0 || result.AckChanged || result.ClearedAcks > 0),
	}, nil
}

func (a *SubstitutionAdapter) ApplyWholeDays(ctx context.Context, input BulkSubstitutionInput) (*SubstitutionMutation, error) {
	result, err := a.deps.Engine.ApplyBulkSubstitution(ctx, input)
	if err != nil {
		return nil, err
	}
	return &SubstitutionMutation{
		WholeDays:   result,
		AfterCommit: a.afterCommit(result.ActiveTouched, result.AppliedWrites > 0 || result.ClearedAcks > 0),
	}, nil
}

func (a *SubstitutionAdapter) End(ctx context.Context, substitutionID, actorAccountID int64) (*SubstitutionMutation, error) {
	row, err := a.deps.InstanceStaff.FindByID(ctx, substitutionID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, ErrSubstitutionNotFound
		}
		return nil, err
	}
	if row == nil || !row.IsSubstitute {
		return nil, ErrSubstitutionNotFound
	}
	if row.IsAbsent {
		return nil, ErrSubstitutionNotRunning
	}
	selected := []int64{row.InstanceID}
	return a.ApplyAppointment(ctx, row.InstanceID, ApplyDeviationsInput{
		ActorAccountID: &actorAccountID,
		SubstitutionRemovals: []DeviationSubstitutionRemovalInput{{
			StaffID: row.StaffID, InstanceIDs: &selected,
		}},
	})
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
