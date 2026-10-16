package api

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/scheduler"
)

// schedulerSessionMaintenance hands Student Presence's session maintenance
// to the Worker.
type schedulerSessionMaintenance struct {
	presence studentpresence.SessionMaintenance
}

func newSchedulerSessionMaintenance(presence studentpresence.SessionMaintenance) scheduler.SessionMaintenance {
	if presence == nil {
		return nil
	}
	return schedulerSessionMaintenance{presence: presence}
}

func (m schedulerSessionMaintenance) EndDailySessions(ctx context.Context) (*scheduler.DailySessionsEnded, error) {
	result, err := m.presence.EndDailySessions(ctx)
	if result == nil {
		return nil, err
	}
	return &scheduler.DailySessionsEnded{
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

func newSchedulerPresenceCleanup(presence studentpresence.PresenceCleanup) scheduler.PresenceCleanup {
	if presence == nil {
		return nil
	}
	return schedulerPresenceCleanup{presence: presence}
}

func (c schedulerPresenceCleanup) CleanupExpiredVisits(ctx context.Context) (*scheduler.ExpiredVisitCleanup, error) {
	result, err := c.presence.CleanupExpiredVisits(ctx)
	if result == nil {
		return nil, err
	}
	return &scheduler.ExpiredVisitCleanup{StudentsProcessed: result.StudentsProcessed, RecordsDeleted: result.RecordsDeleted, Success: result.Success}, err
}

func (c schedulerPresenceCleanup) CleanupStaleSupervisors(ctx context.Context) (*scheduler.StaleSupervisorCleanup, error) {
	result, err := c.presence.CleanupStaleSupervisors(ctx)
	if result == nil {
		return nil, err
	}
	return &scheduler.StaleSupervisorCleanup{RecordsClosed: result.RecordsClosed, Success: result.Success}, err
}

func (c schedulerPresenceCleanup) CleanupStaleAttendance(ctx context.Context) (*scheduler.StaleAttendanceCleanup, error) {
	result, err := c.presence.CleanupStaleAttendance(ctx)
	if result == nil {
		return nil, err
	}
	return &scheduler.StaleAttendanceCleanup{
		RecordsClosed:    result.RecordsClosed,
		StudentsAffected: result.StudentsAffected,
		Success:          result.Success,
		Errors:           result.Errors,
	}, err
}

// ArchiveEndOfDayStatusFlag archives the Worker's end-of-day status flag:
// the sick flag as a sick day, the excused flag as an excused day, both with
// the end-of-day source.
func (r StudentStatusDayRepository) ArchiveEndOfDayStatusFlag(ctx context.Context, flagColumn, sinceColumn string, date absencerecords.Date, fallback time.Time) (int64, error) {
	status, err := endOfDayStatusForFlag(flagColumn)
	if err != nil {
		return 0, err
	}
	return r.ArchiveAndClearStatusFlag(ctx, flagColumn, sinceColumn, status, date, fallback, absencerecords.StudentStatusSourceEndOfDay)
}

func endOfDayStatusForFlag(flagColumn string) (string, error) {
	switch flagColumn {
	case "sick":
		return absencerecords.StudentStatusDaySick, nil
	case "excused":
		return absencerecords.StudentStatusDayExcused, nil
	default:
		return "", fmt.Errorf("unsupported status flag column %q", flagColumn)
	}
}

// pwaUsageCleanup hands the Delivery Platform's standalone usage sweep to the
// Worker. A nil service leaves the job unregistered.
func pwaUsageCleanup(service deliveryCompose.PWAUsageService) scheduler.PWAUsageCleanup {
	if service == nil {
		return nil
	}
	return func(ctx context.Context) (scheduler.PWAUsageCleanupResult, error) {
		result, err := service.CleanupExpiredUsage(ctx)
		if result == nil {
			return scheduler.PWAUsageCleanupResult{}, err
		}
		return scheduler.PWAUsageCleanupResult{RowsDeleted: result.RowsDeleted, RetentionDays: result.RetentionDays}, err
	}
}

// schedulerOverdueBroadcaster sends the Worker's overdue announcements as the
// Delivery Platform's live events, to the whole school.
type schedulerOverdueBroadcaster struct {
	hub realtime.Broadcaster
}

func newSchedulerOverdueBroadcaster(hub *realtime.Hub) scheduler.InstanceOverdueBroadcaster {
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
