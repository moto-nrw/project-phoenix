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

type pastAttendanceKey struct {
	studentID int64
	date      calendar.Date
}

type pastVisitKey struct{ studentID, sessionID int64 }

type pastSessionExpectation struct {
	visit    studentpresence.PastVisit
	deviceID int64
}

type pastPresenceRecords struct {
	attendance map[pastAttendanceKey]bool
	sessions   map[pastSessionKey]studentpresence.LiveGroup
	visits     map[pastVisitKey][]studentpresence.Visit
}

type pastPresenceScope struct {
	attendance            map[pastAttendanceKey]studentpresence.PastPresenceDay
	sessions              map[pastSessionKey]pastSessionExpectation
	studentIDs            []int64
	fromDate, untilDate   calendar.Date
	fromVisit, untilVisit time.Time
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
	if len(days) == 0 {
		return 0, nil
	}
	if err := validatePastPresenceDays(days, calendar.TodayDate()); err != nil {
		return 0, err
	}
	err := w.tx.Run(ctx, func(txCtx context.Context) error {
		records, err := w.loadPastPresenceRecords(txCtx, days)
		if err != nil {
			return err
		}
		for _, day := range days {
			if err := w.recordPastDay(txCtx, day, &records); err != nil {
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
func (w *PresenceHistoryWriter) recordPastDay(ctx context.Context, day studentpresence.PastPresenceDay, records *pastPresenceRecords) error {
	if err := w.recordPastAttendance(ctx, day, records.attendance); err != nil {
		return err
	}
	if day.Visit == nil {
		return nil
	}
	return w.recordPastVisit(ctx, day, records)
}

func (w *PresenceHistoryWriter) recordPastAttendance(ctx context.Context, day studentpresence.PastPresenceDay, recorded map[pastAttendanceKey]bool) error {
	key := pastAttendanceKey{studentID: day.StudentID, date: day.Date}
	if recorded[key] {
		return nil
	}
	checkOut := day.CheckOut
	deviceID := day.DeviceID
	row := &ports.Attendance{StudentID: day.StudentID, Date: day.Date, CheckInTime: day.CheckIn,
		CheckOutTime: &checkOut, DeviceID: day.DeviceID, CheckedOutDeviceID: &deviceID}
	row.CreatedAt, row.UpdatedAt = day.CheckIn, checkOut
	if _, err := w.store.RecordAttendance(ctx, row); err != nil {
		return fmt.Errorf("record past attendance of student %d on %s: %w", day.StudentID, day.Date, err)
	}
	recorded[key] = true
	return nil
}

func (w *PresenceHistoryWriter) recordPastVisit(ctx context.Context, day studentpresence.PastPresenceDay, records *pastPresenceRecords) error {
	visit := day.Visit
	key := pastSessionKey{date: day.Date, activityGroupID: visit.ActivityGroupID, roomID: visit.RoomID}
	session, ok := records.sessions[key]
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
		records.sessions[key] = session
	} else if !session.StartTime.Equal(visit.SessionStart) || session.EndTime == nil || !session.EndTime.Equal(visit.SessionEnd) {
		return fmt.Errorf("%w: visits in room %d on %s name different session bounds", errPresenceHistory, visit.RoomID, day.Date)
	}
	visitKey := pastVisitKey{studentID: day.StudentID, sessionID: session.ID}
	if existing, ok := matchingPastVisit(records.visits[visitKey], visit); ok {
		if existing {
			return nil
		}
		return fmt.Errorf("%w: student %d has a different visit in room %d on %s", errPresenceHistory, day.StudentID, visit.RoomID, day.Date)
	}
	exit := visit.Exit
	row := &studentpresence.Visit{CreatedAt: visit.Entry, UpdatedAt: exit, StudentID: day.StudentID,
		ActiveGroupID: session.ID, EntryTime: visit.Entry, ExitTime: &exit}
	if _, err := w.store.RecordVisit(ctx, row); err != nil {
		return fmt.Errorf("record past visit of student %d on %s: %w", day.StudentID, day.Date, err)
	}
	records.visits[visitKey] = append(records.visits[visitKey], *row)
	return nil
}

// loadPastPresenceRecords recognizes only rows with the deterministic seed
// signature. A different existing record is left untouched and stops the
// batch instead of creating a duplicate beside it.
func (w *PresenceHistoryWriter) loadPastPresenceRecords(ctx context.Context, days []studentpresence.PastPresenceDay) (pastPresenceRecords, error) {
	records := pastPresenceRecords{attendance: map[pastAttendanceKey]bool{}, sessions: map[pastSessionKey]studentpresence.LiveGroup{}, visits: map[pastVisitKey][]studentpresence.Visit{}}
	scope := newPastPresenceScope(days)
	if err := w.loadPastAttendanceRecords(ctx, scope, &records); err != nil {
		return records, err
	}
	if err := w.loadPastSessionRecords(ctx, scope, &records); err != nil {
		return records, err
	}
	if err := w.loadPastVisitRecords(ctx, scope, &records); err != nil {
		return records, err
	}
	return records, nil
}

func (w *PresenceHistoryWriter) loadPastAttendanceRecords(ctx context.Context, scope pastPresenceScope, records *pastPresenceRecords) error {
	attendance, _, err := w.store.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: scope.studentIDs, FromDate: scope.fromDate.String(), UntilDate: scope.untilDate.String()})
	if err != nil {
		return fmt.Errorf("list past attendance: %w", err)
	}
	for key, day := range scope.attendance {
		for _, row := range attendance {
			if row.StudentID != key.studentID || row.Date != key.date {
				continue
			}
			if row.CheckOutTime == nil {
				return fmt.Errorf("%w: student %d already has open attendance on %s", errPresenceHistory, day.StudentID, day.Date)
			}
			if !matchesPastAttendance(row, day) || records.attendance[key] {
				return fmt.Errorf("%w: student %d already has different attendance on %s", errPresenceHistory, day.StudentID, day.Date)
			}
			records.attendance[key] = true
		}
	}
	return nil
}

func (w *PresenceHistoryWriter) loadPastSessionRecords(ctx context.Context, scope pastPresenceScope, records *pastPresenceRecords) error {
	if len(scope.sessions) == 0 {
		return nil
	}
	groups, _, err := w.store.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{ActivityGroupIDs: activityGroupIDs(scope.sessions), EndedOnly: true, From: &scope.fromVisit, Until: &scope.untilVisit})
	if err != nil {
		return fmt.Errorf("list past sessions: %w", err)
	}
	for key, expected := range scope.sessions {
		for _, group := range groups {
			if group.ActivityGroupID == nil || *group.ActivityGroupID != key.activityGroupID || group.RoomID != key.roomID || calendar.DateFromTime(group.StartTime) != key.date {
				continue
			}
			if !matchesPastSession(group, expected) || records.sessions[key].ID != 0 {
				return fmt.Errorf("%w: room %d already has a different session on %s", errPresenceHistory, key.roomID, key.date)
			}
			records.sessions[key] = group
		}
	}
	return nil
}

func (w *PresenceHistoryWriter) loadPastVisitRecords(ctx context.Context, scope pastPresenceScope, records *pastPresenceRecords) error {
	if len(scope.sessions) == 0 {
		return nil
	}
	visits, _, err := w.store.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: scope.studentIDs, EnteredFrom: &scope.fromVisit, EnteredUntil: &scope.untilVisit, ClosedOnly: true})
	if err != nil {
		return fmt.Errorf("list past visits: %w", err)
	}
	for _, visit := range visits {
		for _, session := range records.sessions {
			if visit.ActiveGroupID == session.ID {
				key := pastVisitKey{studentID: visit.StudentID, sessionID: session.ID}
				records.visits[key] = append(records.visits[key], *visit)
			}
		}
	}
	return nil
}

func newPastPresenceScope(days []studentpresence.PastPresenceDay) pastPresenceScope {
	scope := pastPresenceScope{attendance: make(map[pastAttendanceKey]studentpresence.PastPresenceDay, len(days)), sessions: make(map[pastSessionKey]pastSessionExpectation), studentIDs: studentIDs(days), fromDate: days[0].Date, untilDate: days[0].Date, fromVisit: days[0].CheckIn, untilVisit: days[0].CheckOut}
	for _, day := range days {
		key := pastAttendanceKey{studentID: day.StudentID, date: day.Date}
		scope.attendance[key] = day
		if day.Date.Before(scope.fromDate) {
			scope.fromDate = day.Date
		}
		if day.Date.After(scope.untilDate) {
			scope.untilDate = day.Date
		}
		if day.Visit == nil {
			continue
		}
		visit := *day.Visit
		sessionKey := pastSessionKey{date: day.Date, activityGroupID: visit.ActivityGroupID, roomID: visit.RoomID}
		scope.sessions[sessionKey] = pastSessionExpectation{visit: visit, deviceID: day.DeviceID}
		if visit.Entry.Before(scope.fromVisit) {
			scope.fromVisit = visit.Entry
		}
		if visit.Entry.After(scope.untilVisit) {
			scope.untilVisit = visit.Entry
		}
	}
	return scope
}

func studentIDs(days []studentpresence.PastPresenceDay) []int64 {
	ids := make([]int64, 0, len(days))
	seen := map[int64]bool{}
	for _, day := range days {
		if !seen[day.StudentID] {
			ids, seen[day.StudentID] = append(ids, day.StudentID), true
		}
	}
	return ids
}

func activityGroupIDs(sessions map[pastSessionKey]pastSessionExpectation) []int64 {
	ids := make([]int64, 0, len(sessions))
	seen := map[int64]bool{}
	for _, session := range sessions {
		if !seen[session.visit.ActivityGroupID] {
			ids, seen[session.visit.ActivityGroupID] = append(ids, session.visit.ActivityGroupID), true
		}
	}
	return ids
}

func matchesPastAttendance(row *ports.Attendance, day studentpresence.PastPresenceDay) bool {
	return row.CheckOutTime != nil && row.CheckInTime.Equal(day.CheckIn) && row.CheckOutTime.Equal(day.CheckOut) && row.DeviceID == day.DeviceID && row.CheckedOutDeviceID != nil && *row.CheckedOutDeviceID == day.DeviceID && row.CreatedAt.Equal(day.CheckIn) && row.UpdatedAt.Equal(day.CheckOut)
}

func matchesPastSession(session studentpresence.LiveGroup, expected pastSessionExpectation) bool {
	visit := expected.visit
	return session.EndTime != nil && session.DeviceID != nil && *session.DeviceID == expected.deviceID && session.StartTime.Equal(visit.SessionStart) && session.EndTime.Equal(visit.SessionEnd) && session.LastActivity.Equal(visit.SessionEnd) && session.CreatedAt.Equal(visit.SessionStart) && session.UpdatedAt.Equal(visit.SessionEnd)
}

func matchingPastVisit(rows []studentpresence.Visit, visit *studentpresence.PastVisit) (bool, bool) {
	for _, row := range rows {
		if row.ExitTime != nil && row.EntryTime.Equal(visit.Entry) && row.ExitTime.Equal(visit.Exit) && row.CreatedAt.Equal(visit.Entry) && row.UpdatedAt.Equal(visit.Exit) {
			return true, true
		}
	}
	return len(rows) != 0, false
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
