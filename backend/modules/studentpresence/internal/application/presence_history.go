package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// errPresenceHistory rejects a past day that no finished care day could have
// produced; nothing of the batch is written then.
var errPresenceHistory = errors.New("student presence: invalid presence history")

// pastSessionKey identifies the one session the visits of a day, activity and
// room share.
type pastSessionKey struct {
	date                    calendar.Date
	activityGroupID, roomID int64
}

// PresenceHistoryWriter writes finished care days of the past for the demo
// seed (#5031). It shares the owner's store and transaction but not the
// Service, so no production operation gains a caller.
type PresenceHistoryWriter struct {
	store ports.Store
	tx    ports.Transaction
}

func NewPresenceHistory(store ports.Store, tx ports.Transaction) *PresenceHistoryWriter {
	if store == nil || tx == nil {
		panic("student presence: all presence history dependencies are required")
	}
	return &PresenceHistoryWriter{store: store, tx: tx}
}

// RecordPresenceHistory writes finished care days before today in one
// transaction. Each row is created at the instant it describes, so the
// retention windows, which count from creation, treat it like a row recorded
// on that day.
func (w *PresenceHistoryWriter) RecordPresenceHistory(ctx context.Context, days []studentpresence.PastPresenceDay) (int, error) {
	if err := validatePastPresenceDays(days, calendar.TodayDate()); err != nil {
		return 0, err
	}
	err := w.tx.Run(ctx, func(txCtx context.Context) error {
		sessions := map[pastSessionKey]studentpresence.LiveGroup{}
		for _, day := range days {
			if err := w.recordPastDay(txCtx, day, sessions); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return len(days), nil
}

// recordPastDay writes the attendance of a day and its visit, if any; the
// visits of a day, activity and room share one session.
func (w *PresenceHistoryWriter) recordPastDay(ctx context.Context, day studentpresence.PastPresenceDay, sessions map[pastSessionKey]studentpresence.LiveGroup) error {
	if err := w.recordPastAttendance(ctx, day); err != nil {
		return err
	}
	if day.Visit == nil {
		return nil
	}
	return w.recordPastVisit(ctx, day, sessions)
}

func (w *PresenceHistoryWriter) recordPastAttendance(ctx context.Context, day studentpresence.PastPresenceDay) error {
	checkOut := day.CheckOut
	deviceID := day.DeviceID
	row := &ports.Attendance{StudentID: day.StudentID, Date: day.Date, CheckInTime: day.CheckIn,
		CheckOutTime: &checkOut, DeviceID: day.DeviceID, CheckedOutDeviceID: &deviceID}
	row.CreatedAt, row.UpdatedAt = day.CheckIn, checkOut
	if _, err := w.store.RecordAttendance(ctx, row); err != nil {
		return fmt.Errorf("record past attendance of student %d on %s: %w", day.StudentID, day.Date, err)
	}
	return nil
}

func (w *PresenceHistoryWriter) recordPastVisit(ctx context.Context, day studentpresence.PastPresenceDay, sessions map[pastSessionKey]studentpresence.LiveGroup) error {
	visit := day.Visit
	key := pastSessionKey{date: day.Date, activityGroupID: visit.ActivityGroupID, roomID: visit.RoomID}
	session, ok := sessions[key]
	if !ok {
		end := visit.SessionEnd
		activityID, deviceID := visit.ActivityGroupID, day.DeviceID
		recorded, _, err := w.store.RecordGroup(ctx, studentpresence.LiveGroup{
			CreatedAt: visit.SessionStart, UpdatedAt: end, StartTime: visit.SessionStart, EndTime: &end, LastActivity: end,
			ActivityGroupID: &activityID, DeviceID: &deviceID, RoomID: visit.RoomID,
		})
		if err != nil {
			return fmt.Errorf("record past session in room %d on %s: %w", visit.RoomID, day.Date, err)
		}
		session = recorded
		sessions[key] = session
	} else if !session.StartTime.Equal(visit.SessionStart) || session.EndTime == nil || !session.EndTime.Equal(visit.SessionEnd) {
		return fmt.Errorf("%w: visits in room %d on %s name different session bounds", errPresenceHistory, visit.RoomID, day.Date)
	}
	exit := visit.Exit
	row := &studentpresence.Visit{CreatedAt: visit.Entry, UpdatedAt: exit, StudentID: day.StudentID,
		ActiveGroupID: session.ID, EntryTime: visit.Entry, ExitTime: &exit}
	if _, err := w.store.RecordVisit(ctx, row); err != nil {
		return fmt.Errorf("record past visit of student %d on %s: %w", day.StudentID, day.Date, err)
	}
	return nil
}

func validatePastPresenceDays(days []studentpresence.PastPresenceDay, today calendar.Date) error {
	for _, day := range days {
		if err := validatePastPresenceDay(day, today); err != nil {
			return err
		}
	}
	return nil
}

// validatePastPresenceDay accepts only a closed day before today whose visit
// lies inside both the attendance and its session.
func validatePastPresenceDay(day studentpresence.PastPresenceDay, today calendar.Date) error {
	switch {
	case day.StudentID <= 0 || day.DeviceID <= 0:
		return fmt.Errorf("%w: student and device are required", errPresenceHistory)
	case day.Date == "" || !day.Date.Before(today):
		return fmt.Errorf("%w: %q is not a day before %s", errPresenceHistory, day.Date, today)
	case calendar.DateFromTime(day.CheckIn) != day.Date || calendar.DateFromTime(day.CheckOut) != day.Date:
		return fmt.Errorf("%w: attendance of student %d leaves %s", errPresenceHistory, day.StudentID, day.Date)
	case !day.CheckOut.After(day.CheckIn):
		return fmt.Errorf("%w: student %d leaves before arriving on %s", errPresenceHistory, day.StudentID, day.Date)
	}
	if day.Visit == nil {
		return nil
	}
	visit := day.Visit
	switch {
	case visit.ActivityGroupID <= 0 || visit.RoomID <= 0:
		return fmt.Errorf("%w: visit needs an activity and a room", errPresenceHistory)
	case !visit.Exit.After(visit.Entry) || !visit.SessionEnd.After(visit.SessionStart):
		return fmt.Errorf("%w: visit of student %d on %s ends before it starts", errPresenceHistory, day.StudentID, day.Date)
	case outside(visit.Entry, visit.Exit, visit.SessionStart, visit.SessionEnd) || outside(visit.Entry, visit.Exit, day.CheckIn, day.CheckOut):
		return fmt.Errorf("%w: visit of student %d on %s lies outside its session or attendance", errPresenceHistory, day.StudentID, day.Date)
	}
	return nil
}

func outside(start, end, from, until time.Time) bool { return start.Before(from) || end.After(until) }
