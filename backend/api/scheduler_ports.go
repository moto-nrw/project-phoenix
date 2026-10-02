package api

import (
	"context"
	"fmt"

	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/jobruntime"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/services/scheduler"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The bindings below hand the retained repositories and services to the
// scheduler's consumer-owned ports (#2746). The scheduler names no model,
// repository or runtime type, and the root imports none of the retained
// packages either: values flow through the composed graph and its named
// string types are converted where they are known.

// schedulerTenantRuntime runs the Worker's transactions in the root's tenant
// runtime.
func schedulerTenantRuntime(runtime apiCommon.TenantRuntime) (scheduler.TenantRuntime, error) {
	bound, err := jobruntime.New(runtime)
	if err != nil {
		return nil, err
	}
	return bound, nil
}

// verifySchedulerSettingKeys refuses a Worker whose scheduler reads a key
// the Settings Platform registry does not define, or preloads a secret.
func verifySchedulerSettingKeys() error {
	return verifyPreloadSettingKeys(scheduler.SettingKeys())
}

func verifyPreloadSettingKeys(keys []string) error {
	if err := settingsCompose.VerifyPreloadKeys(keys); err != nil {
		return fmt.Errorf("worker setting keys: %w", err)
	}
	return nil
}

// schedulerSettings binds the Settings Platform service and its cross-tenant
// batch read, so one query per minute preloads the polling settings of every
// school.
func schedulerSettings(api *API) (scheduler.SettingsResolver, error) {
	snapshots, err := settingsCompose.NewTenantSnapshots(api.Services.Settings)
	if err != nil {
		return nil, fmt.Errorf("worker settings snapshots: %w", err)
	}
	return schedulerSettingsPort{SettingsResolver: api.Services.Settings, snapshots: snapshots}, nil
}

type schedulerSettingsPort struct {
	scheduler.SettingsResolver
	snapshots settingsCompose.TenantSnapshots
}

func (p schedulerSettingsPort) ResolveSettingsSnapshots(ctx context.Context, tenantIDs []int64, keys []string) (map[int64]scheduler.SettingsSnapshot, error) {
	resolved, err := p.snapshots(ctx, tenantIDs, keys)
	if err != nil {
		return nil, err
	}
	snapshots := make(map[int64]scheduler.SettingsSnapshot, len(resolved))
	for tenantID, snapshot := range resolved {
		snapshots[tenantID] = snapshot
	}
	return snapshots, nil
}

func (p schedulerSettingsPort) BindSettingsSnapshot(ctx context.Context, snapshot scheduler.SettingsSnapshot) (context.Context, error) {
	bound, ok := snapshot.(settingsCompose.TenantSnapshot)
	if !ok {
		return ctx, fmt.Errorf("settings snapshot has type %T", snapshot)
	}
	return bound.Bind(ctx), nil
}

// schedulerBookingConsistency binds the booking drift audit.
func schedulerBookingConsistency(api *API) scheduler.BookingConsistencyAudit {
	bookings := api.repos.BookingConsistency
	if bookings == nil {
		return nil
	}
	return func(ctx context.Context, day calendar.Date) (*scheduler.BookingConsistencyReport, error) {
		report, err := callOnDay(bookings.Audit, ctx, day)
		if err != nil || report == nil {
			return nil, err
		}
		return &scheduler.BookingConsistencyReport{
			TenantID:                        report.TenantID,
			AuditDate:                       calendar.Date(report.AuditDate),
			PickupProjectionMissingDays:     report.PickupProjectionMissingDays,
			ApprovedWithoutRequiredOffering: report.ApprovedWithoutRequiredOffering,
			ApprovedWithoutOptionalOffering: report.ApprovedWithoutOptionalOffering,
			TotalFindings:                   report.TotalFindings(),
		}, nil
	}
}

// schedulerDayInstances binds the overdue tick to the day's activity
// instances with their composed status.
func schedulerDayInstances(api *API) scheduler.DayInstanceReader {
	instances := api.repos.ActivityInstance
	if instances == nil {
		return nil
	}
	return func(ctx context.Context, day calendar.Date) ([]scheduler.DayInstance, error) {
		rows, err := instances.FindByTenantAndDate(ctx, timetableCompose.ActivityInstanceDate(day))
		if err != nil {
			return nil, err
		}
		dayInstances := make([]scheduler.DayInstance, 0, len(rows))
		for _, row := range rows {
			if row == nil {
				continue
			}
			dayInstances = append(dayInstances, scheduler.DayInstance{
				ID:        row.ID,
				Date:      calendar.Date(row.Date),
				StartTime: row.StartTime,
				RoomID:    row.RoomID,
				Status:    row.Status,
			})
		}
		return dayInstances, nil
	}
}

// schedulerExistingRooms binds the overdue tick's room check.
func schedulerExistingRooms(api *API) scheduler.ExistingRoomReader {
	rooms := api.repos.Room
	if rooms == nil {
		return nil
	}
	return func(ctx context.Context, roomIDs []int64) ([]int64, error) {
		found, err := rooms.FindByIDs(ctx, roomIDs)
		if err != nil {
			return nil, err
		}
		existing := make([]int64, 0, len(found))
		for _, room := range found {
			if room != nil {
				existing = append(existing, room.ID)
			}
		}
		return existing, nil
	}
}

// schedulerStudentLifecycle binds the activate-students tick to the
// retained student repository.
func schedulerStudentLifecycle(api *API) scheduler.StudentLifecycleRepository {
	students := api.repos.Student
	if students == nil {
		return nil
	}
	return studentLifecyclePort{
		pending: func(ctx context.Context, asOf calendar.Date) ([]int64, error) {
			rows, err := students.FindPendingDueForActivation(ctx, asOf)
			ids := make([]int64, 0, len(rows))
			for _, row := range rows {
				if row != nil {
					ids = append(ids, row.ID)
				}
			}
			return ids, err
		},
		activeDue: func(ctx context.Context, asOf calendar.Date) ([]int64, error) {
			rows, err := students.FindActiveDueForDeactivation(ctx, asOf)
			ids := make([]int64, 0, len(rows))
			for _, row := range rows {
				if row != nil {
					ids = append(ids, row.ID)
				}
			}
			return ids, err
		},
		transition: func(ctx context.Context, studentID int64, expected, next string) (bool, error) {
			return transitionStudentStatus(students.TransitionStatus, ctx, studentID, expected, next)
		},
	}
}

type studentLifecyclePort struct {
	pending    func(context.Context, calendar.Date) ([]int64, error)
	activeDue  func(context.Context, calendar.Date) ([]int64, error)
	transition func(context.Context, int64, string, string) (bool, error)
}

func (p studentLifecyclePort) FindPendingDueForActivation(ctx context.Context, asOf calendar.Date) ([]int64, error) {
	return p.pending(ctx, asOf)
}

func (p studentLifecyclePort) FindActiveDueForDeactivation(ctx context.Context, asOf calendar.Date) ([]int64, error) {
	return p.activeDue(ctx, asOf)
}

func (p studentLifecyclePort) TransitionStatus(ctx context.Context, studentID int64, expected, next string) (bool, error) {
	return p.transition(ctx, studentID, expected, next)
}

// schedulerStudentAudit binds the history entry of an automated status
// transition.
func schedulerStudentAudit(api *API) scheduler.StudentLifecycleAuditor {
	audit := api.Services.StudentAudit
	if audit == nil {
		return nil
	}
	return studentAuditPort(func(ctx context.Context, studentID int64, before, after string) error {
		return recordStudentStatusChange(audit.RecordSystemStatusChange, ctx, studentID, before, after)
	})
}

type studentAuditPort func(context.Context, int64, string, string) error

func (p studentAuditPort) RecordSystemStatusChange(ctx context.Context, studentID int64, before, after string) error {
	return p(ctx, studentID, before, after)
}

// schedulerStudentChangeLogCleanup binds the change-history retention sweep.
func schedulerStudentChangeLogCleanup(api *API) scheduler.StudentChangeLogCleanup {
	service := api.Services.StudentChangeLogCleanup
	if service == nil {
		return nil
	}
	return func(ctx context.Context) (scheduler.StudentChangeLogCleanupResult, error) {
		result, err := service.CleanupExpiredChangeLog(ctx)
		if err != nil || result == nil {
			return scheduler.StudentChangeLogCleanupResult{}, err
		}
		return scheduler.StudentChangeLogCleanupResult{
			EditsDeleted:     result.EditsDeleted,
			StudentsAffected: result.StudentsAffected,
			RetentionDays:    result.RetentionDays,
			DurationMS:       result.DurationMS,
		}, nil
	}
}

// callOnDay passes a calendar day to a retained method whose date parameter
// is a named string type of a package the root does not import.
func callOnDay[D ~string, R any](call func(context.Context, D) (R, error), ctx context.Context, day calendar.Date) (R, error) {
	return call(ctx, D(day))
}

// transitionStudentStatus passes stored status values to the retained
// compare-and-set, whose status parameter is such a named string type.
func transitionStudentStatus[S ~string](transition func(context.Context, int64, S, S) (bool, error), ctx context.Context, studentID int64, expected, next string) (bool, error) {
	return transition(ctx, studentID, S(expected), S(next))
}

// recordStudentStatusChange passes stored status values to the retained
// status audit.
func recordStudentStatusChange[S ~string](record func(context.Context, int64, S, S) error, ctx context.Context, studentID int64, before, after string) error {
	return record(ctx, studentID, S(before), S(after))
}
