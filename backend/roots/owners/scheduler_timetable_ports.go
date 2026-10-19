package owners

import (
	"context"
	"time"

	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// schedulerMaterializer hands Timetable's materialization to the Worker. The
// Worker's runs are the scheduler's materializations.
type schedulerMaterializer struct {
	timetable timetable.MaterializationCapability
}

func newSchedulerMaterializer(capability timetable.MaterializationCapability) schedulerPorts.TimetableMaterializer {
	if capability == nil {
		return nil
	}
	return schedulerMaterializer{timetable: capability}
}

func (m schedulerMaterializer) ResolveWindow(baseDate calendar.Date, weeksAhead int) (calendar.Date, calendar.Date) {
	return m.timetable.ResolveWindow(baseDate, weeksAhead)
}

func (m schedulerMaterializer) MaterializeScheduledWindow(ctx context.Context, from, to calendar.Date) (*schedulerPorts.MaterializationRun, error) {
	result, err := m.timetable.MaterializeForTenant(ctx, from, to, timetable.MaterializationSourceScheduler)
	if result == nil {
		return nil, err
	}
	return &schedulerPorts.MaterializationRun{
		InstancesCreated:          result.InstancesCreated,
		CandidatesSkippedExisting: result.CandidatesSkippedExisting,
		CandidatesRaced:           result.CandidatesRaced,
		DurationMS:                result.DurationMS,
	}, err
}

// schedulerTimetableCleanup hands Timetable's retention sweep to the Worker.
type schedulerTimetableCleanup struct {
	timetable timetable.TimetableCleanup
}

func newSchedulerTimetableCleanup(cleanup timetable.TimetableCleanup) schedulerPorts.TimetableCleanup {
	if cleanup == nil {
		return nil
	}
	return schedulerTimetableCleanup{timetable: cleanup}
}

func (c schedulerTimetableCleanup) CleanupExpiredTimetableData(ctx context.Context) (*schedulerPorts.TimetableCleanupRun, error) {
	result, err := c.timetable.CleanupExpiredTimetableData(ctx)
	if result == nil {
		return nil, err
	}
	return &schedulerPorts.TimetableCleanupRun{
		InstancesDeleted:  result.InstancesDeleted,
		ExceptionsDeleted: result.ExceptionsDeleted,
		StudentsAffected:  result.StudentsAffected,
		RetentionDays:     result.RetentionDays,
		DurationMS:        result.DurationMS,
	}, err
}

// schedulerAutoStart hands Timetable's automatic start to the Worker.
type schedulerAutoStart struct {
	timetable timetable.InstanceAutoStart
}

func newSchedulerAutoStart(autoStart timetable.InstanceAutoStart) schedulerPorts.InstanceAutoStart {
	if autoStart == nil {
		return nil
	}
	return schedulerAutoStart{timetable: autoStart}
}

func (a schedulerAutoStart) RunForTenant(ctx context.Context, now time.Time) (*schedulerPorts.AutoStartRun, error) {
	result, err := a.timetable.RunForTenant(ctx, now)
	if result == nil {
		return nil, err
	}
	return &schedulerPorts.AutoStartRun{
		Checked:         result.Checked,
		Started:         result.Started,
		SkippedNoStaff:  result.SkippedNoStaff,
		SkippedDuty:     result.SkippedDuty,
		SkippedConflict: result.SkippedConflict,
		SkippedMoved:    result.SkippedMoved,
		Failed:          result.Failed,
		DurationMS:      result.DurationMS,
	}, err
}

// schedulerAutoEnd hands Timetable's automatic end to the Worker.
type schedulerAutoEnd struct {
	timetable timetable.InstanceAutoEnd
}

func newSchedulerAutoEnd(autoEnd timetable.InstanceAutoEnd) schedulerPorts.InstanceAutoEnd {
	if autoEnd == nil {
		return nil
	}
	return schedulerAutoEnd{timetable: autoEnd}
}

func (a schedulerAutoEnd) RunForTenant(ctx context.Context, now time.Time, grace time.Duration) (*schedulerPorts.AutoEndRun, error) {
	result, err := a.timetable.RunForTenant(ctx, now, grace)
	if result == nil {
		return nil, err
	}
	return &schedulerPorts.AutoEndRun{
		Checked:           result.Checked,
		Completed:         result.Completed,
		SkippedConcurrent: result.SkippedConcurrent,
		Failed:            result.Failed,
		DurationMS:        result.DurationMS,
	}, err
}
