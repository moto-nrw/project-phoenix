package api

import (
	"context"
	"fmt"

	organizationModule "github.com/moto-nrw/project-phoenix/modules/organizationtenancy"

	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"

	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	schedulerRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/compose"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The bindings below hand the retained repositories and services to the
// scheduler's consumer-owned ports (#2746). The scheduler names no model,
// repository or runtime type, and the root imports none of the retained
// packages either: values flow through the composed graph and its named
// string types are converted where they are known.

// verifySchedulerSettingKeys refuses a Worker whose scheduler reads a key
// the Settings Platform registry does not define, or preloads a secret.
func verifySchedulerSettingKeys() error {
	return verifyPreloadSettingKeys(schedulerRuntimeCompose.SettingKeys())
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
func schedulerSettings(graph *serveGraph) (schedulerPorts.SettingsResolver, error) {
	snapshots, err := settingsCompose.NewTenantSnapshots(graph.services.Settings)
	if err != nil {
		return nil, fmt.Errorf("worker settings snapshots: %w", err)
	}
	return schedulerSettingsPort{SettingsResolver: graph.services.Settings, snapshots: snapshots}, nil
}

type schedulerSettingsPort struct {
	schedulerPorts.SettingsResolver
	snapshots settingsCompose.TenantSnapshots
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
	bound, ok := snapshot.(settingsCompose.TenantSnapshot)
	if !ok {
		return ctx, fmt.Errorf("settings snapshot has type %T", snapshot)
	}
	return bound.Bind(ctx), nil
}

// schedulerBookingConsistency binds the booking drift audit, which the root
// builds over the graph's People Directory and Care Plan (ADR 0049). A
// construction failure fails the Worker's composition.
func schedulerBookingConsistency(graph *serveGraph, reads workerReads) (schedulerPorts.BookingConsistencyAudit, error) {
	bookings, err := NewBookingConsistencyAudit(graph.db, graph.services.PeopleDirectory, reads.carePlan)
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
	instances := timetableRepositories.ActivityInstance
	if instances == nil {
		return nil
	}
	return func(ctx context.Context, day calendar.Date) ([]schedulerPorts.DayInstance, error) {
		rows, err := instances.FindByTenantAndDate(ctx, timetableCompose.ActivityInstanceDate(day))
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
				Date:      calendar.Date(row.Date),
				StartTime: row.StartTime,
				RoomID:    row.RoomID,
				Planned:   timetable.IsPlannedInstanceStatus(row.Status),
			})
		}
		return dayInstances, nil
	}
}

// workerReads are the Worker's retained room, work-session and Timetable
// reads. They are built over the owner modules where the graph is composed
// and handed to the Worker, not kept on the graph (#2743).
type workerReads struct {
	existingRooms schedulerPorts.ExistingRoomReader
	timetable     timetableCompose.Repositories
	workSessions  interface {
		GetTodayPresenceMap(context.Context) (map[int64]string, error)
	}
	// students and staff are the retained People Directory and School
	// Membership repositories the activation tick and the reminder
	// recipients read (#2743).
	students StudentRootReads
	staff    interface {
		ListAllStaffAccountIDs(ctx context.Context) (map[int64]int64, error)
	}
	// carePlan is the Care Plan owner the booking drift audit reads, and
	// studentStatusDays archives the status flags over it.
	carePlan          carePlanModule.Query
	studentStatusDays schedulerPorts.StudentStatusFlagArchiver
	// statusDayRetention deletes the expired status days in the nightly
	// cleanup (#3958).
	statusDayRetention carePlanModule.StudentStatusDayRetention
	// identity is the Identity & Access module the root composed (#4006):
	// the session, invitation and operator link maintenance and the reminder
	// recipients' accounts.
	identity *identityaccess.Module
	// schools are the schools the Worker iterates its tenant jobs over.
	schools organizationModule.Query
}

func newWorkerReads(modules moduleServices) workerReads {
	return workerReads{
		existingRooms: schedulerExistingRooms(modules.rooms),
		timetable:     modules.timetableRepositories,
		workSessions:  workforceCompose.WorkPresence(modules.workforce),
		students:      modules.students,
		staff:         modules.staff,
		// A nil owner leaves the interface nil, so the Worker runs no audit.
		carePlan:           carePlanQuery(modules.carePlan),
		studentStatusDays:  modules.studentStatusDays,
		statusDayRetention: statusDayRetention(modules.carePlan),
		identity:           modules.identity,
		schools:            modules.organizations,
	}
}

// statusDayRetention keeps a missing Care Plan owner a nil interface.
func statusDayRetention(module *carePlanModule.Module) carePlanModule.StudentStatusDayRetention {
	if module == nil {
		return nil
	}
	return module
}

// carePlanQuery keeps a missing Care Plan owner a nil interface.
func carePlanQuery(module *carePlanModule.Module) carePlanModule.Query {
	if module == nil {
		return nil
	}
	return module
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
func schedulerStudentLifecycle(reads workerReads) schedulerPorts.StudentLifecycleRepository {
	students := reads.students
	if students.PendingDueForActivation == nil {
		return nil
	}
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
func schedulerStudentAudit(graph *serveGraph) schedulerPorts.StudentLifecycleAuditor {
	audit := graph.services.StudentAudit
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
func schedulerStudentChangeLogCleanup(graph *serveGraph) schedulerPorts.StudentChangeLogCleanup {
	service := graph.services.StudentChangeLogCleanup
	if service == nil {
		return nil
	}
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
