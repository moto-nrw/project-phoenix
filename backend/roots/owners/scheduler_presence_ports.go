package owners

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// schedulerSessionMaintenance hands Student Presence's session maintenance
// to the Worker.
type schedulerSessionMaintenance struct {
	presence studentpresence.SessionMaintenance
}

func newSchedulerSessionMaintenance(presence studentpresence.SessionMaintenance) schedulerPorts.SessionMaintenance {
	if presence == nil {
		return nil
	}
	return schedulerSessionMaintenance{presence: presence}
}

func (m schedulerSessionMaintenance) EndDailySessions(ctx context.Context) (*schedulerPorts.DailySessionsEnded, error) {
	result, err := m.presence.EndDailySessions(ctx)
	if result == nil {
		return nil, err
	}
	return &schedulerPorts.DailySessionsEnded{
		SessionsEnded:       result.SessionsEnded,
		VisitsEnded:         result.VisitsEnded,
		SupervisorsEnded:    result.SupervisorsEnded,
		EndedActiveGroupIDs: result.EndedActiveGroupIDs,
		Success:             result.Success,
		Errors:              result.Errors,
	}, err
}

func (m schedulerSessionMaintenance) CleanupAbandonedSessions(ctx context.Context, olderThan time.Duration) (int, error) {
	return m.presence.CleanupAbandonedSessions(ctx, olderThan)
}

// schedulerPresenceCleanup hands Student Presence's nightly cleanup to the
// Worker.
type schedulerPresenceCleanup struct {
	presence studentpresence.PresenceCleanup
}

func newSchedulerPresenceCleanup(presence studentpresence.PresenceCleanup) schedulerPorts.PresenceCleanup {
	if presence == nil {
		return nil
	}
	return schedulerPresenceCleanup{presence: presence}
}

func (c schedulerPresenceCleanup) CleanupExpiredVisits(ctx context.Context) (*schedulerPorts.ExpiredVisitCleanup, error) {
	result, err := c.presence.CleanupExpiredVisits(ctx)
	if result == nil {
		return nil, err
	}
	return &schedulerPorts.ExpiredVisitCleanup{StudentsProcessed: result.StudentsProcessed, RecordsDeleted: result.RecordsDeleted, Success: result.Success}, err
}

func (c schedulerPresenceCleanup) CleanupStaleSupervisors(ctx context.Context) (*schedulerPorts.StaleSupervisorCleanup, error) {
	result, err := c.presence.CleanupStaleSupervisors(ctx)
	if result == nil {
		return nil, err
	}
	return &schedulerPorts.StaleSupervisorCleanup{RecordsClosed: result.RecordsClosed, Success: result.Success}, err
}

func (c schedulerPresenceCleanup) CleanupStaleAttendance(ctx context.Context) (*schedulerPorts.StaleAttendanceCleanup, error) {
	result, err := c.presence.CleanupStaleAttendance(ctx)
	if result == nil {
		return nil, err
	}
	return &schedulerPorts.StaleAttendanceCleanup{
		RecordsClosed:    result.RecordsClosed,
		StudentsAffected: result.StudentsAffected,
		Success:          result.Success,
		Errors:           result.Errors,
	}, err
}

// ArchiveEndOfDayStatusFlag archives the Worker's end-of-day status flag the
// way Care Plan maps it to a status day.
func (r StudentStatusDayRepository) ArchiveEndOfDayStatusFlag(ctx context.Context, flagColumn, sinceColumn string, date absencerecords.Date, fallback time.Time) (int64, error) {
	archive, err := careplan.EndOfDayStatusFlagArchive(flagColumn, sinceColumn, careplan.Date(date), fallback)
	if err != nil {
		return 0, err
	}
	return r.Capability.ArchiveStudentStatusFlags(ctx, archive)
}

// pwaUsageCleanup hands the Delivery Platform's standalone usage sweep to the
// Worker. A nil service leaves the job unregistered.
func pwaUsageCleanup(service deliveryCompose.PWAUsageService) schedulerPorts.PWAUsageCleanup {
	if service == nil {
		return nil
	}
	return func(ctx context.Context) (schedulerPorts.PWAUsageCleanupResult, error) {
		result, err := service.CleanupExpiredUsage(ctx)
		if result == nil {
			return schedulerPorts.PWAUsageCleanupResult{}, err
		}
		return schedulerPorts.PWAUsageCleanupResult{RowsDeleted: result.RowsDeleted, RetentionDays: result.RetentionDays}, err
	}
}

// schedulerOverdueBroadcaster sends the Worker's overdue announcements as the
// Delivery Platform's live events, to the whole school.
type schedulerOverdueBroadcaster struct {
	hub realtime.Broadcaster
}

func newSchedulerOverdueBroadcaster(hub *realtime.Hub) schedulerPorts.InstanceOverdueBroadcaster {
	if hub == nil {
		return nil
	}
	return schedulerOverdueBroadcaster{hub: hub}
}

func (b schedulerOverdueBroadcaster) BroadcastInstanceOverdue(tenantID, instanceID int64, date, startTime string, roomID int64) error {
	return b.hub.BroadcastToTenant(tenantID, realtime.NewInstanceOverdueEvent(instanceID, date, startTime, roomID))
}

func (b schedulerOverdueBroadcaster) BroadcastInstanceOverdueRefresh(tenantID, instanceID int64) error {
	return b.hub.BroadcastToTenant(tenantID, realtime.NewInstanceOverdueRefreshEvent(instanceID))
}
