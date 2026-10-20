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

func (s *service) computeActivityPart(ctx context.Context, scope reminder.Scope, today string, nowMin int, activityStart, activityOverdue bool, nextChange int) (activityPart []reminder.Reminder, next int, err error) {
	if activityStart || activityOverdue {
		// Admins see all activities (nil filter); caregivers are limited to the
		// rooms they supervise.
		var roomIDs []int64
		if !scope.IsAdmin {
			rooms, rerr := s.supervisedRoomIDs(ctx, scope)
			if rerr != nil {
				return nil, nextChange, rerr
			}
			roomIDs = rooms
		}
		lead, lerr := s.leadMinutes(ctx, s.Settings.ActivityLeadMinutes)
		if lerr != nil {
			return nil, nextChange, lerr
		}
		overdueThreshold, oerr := s.overdueThresholdMinutes(ctx)
		if oerr != nil {
			return nil, nextChange, oerr
		}
		activityReminders, activityNext, aerr := s.activityReminders(ctx, scope, roomIDs, today, nowMin, lead, overdueThreshold, activityStart, activityOverdue)
		if aerr != nil {
			return nil, nextChange, aerr
		}
		activityPart = activityReminders
		nextChange = minFuture(nextChange, activityNext)
	}

	return activityPart, nextChange, nil
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
