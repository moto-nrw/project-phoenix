package owners

import (
	"context"
	"fmt"

	snapshotRecords "github.com/moto-nrw/project-phoenix/modules/settings/records"

	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"

	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	schedulerRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/compose"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// verifySchedulerSettingKeys refuses a Worker whose scheduler reads a key
// the Settings Platform registry does not define, or preloads a secret.
func verifySchedulerSettingKeys() error {
	return verifyPreloadSettingKeys(schedulerRuntimeCompose.SettingKeys())
}

func verifyPreloadSettingKeys(keys []string) error {
	if err := settingsCompose.NewPreloadKeyVerifier()(keys); err != nil {
		return fmt.Errorf("worker setting keys: %w", err)
	}
	return nil
}

// schedulerSettings binds the Settings Platform service and its cross-tenant
// batch read, so one query per minute preloads the polling settings of every
// school.
// verifiedSchedulerSettings checks the worker's setting keys against the
// registry before it builds the worker's settings port.
func verifiedSchedulerSettings(settings settingsCompose.BatchSettingsService) (schedulerPorts.SettingsResolver, error) {
	if err := verifySchedulerSettingKeys(); err != nil {
		return nil, err
	}
	return schedulerSettings(settings)
}

func schedulerSettings(settings settingsCompose.BatchSettingsService) (schedulerPorts.SettingsResolver, error) {
	snapshots, err := settingsCompose.NewTenantSnapshots(settings)
	if err != nil {
		return nil, fmt.Errorf("worker settings snapshots: %w", err)
	}
	return schedulerSettingsPort{SettingsResolver: settings, snapshots: snapshots}, nil
}

type schedulerSettingsPort struct {
	schedulerPorts.SettingsResolver
	// snapshots is the Settings Platform's cross-tenant batch read.
	snapshots func(ctx context.Context, tenantIDs []int64, keys []string) (map[int64]snapshotRecords.TenantSnapshot, error)
}

func (p schedulerSettingsPort) ResolveSettingsSnapshots(ctx context.Context, tenantIDs []int64, keys []string) (map[int64]schedulerPorts.SettingsSnapshot, error) {
	resolved, err := p.snapshots(ctx, tenantIDs, keys)
	if err != nil {
		return nil, err
	}
	snapshots := make(map[int64]schedulerPorts.SettingsSnapshot, len(resolved))
	for tenantID, snapshot := range resolved {
		snapshots[tenantID] = snapshot
	}
	return snapshots, nil
}

func (p schedulerSettingsPort) BindSettingsSnapshot(ctx context.Context, snapshot schedulerPorts.SettingsSnapshot) (context.Context, error) {
	bound, ok := snapshot.(snapshotRecords.TenantSnapshot)
	if !ok {
		return ctx, fmt.Errorf("settings snapshot has type %T", snapshot)
	}
	return bound.Bind(ctx), nil
}

// schedulerBookingConsistency binds the booking drift audit, which the root
// builds over the graph's People Directory and Care Plan (ADR 0049). A
// construction failure fails the Worker's composition.
func schedulerBookingConsistency(db transactionruntime.Database, persons peopledirectory.Capability, carePlan carePlanModule.Query) (schedulerPorts.BookingConsistencyAudit, error) {
	bookings, err := NewBookingConsistencyAudit(db, persons, carePlan)
	if err != nil {
		return nil, fmt.Errorf("compose worker booking consistency audit: %w", err)
	}
	return func(ctx context.Context, day calendar.Date) (*schedulerPorts.BookingConsistencyReport, error) {
		report, err := callOnDay(bookings, ctx, day)
		if err != nil || report == nil {
			return nil, err
		}
		return &schedulerPorts.BookingConsistencyReport{
			TenantID:                        report.TenantID,
			AuditDate:                       calendar.Date(report.AuditDate),
			PickupProjectionMissingDays:     report.PickupProjectionMissingDays,
			ApprovedWithoutRequiredOffering: report.ApprovedWithoutRequiredOffering,
			ApprovedWithoutOptionalOffering: report.ApprovedWithoutOptionalOffering,
			TotalFindings:                   report.TotalFindings(),
		}, nil
	}, nil
}

// schedulerDayInstances binds the overdue tick to the day's activity
// instances with their composed status.
func schedulerDayInstances(timetableRepositories timetableCompose.Repositories) schedulerPorts.DayInstanceReader {
	instances := timetableCompose.NewTimetableInstanceReads(timetableRepositories.ActivityInstance.Reads)
	return func(ctx context.Context, day calendar.Date) ([]schedulerPorts.DayInstance, error) {
		rows, err := instances.FindByTenantAndDateRange(ctx, day, day)
		if err != nil {
			return nil, err
		}
		dayInstances := make([]schedulerPorts.DayInstance, 0, len(rows))
		for _, row := range rows {
			if row == nil {
				continue
			}
			dayInstances = append(dayInstances, schedulerPorts.DayInstance{
				ID:        row.ID,
				Date:      row.Date,
				StartTime: row.StartTime,
				RoomID:    row.RoomID,
				Planned:   timetable.IsPlannedInstanceStatus(row.Status),
			})
		}
		return dayInstances, nil
	}
}

// schedulerExistingRooms binds the overdue tick's room check.
func schedulerExistingRooms(capability facilitiesModule.Capability) schedulerPorts.ExistingRoomReader {
	rooms := facilitiesCompose.NewRoomRepository(capability)
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
func schedulerStudentLifecycle(students StudentRootReads) schedulerPorts.StudentLifecycleRepository {
	return studentLifecyclePort{
		pending: students.PendingDueForActivation, activeDue: students.ActiveDueForDeactivation,
		transition: students.TransitionStatus,
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
func schedulerStudentAudit(audit peoplePorts.StudentAuditService) schedulerPorts.StudentLifecycleAuditor {
	return studentAuditPort(func(ctx context.Context, studentID int64, before, after string) error {
		return recordStudentStatusChange(audit.RecordSystemStatusChange, ctx, studentID, before, after)
	})
}

type studentAuditPort func(context.Context, int64, string, string) error

func (p studentAuditPort) RecordSystemStatusChange(ctx context.Context, studentID int64, before, after string) error {
	return p(ctx, studentID, before, after)
}

// schedulerStudentChangeLogCleanup binds the change-history retention sweep.
func schedulerStudentChangeLogCleanup(service peoplePorts.StudentChangeLogCleanupService) schedulerPorts.StudentChangeLogCleanup {
	return func(ctx context.Context) (schedulerPorts.StudentChangeLogCleanupResult, error) {
		result, err := service.CleanupExpiredChangeLog(ctx)
		if err != nil || result == nil {
			return schedulerPorts.StudentChangeLogCleanupResult{}, err
		}
		return schedulerPorts.StudentChangeLogCleanupResult{
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

// recordStudentStatusChange passes stored status values to the retained
// status audit.
func recordStudentStatusChange[S ~string](record func(context.Context, int64, S, S) error, ctx context.Context, studentID int64, before, after string) error {
	return record(ctx, studentID, S(before), S(after))
}
