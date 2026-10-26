package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

// Running-over reminders (#4765): a started block that is still running a
// configured number of minutes after its planned end. Children may still be
// in the room while their pickup is waiting, so the reminder names the block,
// its room and how many children in that room have reached their pickup time.
//
// The reminder disappears as soon as the block is completed: completed
// instances are no longer "active", which is a data change and reaches the
// client through the SSE-stale event like every other one.

// runningOverWindow bundles the time parameters of one running-over evaluation.
type runningOverWindow struct {
	nowMin int
	grace  int
	today  calendar.Date
}

// runningOverFacts are the reads a due running-over reminder is rendered from:
// the room names, the children currently in each room and their pickup times.
type runningOverFacts struct {
	roomNames    map[int64]string
	roomStudents map[int64][]int64
	pickupTimes  map[int64]*ports.EffectivePickupTime
}

// dueRunningOver selects the in-scope started blocks that are past their end
// by at least the grace period, and the soonest future moment another one
// will be. Pure, shared by Compute and ComputeBatch.
//
// A block without a room (a duty block, #3822) holds no children and is
// skipped. So is a spontaneous block: its planned end is a placeholder, and
// it runs until someone ends it.
func dueRunningOver(
	instances []*ports.ActivityInstance,
	inScope func(*ports.ActivityInstance) bool,
	w runningOverWindow,
) ([]*ports.ActivityInstance, int) {
	nextChange := -1
	due := make([]*ports.ActivityInstance, 0)
	for _, inst := range instances {
		if inst == nil || inst.Status != ports.InstanceStatusActive || inst.RoomID <= 0 || inst.IsSpontaneous {
			continue
		}
		if inScope != nil && !inScope(inst) {
			continue
		}
		elapsedDays := 0
		if w.today != "" && inst.Date != "" {
			if inst.Date.After(w.today) {
				continue
			}
			elapsedDays = inst.Date.DaysUntil(w.today)
		}
		threshold := minutesOfDay(inst.EndTime) + w.grace - elapsedDays*24*60
		if w.nowMin >= threshold {
			due = append(due, inst)
			continue
		}
		nextChange = minFuture(nextChange, futureBoundary(threshold, w.nowMin))
	}
	return due, nextChange
}

// runningOverRoomIDs collects the rooms of the given blocks.
func runningOverRoomIDs(due []*ports.ActivityInstance) map[int64]struct{} {
	rooms := make(map[int64]struct{}, len(due))
	for _, inst := range due {
		rooms[inst.RoomID] = struct{}{}
	}
	return rooms
}

// buildRunningOverReminders renders due blocks. Pure. A child counts once its
// pickup time is reached (pickup minute <= now); a child without a pickup
// time for today never counts.
func buildRunningOverReminders(due []*ports.ActivityInstance, today calendar.Date, nowMin int, facts runningOverFacts) []reminder.Reminder {
	out := make([]reminder.Reminder, 0, len(due))
	for _, inst := range due {
		count := 0
		for _, studentID := range facts.roomStudents[inst.RoomID] {
			effective := facts.pickupTimes[studentID]
			if effective == nil || effective.PickupTime == nil {
				continue
			}
			if minutesOfDay(*effective.PickupTime) <= nowMin {
				count++
			}
		}
		endMin := minutesOfDay(inst.EndTime)
		out = append(out, reminder.Reminder{
			Type:               reminder.TypeActivityRunningOver,
			ActivityInstanceID: int64String(inst.ID),
			Title:              inst.Title,
			Subtitle:           facts.roomNames[inst.RoomID],
			DueTime:            formatMinutes(endMin),
			MinutesAway:        runningOverMinutesAway(inst, today, nowMin),
			PickupDueCount:     &count,
		})
	}
	return out
}

func runningOverMinutesAway(inst *ports.ActivityInstance, today calendar.Date, nowMin int) int {
	if today == "" || inst.Date == "" {
		return minutesOfDay(inst.EndTime) - nowMin
	}
	return minutesOfDay(inst.EndTime) - nowMin - inst.Date.DaysUntil(today)*24*60
}

// loadRunningOverFacts reads what the due blocks are rendered from. Every
// room must resolve, matching the overdue guard: a reminder for a room that
// cannot be named would hide a misconfiguration.
func (s *service) loadRunningOverFacts(ctx context.Context, today string, due []*ports.ActivityInstance) (runningOverFacts, error) {
	facts := runningOverFacts{
		roomNames:    map[int64]string{},
		roomStudents: map[int64][]int64{},
		pickupTimes:  map[int64]*ports.EffectivePickupTime{},
	}
	if len(due) == 0 {
		return facts, nil
	}
	if s.Room == nil {
		return facts, errActivityRoomReaderMissing
	}
	wanted := runningOverRoomIDs(due)
	rooms, err := s.Room.FindByIDs(ctx, sortedIDs(wanted))
	if err != nil {
		return facts, fmt.Errorf("load activity reminder rooms: %w", err)
	}
	if err := resolveActivityRooms(rooms, wanted); err != nil {
		return facts, err
	}
	for _, room := range rooms {
		if room != nil {
			facts.roomNames[room.ID] = room.Name
		}
	}

	byRoom, err := s.Visits.ListOpenVisitStudentIDsByRoom(ctx)
	if err != nil {
		return facts, fmt.Errorf("load open visits by room: %w", err)
	}
	students := make(map[int64]struct{})
	for roomID := range wanted {
		facts.roomStudents[roomID] = byRoom[roomID]
		for _, id := range byRoom[roomID] {
			students[id] = struct{}{}
		}
	}
	if len(students) == 0 || s.Pickup == nil {
		return facts, nil
	}
	times, err := s.Pickup.GetBulkEffectivePickupTimesForDate(ctx, sortedIDs(students), today)
	if err != nil {
		return facts, err
	}
	facts.pickupTimes = times
	return facts, nil
}

// runningOverReminders is the single-caller path over today's loaded blocks:
// the started ones in the caller's rooms (every room for admins).
func (s *service) runningOverReminders(ctx context.Context, scope reminder.Scope, roomIDs []int64, instances []*ports.ActivityInstance, today string, nowMin, grace int) ([]reminder.Reminder, int, error) {
	w := runningOverWindow{nowMin: nowMin, grace: grace, today: calendar.Date(today)}
	due, nextChange := dueRunningOver(instances, activityScopeFilter(activityRoomFilter(scope, roomIDs), nil), w)
	facts, err := s.loadRunningOverFacts(ctx, today, due)
	if err != nil {
		return nil, nextChange, err
	}
	return buildRunningOverReminders(due, calendar.Date(today), nowMin, facts), nextChange, nil
}

// runningOverMinutes resolves the grace period after the planned end. As with
// leadMinutes, a resolution failure is surfaced and a non-positive value
// normalizes to the registry default.
func (s *service) runningOverMinutes(ctx context.Context) (int, error) {
	return s.leadMinutes(ctx, s.Settings.ActivityRunningOverMinutes)
}
