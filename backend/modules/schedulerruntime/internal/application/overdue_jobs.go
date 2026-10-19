package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// scheduleInstanceOverdueTask registers the tick when all dependencies are
// wired. A partial setup registers no task, matching the other typed jobs'
// pattern so misconfiguration cannot emit overdue events for unresolved rooms.
func (s *Scheduler) scheduleInstanceOverdueTask() {
	if s.instanceRepo == nil || s.instanceRoomRepo == nil || s.overdueBroadcaster == nil {
		s.getLogger().Info("instance overdue tick not configured (missing instance repo, room repo, or broadcaster)")
		return
	}

	s.registerTask("instance-overdue", "1m-poll", s.runInstanceOverdueTaskPolling)
}

// runInstanceOverdueTaskPolling mirrors the minute-polling loops used by
// cleanup / session-end / materialization. Startup check + minute alignment
// + 60 s ticker + done-signal exit.
func (s *Scheduler) runInstanceOverdueTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in instance overdue task",
		"instance overdue tick using minute-polling",
		s.checkAndRunOverdue)
}

// checkAndRunOverdue rotates the day-cache when midnight has been crossed,
// then iterates active tenants and delegates the per-tenant work to
// runOverdueForTenant. Extracted into two methods so unit tests can invoke
// the inner loop directly with a synthetic tenant ctx without building the
// full school repo + settings service stack.
func (s *Scheduler) checkAndRunOverdue(ctx context.Context, task *ScheduledTask) {
	task.mu.Lock()
	if task.Running {
		task.mu.Unlock()
		return
	}
	task.Running = true
	task.mu.Unlock()
	defer func() {
		task.mu.Lock()
		task.Running = false
		task.mu.Unlock()
	}()

	s.rotateOverdueCacheIfNewDay(schedulerTime(s.now))

	ctx, cancel := s.taskContext(ctx, 5*time.Minute)
	defer cancel()

	s.forEachTenantSettings(ctx, "instance-overdue", func(tenantCtx context.Context, tenantID int64) error {
		threshold := s.resolveIntSetting(tenantCtx, settingTimetableOverdueThresholdMinutes, "", 5)
		s.runOverdueForTenant(tenantCtx, tenantID, threshold, time.Now())
		return nil
	})
}

// runOverdueForTenant iterates today's instances for a single tenant and
// emits instance_overdue once per still-planned, past-threshold row. `now`
// is injected for deterministic tests. `threshold` < 1 is a no-op.
func (s *Scheduler) runOverdueForTenant(ctx context.Context, tenantID int64, threshold int, now time.Time) {
	if threshold < 1 {
		return
	}

	today := calendar.DateFromTime(now)
	instances, err := s.instanceRepo(ctx, today)
	if err != nil {
		s.getLogger().Warn("overdue tick: load today's instances failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return
	}

	cutoff := time.Duration(threshold) * time.Minute
	candidates := make([]ports.DayInstance, 0, len(instances))
	roomIDs := make(map[int64]struct{})

	for _, inst := range instances {
		if !inst.Planned {
			continue
		}
		instanceStart := combineDayAndTime(today, inst.StartTime)
		if now.Sub(instanceStart) < cutoff {
			continue
		}
		candidates = append(candidates, inst)
		roomIDs[inst.RoomID] = struct{}{}
	}

	if !s.resolveOverdueRooms(ctx, tenantID, roomIDs) {
		return
	}

	for _, inst := range candidates {
		key := overdueKey{tenantID: tenantID, instanceID: inst.ID}
		if _, seen := s.overdueEmitted.Load(key); seen {
			continue
		}
		s.emitInstanceOverdue(ctx, tenantID, inst)
		s.overdueEmitted.Store(key, now)
	}
}

// emitInstanceOverdue builds the SSE envelope and fires it tenant-wide:
// a planned instance has no bridged active.group yet, so there is no group-
// scoped topic to route through. Admin dashboard subscribers pick it up.
func (s *Scheduler) emitInstanceOverdue(ctx context.Context, tenantID int64, inst ports.DayInstance) {
	instanceStart := inst.StartTime.Format("15:04:05")
	if err := s.overdueBroadcaster.BroadcastInstanceOverdue(tenantID, inst.ID, inst.Date.String(), instanceStart, inst.RoomID); err != nil {
		s.getLogger().Warn("overdue tick: broadcast failed",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("instance_id", inst.ID),
			slog.String("error", err.Error()),
		)
		return
	}
	if err := s.overdueBroadcaster.BroadcastInstanceOverdueRefresh(tenantID, inst.ID); err != nil {
		s.getLogger().Warn("overdue tick: active supervision broadcast failed",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("instance_id", inst.ID),
			slog.String("error", err.Error()),
		)
		return
	}
	s.getLogger().Info("instance_overdue broadcast",
		slog.Int64("tenant_id", tenantID),
		slog.Int64("instance_id", inst.ID),
		slog.String("start_time", instanceStart),
	)
	_ = ctx // reserved: if later the broadcaster needs ctx for cancellation.
}

// rotateOverdueCacheIfNewDay clears the re-fire guard on civil-date roll-
// over. Called at the top of every tick so a restart mid-day does not
// miss the rollover.
func (s *overdueEmissionState) rotateOverdueCacheIfNewDay(now time.Time) {
	today := calendar.DateFromTime(now)
	s.overdueEmittedDayMu.Lock()
	defer s.overdueEmittedDayMu.Unlock()
	if s.overdueEmittedDay != today {
		s.overdueEmitted.Range(func(k, _ any) bool {
			s.overdueEmitted.Delete(k)
			return true
		})
		s.overdueEmittedDay = today
	}
}

// combineDayAndTime returns "day at time-of-day", reading the wall-clock
// hour/minute/second from `tod` (typically an ActivityInstance.StartTime
// which lives as a bare TIME). Stays in the server's local zone so the
// comparison with time.Now() is apples-to-apples.
func combineDayAndTime(day calendar.Date, tod time.Time) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(),
		tod.Hour(), tod.Minute(), tod.Second(), tod.Nanosecond(), time.Local)
}

func (s *Scheduler) resolveOverdueRooms(ctx context.Context, tenantID int64, roomIDs map[int64]struct{}) bool {
	if len(roomIDs) > 0 {
		if s.instanceRoomRepo == nil {
			s.getLogger().Warn("overdue tick: room repository is not configured",
				slog.Int64("tenant_id", tenantID),
			)
			return false
		}
		ids := make([]int64, 0, len(roomIDs))
		for roomID := range roomIDs {
			ids = append(ids, roomID)
		}
		existingRoomIDs, err := s.instanceRoomRepo(ctx, ids)
		if err != nil {
			s.getLogger().Warn("overdue tick: load instance rooms failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			return false
		}
		resolvedRoomIDs := make(map[int64]struct{}, len(existingRoomIDs))
		for _, roomID := range existingRoomIDs {
			resolvedRoomIDs[roomID] = struct{}{}
		}
		for roomID := range roomIDs {
			if _, found := resolvedRoomIDs[roomID]; !found {
				s.getLogger().Warn("overdue tick: instance room could not be resolved",
					slog.Int64("tenant_id", tenantID),
					slog.Int64("room_id", roomID),
				)
				return false
			}
		}
	}
	return true
}
