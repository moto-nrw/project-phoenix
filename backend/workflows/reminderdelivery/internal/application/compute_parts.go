// Package application computes visual-only staff reminders (issue #1457):
// upcoming pickups, overdue pickups, and activity starts. There is no sound and
// no push — the data is derived on request from schedules already in the system
// and rendered on the staff "Erinnerungen" page. All thresholds (which types
// are on, lead-time minutes) are tenant settings resolved at request time.
package application

import (
	"context"

	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

func (s *service) computePickupPart(ctx context.Context, scope reminder.Scope, today string, nowMin int, pickupUpcoming, pickupOverdue bool, nextChange int) (pickupPart []reminder.Reminder, next int, err error) {
	if pickupUpcoming || pickupOverdue {
		studentIDs, serr := s.pickupScopeStudentIDs(ctx, scope, today)
		if serr != nil {
			return nil, nextChange, serr
		}
		lead, lerr := s.leadMinutes(ctx, s.Settings.PickupLeadMinutes)
		if lerr != nil {
			return nil, nextChange, lerr
		}
		pickupReminders, pickupNext, perr := s.pickupReminders(ctx, scope, studentIDs, today, nowMin, lead, pickupUpcoming, pickupOverdue)
		if perr != nil {
			return nil, nextChange, perr
		}
		pickupPart = pickupReminders
		nextChange = minFuture(nextChange, pickupNext)
	}

	return pickupPart, nextChange, nil
}

// activityGates are the tenant's activity reminder switches.
type activityGates struct {
	start       bool
	overdue     bool
	runningOver bool
}

func (g activityGates) any() bool { return g.start || g.overdue || g.runningOver }

// activityGates resolves the activity switches in a fixed order. As with the
// pickup switches, a failed read surfaces instead of reading as "off".
func (s *service) activityGates(ctx context.Context) (activityGates, error) {
	var gates activityGates
	var err error
	if gates.start, err = s.Settings.ActivityStartEnabled(ctx); err != nil {
		return gates, err
	}
	if gates.overdue, err = s.Settings.ActivityOverdueEnabled(ctx); err != nil {
		return gates, err
	}
	if gates.runningOver, err = s.Settings.ActivityRunningOverEnabled(ctx); err != nil {
		return gates, err
	}
	return gates, nil
}

func (s *service) computeActivityPart(ctx context.Context, scope reminder.Scope, today string, nowMin int, gates activityGates, nextChange int) (activityPart []reminder.Reminder, next int, err error) {
	if !gates.any() {
		return nil, nextChange, nil
	}
	roomIDs, instances, err := s.activityInputs(ctx, scope, today)
	if err != nil {
		return nil, nextChange, err
	}
	if gates.start || gates.overdue {
		part, partNext, perr := s.startAndOverduePart(ctx, scope, roomIDs, instances, nowMin, gates)
		if perr != nil {
			return nil, nextChange, perr
		}
		activityPart = append(activityPart, part...)
		nextChange = minFuture(nextChange, partNext)
	}
	if gates.runningOver {
		part, partNext, rerr := s.runningOverPart(ctx, scope, roomIDs, today, nowMin)
		if rerr != nil {
			return nil, nextChange, rerr
		}
		activityPart = append(activityPart, part...)
		nextChange = minFuture(nextChange, partNext)
	}
	return activityPart, nextChange, nil
}

// activityInputs reads the caller's rooms and today's blocks once for every
// activity type. Admins see all activities (nil room list); caregivers are
// limited to the rooms they supervise. Without an instance reader the blocks
// stay empty, so the settings are still resolved and a broken read still
// fails the request.
func (s *service) activityInputs(ctx context.Context, scope reminder.Scope, today string) ([]int64, []*ports.ActivityInstance, error) {
	var roomIDs []int64
	if !scope.IsAdmin {
		rooms, err := s.supervisedRoomIDs(ctx, scope)
		if err != nil {
			return nil, nil, err
		}
		roomIDs = rooms
	}
	if s.Instance == nil {
		return roomIDs, nil, nil
	}
	instances, err := s.Instance.FindByTenantAndDate(ctx, today)
	if err != nil {
		return nil, nil, err
	}
	return roomIDs, instances, nil
}

func (s *service) runningOverPart(ctx context.Context, scope reminder.Scope, roomIDs []int64, today string, nowMin int) ([]reminder.Reminder, int, error) {
	grace, err := s.runningOverMinutes(ctx)
	if err != nil {
		return nil, -1, err
	}
	if s.Instance == nil {
		return nil, -1, nil
	}
	instances, err := s.Instance.FindActiveByTenant(ctx)
	if err != nil {
		return nil, -1, err
	}
	return s.runningOverReminders(ctx, scope, roomIDs, instances, today, nowMin, grace)
}

func (s *service) startAndOverduePart(ctx context.Context, scope reminder.Scope, roomIDs []int64, instances []*ports.ActivityInstance, nowMin int, gates activityGates) ([]reminder.Reminder, int, error) {
	lead, err := s.leadMinutes(ctx, s.Settings.ActivityLeadMinutes)
	if err != nil {
		return nil, -1, err
	}
	overdueThreshold, err := s.overdueThresholdMinutes(ctx)
	if err != nil {
		return nil, -1, err
	}
	if s.Instance == nil {
		return nil, -1, nil
	}
	return s.activityRemindersFor(ctx, scope, roomIDs, instances, nowMin, lead, overdueThreshold, gates.start, gates.overdue)
}

func (s *service) renderDuePickups(ctx context.Context, readable map[int64]*ports.Student, dues []duePickup, nextChange int) ([]reminder.Reminder, int, error) {
	dueIDs := make([]int64, len(dues))
	for i, d := range dues {
		dueIDs[i] = d.id
	}

	// Hydrate display names for the due students only (all already read-gated).
	infos, err := s.hydrateNames(ctx, readable, dueIDs)
	if err != nil {
		return nil, nextChange, err
	}

	out, dropped := buildPickupReminders(dues, infos)
	s.logDroppedPickups(dropped)
	return out, nextChange, nil
}

func activityNextBoundary(startMin, endMin int, w activityWindow, nextChange int) int {
	// Track the boundaries at which this instance's reminder state flips, so
	// the frontend can refetch exactly then instead of waiting for the poll:
	//   upcoming: enters the window at startMin-lead, leaves it at startMin+1
	//   overdue:  appears at startMin+overdueThreshold, disappears at endMin
	// The upcoming exit is startMin+1, NOT startMin: at exactly startMin the
	// diff is 0, which is still "upcoming" (diff >= 0); the reminder only
	// disappears the first minute startMin is in the past. Scheduling startMin
	// would refetch a minute early (nothing changed) and drop the real
	// startMin+1 boundary — same reasoning as the pickup flip below.
	if w.upcoming {
		nextChange = minFuture(nextChange, futureBoundary(startMin-w.lead, w.nowMin))
		nextChange = minFuture(nextChange, futureBoundary(startMin+1, w.nowMin))
	}
	if w.overdue {
		overdueStart := startMin + w.overdueThreshold
		if overdueStart < endMin {
			nextChange = minFuture(nextChange, futureBoundary(overdueStart, w.nowMin))
			nextChange = minFuture(nextChange, futureBoundary(endMin, w.nowMin))
		}
	}

	return nextChange
}
