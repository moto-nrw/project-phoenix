package api

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// presenceHistoryDays is how far the presence history reaches back: the
// Statistik's default window "Letzte 30 Tage" up to yesterday. The room visits
// count from the day they were recorded, and the children's consent keeps
// them for 30 days (seedPrivacyConsentsStep), so the whole history stays
// inside the room section and the nightly cleanup.
const presenceHistoryDays = 29

// PastPresenceDay is one finished care day of one child: arrival and
// departure through a kiosk and, optionally, one room visit in a session.
type PastPresenceDay struct {
	StudentID         int64
	Date              calendar.Date
	DeviceID          int64
	CheckIn, CheckOut time.Time
	Visit             *PastVisit
}

// PastVisit is the room visit of a PastPresenceDay inside its session.
type PastVisit struct {
	ActivityGroupID, RoomID  int64
	SessionStart, SessionEnd time.Time
	Entry, Exit              time.Time
}

// PresenceHistoryWriter records past care days of one school (#5031). No API
// writes presence after the fact, so the dev tools bind it to the owner of
// the attendance directly; nil leaves the past without presence.
type PresenceHistoryWriter func(ctx context.Context, schoolID int64, days []PastPresenceDay) (int, error)

// presenceHistoryStatus is one planned absence, recorded through the regular
// status-day endpoint like a report of the parents.
type presenceHistoryStatus struct {
	studentID int64
	status    string
	dates     []calendar.Date
}

// presenceHistoryStatusDay is a status day the history step created and may
// therefore clear again when the remaining history cannot be recorded.
type presenceHistoryStatusDay struct {
	studentID, id int64
}

// presenceHistoryPlan is the deterministic past of the school: who came on
// which care day, who was sick or excused, and who stayed away unexplained.
type presenceHistoryPlan struct {
	days        []PastPresenceDay
	statuses    []presenceHistoryStatus
	unexplained int
}

// presenceHistoryInput is what the plan derives the past from.
type presenceHistoryInput struct {
	today    calendar.Date
	careDays func(calendar.Date) bool
	students []presenceHistoryStudent
	deviceID int64
	// sessions are the activities with their rooms, one per room.
	sessions []presenceHistorySession
}

// presenceHistorySession is an activity in its room; capacity weighs how many
// children spend the afternoon there.
type presenceHistorySession struct {
	activityID, roomID int64
	capacity           int
}

// presenceHistoryStudent is a child of the school; lastDay is its last care
// day when the school recorded an end of care.
type presenceHistoryStudent struct {
	id      int64
	lastDay *calendar.Date
}

// seedPresenceHistoryStep fills the Statistik, the room history of a child
// and of a room with a plausible past (#5031): on most care days a child
// comes, visits one room and goes home; sometimes it is sick or excused,
// rarely it stays away without a word. The days follow the Statistik's own
// care days, so the absences are its only missing days.
type seedPresenceHistoryStep struct{ seeder *Seeder }

func (seedPresenceHistoryStep) Name() string { return "Seeding presence history" }

func (s seedPresenceHistoryStep) Run(ctx context.Context, rt *Runtime) error {
	write := s.seeder.options.PresenceHistory
	if write == nil {
		fmt.Println("  no database for the presence history, past attendance skipped")
		return nil
	}
	input, err := presenceHistoryInputFor(rt, s.seeder.definition)
	if err != nil {
		return err
	}
	plan := planPresenceHistory(input)
	return seedPresenceHistoryPlan(ctx, rt, rt.Bootstrap.SchoolID, write, plan)
}

// seedPresenceHistoryPlan writes the two independently committed parts of a
// history seed. Status days use the normal HTTP write path, while attendance
// needs the dev-tools-only writer. Keep the IDs from the former so either a
// failed report or a rejected attendance batch restores a clean retry.
func seedPresenceHistoryPlan(ctx context.Context, rt *Runtime, schoolID int64, write PresenceHistoryWriter, plan presenceHistoryPlan) error {
	statusDays, createdStatusDays, err := seedPresenceHistoryStatusDays(rt, plan.statuses)
	if err != nil {
		return rollbackPresenceHistoryStatusDays(rt, createdStatusDays, err)
	}
	written, err := write(ctx, schoolID, plan.days)
	if err != nil {
		return rollbackPresenceHistoryStatusDays(rt, createdStatusDays, fmt.Errorf("seed presence history: %w", err))
	}
	fmt.Printf("  %d past care days attended, %d sick or excused, %d unexplained\n", written, statusDays, plan.unexplained)
	return nil
}

// seedPresenceHistoryStatusDays writes planned absences through the public
// route and returns exactly the rows this step created. The route reports all
// active rows in the requested range, so select only the requested status and
// dates before using the IDs for compensation.
func seedPresenceHistoryStatusDays(rt *Runtime, statuses []presenceHistoryStatus) (int, []presenceHistoryStatusDay, error) {
	rt.Client.BindAuth(rt.TenantAuth)
	statusDays := 0
	created := make([]presenceHistoryStatusDay, 0)
	for _, status := range statuses {
		dates := make([]string, 0, len(status.dates))
		for _, date := range status.dates {
			dates = append(dates, date.String())
		}
		raw, err := rt.Client.Post(fmt.Sprintf("/api/students/%d/status-days", status.studentID), map[string]any{
			"status": status.status, "dates": dates, "reason": "Von den Eltern gemeldet",
		})
		if err != nil {
			return statusDays, created, fmt.Errorf("seed past %s days: %w", status.status, err)
		}
		rows, err := createdPresenceHistoryStatusDays(raw, status)
		if err != nil {
			return statusDays, created, err
		}
		created = append(created, rows...)
		statusDays += len(dates)
	}
	return statusDays, created, nil
}

// createdPresenceHistoryStatusDays extracts the status-day IDs that the
// single-student endpoint just created. It deliberately rejects an incomplete
// response rather than risking a later compensation of an unrelated row.
func createdPresenceHistoryStatusDays(raw []byte, status presenceHistoryStatus) ([]presenceHistoryStatusDay, error) {
	var response struct {
		Data []struct {
			ID     int64  `json:"id"`
			Date   string `json:"date"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &response); err != nil {
		return nil, fmt.Errorf("read created past %s days: %w", status.status, err)
	}
	wanted := make(map[string]bool, len(status.dates))
	for _, date := range status.dates {
		wanted[date.String()] = true
	}
	created := make([]presenceHistoryStatusDay, 0, len(status.dates))
	for _, row := range response.Data {
		if row.ID == 0 || row.Status != status.status || !wanted[row.Date] {
			continue
		}
		created = append(created, presenceHistoryStatusDay{studentID: status.studentID, id: row.ID})
		delete(wanted, row.Date)
	}
	if len(wanted) != 0 {
		return nil, fmt.Errorf("read created past %s days: response missed %d status days", status.status, len(wanted))
	}
	return created, nil
}

// rollbackPresenceHistoryStatusDays clears only the status rows the current
// step created. The normal delete route retains the owner guards and makes a
// failed seed safely repeatable without deleting pre-existing absences.
func rollbackPresenceHistoryStatusDays(rt *Runtime, created []presenceHistoryStatusDay, seedErr error) error {
	var rollbackErrs []error
	for _, statusDay := range created {
		if _, err := rt.Client.Delete(fmt.Sprintf("/api/students/%d/status-days/%d", statusDay.studentID, statusDay.id)); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("clear seeded status day %d for student %d: %w", statusDay.id, statusDay.studentID, err))
		}
	}
	if len(rollbackErrs) == 0 {
		return seedErr
	}
	return errors.Join(append([]error{seedErr}, rollbackErrs...)...)
}

func presenceHistoryInputFor(rt *Runtime, definition demoProfileDefinition) (presenceHistoryInput, error) {
	if rt.FixedSeeder == nil || rt.Bootstrap == nil {
		return presenceHistoryInput{}, fmt.Errorf("presence history prerequisites not available")
	}
	today := calendar.TodayDate()
	from := today.AddDays(-presenceHistoryDays)
	closed, err := seedClosedDays(rt, from, today.AddDays(-1))
	if err != nil {
		return presenceHistoryInput{}, err
	}
	weekendCare := string(definition.Settings[profileSettingWeekendFollowsFriday].Value) == "true"
	input := presenceHistoryInput{
		today: today,
		careDays: func(day calendar.Date) bool {
			return (weekendCare || !calendar.IsWeekend(day)) && !closed[day]
		},
	}
	if input.deviceID, err = seedTerminalID(rt, DemoDevices[0].DeviceID); err != nil {
		return presenceHistoryInput{}, err
	}
	if input.students, err = listPresenceHistoryStudents(rt); err != nil {
		return presenceHistoryInput{}, err
	}
	input.sessions = presenceHistorySessions(rt.FixedSeeder)
	if input.deviceID == 0 || len(input.students) == 0 || len(input.sessions) == 0 {
		return presenceHistoryInput{}, fmt.Errorf("presence history prerequisites not available")
	}
	return input, nil
}

// presenceHistorySessions are the demo activities with their rooms, one
// activity per room.
func presenceHistorySessions(fixed *FixedSeeder) []presenceHistorySession {
	var sessions []presenceHistorySession
	rooms := map[int64]bool{}
	for _, activity := range DemoActivities {
		activityID := fixed.activityIDs[activity.Name]
		roomID := fixed.activityRoomIDs[activityID]
		if activityID == 0 || roomID == 0 || rooms[roomID] {
			continue
		}
		rooms[roomID] = true
		sessions = append(sessions, presenceHistorySession{activityID: activityID, roomID: roomID, capacity: demoRoomCapacity(activity.DefaultRoom)})
	}
	return sessions
}

// seedClosedDays reads the days the Statistik leaves out of the care days:
// public holidays, closing days and active holiday periods of the school.
func seedClosedDays(rt *Runtime, from, until calendar.Date) (map[calendar.Date]bool, error) {
	rt.Client.BindAuth(rt.TenantAuth)
	closed := map[calendar.Date]bool{}
	var holidays struct {
		Data []struct {
			Date string `json:"date"`
		} `json:"data"`
	}
	if err := getSeedJSON(rt, fmt.Sprintf("/api/time-tracking/holidays?from=%s&to=%s", from, until), &holidays); err != nil {
		return nil, fmt.Errorf("read public holidays: %w", err)
	}
	for _, holiday := range holidays.Data {
		if day, err := calendar.ParseDate(holiday.Date); err == nil {
			closed[day] = true
		}
	}
	for _, path := range []string{"/api/timetable/closing-days/", "/api/timetable/periods/"} {
		if err := closeSeedRanges(rt, path, closed, from, until); err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
	}
	return closed, nil
}

// closeSeedRanges closes the days of the closing days or of the active
// holiday periods; other periods keep the school open.
func closeSeedRanges(rt *Runtime, path string, closed map[calendar.Date]bool, from, until calendar.Date) error {
	var ranges struct {
		Data []struct {
			StartDate  string `json:"start_date"`
			EndDate    string `json:"end_date"`
			PeriodType string `json:"period_type"`
			IsActive   *bool  `json:"is_active"`
		} `json:"data"`
	}
	if err := getSeedJSON(rt, path, &ranges); err != nil {
		return err
	}
	for _, entry := range ranges.Data {
		if entry.PeriodType != "" && (entry.PeriodType != "holiday" || entry.IsActive == nil || !*entry.IsActive) {
			continue
		}
		if err := closeRange(closed, entry.StartDate, entry.EndDate, from, until); err != nil {
			return err
		}
	}
	return nil
}

// seedTerminalID reads the database ID of the kiosk that books the past
// days; zero when the seed created none.
func seedTerminalID(rt *Runtime, deviceID string) (int64, error) {
	devices, err := listSeedDevices(rt, "terminal")
	if err != nil {
		return 0, fmt.Errorf("read devices for the presence history: %w", err)
	}
	for _, device := range devices {
		if device.DeviceID == deviceID {
			return device.ID, nil
		}
	}
	return 0, nil
}

// listPresenceHistoryStudents reads every child of the school, also those the
// enrollment and parent steps added, in ID order so the plan stays stable.
func listPresenceHistoryStudents(rt *Runtime) ([]presenceHistoryStudent, error) {
	var students []presenceHistoryStudent
	for page := 1; ; page++ {
		var envelope struct {
			Data []struct {
				ID         int64  `json:"id"`
				CareEndsOn string `json:"care_ends_on"`
			} `json:"data"`
			Pagination struct {
				TotalPages int `json:"total_pages"`
			} `json:"pagination"`
		}
		if err := getSeedJSON(rt, fmt.Sprintf("/api/students?page=%d&page_size=100", page), &envelope); err != nil {
			return nil, fmt.Errorf("read students for the presence history: %w", err)
		}
		for _, row := range envelope.Data {
			student := presenceHistoryStudent{id: row.ID}
			if lastDay, err := calendar.ParseDate(row.CareEndsOn); err == nil {
				student.lastDay = &lastDay
			}
			students = append(students, student)
		}
		if len(envelope.Data) == 0 || page >= envelope.Pagination.TotalPages {
			break
		}
	}
	sort.Slice(students, func(i, j int) bool { return students[i].id < students[j].id })
	return students, nil
}

func getSeedJSON[T any](rt *Runtime, path string, target *T) error {
	raw, err := rt.Client.Get(path)
	if err != nil {
		return err
	}
	return parseJSON(raw, target)
}

func closeRange(closed map[calendar.Date]bool, rawStart, rawEnd string, from, until calendar.Date) error {
	start, err := calendar.ParseDate(rawStart)
	if err != nil {
		return err
	}
	end, err := calendar.ParseDate(rawEnd)
	if err != nil {
		return err
	}
	for day := start; !day.After(end); day = day.AddDays(1) {
		if day.Before(from) || day.After(until) {
			continue
		}
		closed[day] = true
	}
	return nil
}

// planPresenceHistory decides the past from a fixed hash of child and day,
// so a run on the same day always seeds the same history. Each child has its
// own absence rate between 4 and 14 per cent; an absence is sick, excused or,
// rarely, unexplained.
func planPresenceHistory(input presenceHistoryInput) presenceHistoryPlan {
	var plan presenceHistoryPlan
	berlin := seedBerlinLocation()
	statuses := map[int64]map[string][]calendar.Date{}
	for day := input.today.AddDays(-presenceHistoryDays); day.Before(input.today); day = day.AddDays(1) {
		if !input.careDays(day) {
			continue
		}
		for index, student := range input.students {
			if student.lastDay != nil && day.After(*student.lastDay) {
				continue
			}
			absent, status := plannedAbsence(index, day)
			switch {
			case !absent:
				plan.days = append(plan.days, pastPresenceDay(input, berlin, index, student.id, day))
			case status == "":
				plan.unexplained++
			default:
				addPresenceStatus(statuses, student.id, status, day)
			}
		}
	}
	plan.statuses = sortedPresenceStatuses(statuses)
	return plan
}

// plannedAbsence decides whether a child stays away on a day and, if so,
// how: sick, excused or unexplained (empty status).
func plannedAbsence(index int, day calendar.Date) (bool, string) {
	if presenceRoll(index, day, "absent")%100 >= uint32(4+(index*7)%11) {
		return false, ""
	}
	return true, absenceStatus(presenceRoll(index, day, "kind") % 100)
}

func addPresenceStatus(statuses map[int64]map[string][]calendar.Date, studentID int64, status string, day calendar.Date) {
	if statuses[studentID] == nil {
		statuses[studentID] = map[string][]calendar.Date{}
	}
	statuses[studentID][status] = append(statuses[studentID][status], day)
}

// absenceStatus splits the absences: a little over half sick, a third
// excused, the rest unexplained (empty).
func absenceStatus(roll uint32) string {
	switch {
	case roll < 55:
		return "sick"
	case roll < 85:
		return "excused"
	default:
		return ""
	}
}

// pastPresenceDay places a child between 11:45 and 12:44 at the OGS and lets
// it go home between 15:00 and 16:29; in between it spends the afternoon in
// one room, chosen by capacity, whose session runs from 12:00 to 16:45.
func pastPresenceDay(input presenceHistoryInput, berlin *time.Location, index int, studentID int64, day calendar.Date) PastPresenceDay {
	at := func(hour, minute int) time.Time {
		return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, berlin)
	}
	checkIn := at(11, 45).Add(time.Duration(presenceRoll(index, day, "in")%60) * time.Minute)
	checkOut := at(15, 0).Add(time.Duration(presenceRoll(index, day, "out")%90) * time.Minute)
	sessionStart, sessionEnd := at(12, 0), at(16, 45)
	entry := checkIn.Add(5 * time.Minute)
	if entry.Before(sessionStart) {
		entry = sessionStart
	}
	session := weightedSession(input.sessions, presenceRoll(index, day, "room"))
	return PastPresenceDay{
		StudentID: studentID, Date: day, DeviceID: input.deviceID, CheckIn: checkIn, CheckOut: checkOut,
		Visit: &PastVisit{
			ActivityGroupID: session.activityID, RoomID: session.roomID,
			SessionStart: sessionStart, SessionEnd: sessionEnd,
			Entry: entry, Exit: checkOut.Add(-5 * time.Minute),
		},
	}
}

// weightedSession picks a room in proportion to its capacity, so the rooms
// fill alike instead of the small ones overflowing.
func weightedSession(sessions []presenceHistorySession, roll uint32) presenceHistorySession {
	total := 0
	for _, session := range sessions {
		total += max(session.capacity, 1)
	}
	pick := int(roll % uint32(total))
	for _, session := range sessions {
		pick -= max(session.capacity, 1)
		if pick < 0 {
			return session
		}
	}
	return sessions[len(sessions)-1]
}

func demoRoomCapacity(name string) int {
	for _, room := range DemoRooms {
		if room.Name == name {
			return room.Capacity
		}
	}
	return 0
}

func sortedPresenceStatuses(statuses map[int64]map[string][]calendar.Date) []presenceHistoryStatus {
	result := make([]presenceHistoryStatus, 0, len(statuses))
	for studentID, byStatus := range statuses {
		for status, dates := range byStatus {
			result = append(result, presenceHistoryStatus{studentID: studentID, status: status, dates: dates})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].studentID != result[j].studentID {
			return result[i].studentID < result[j].studentID
		}
		return result[i].status < result[j].status
	})
	return result
}

// presenceRoll is a stable pseudo-random number for one decision about one
// child on one day.
func presenceRoll(index int, day calendar.Date, decision string) uint32 {
	hash := fnv.New32a()
	_, _ = fmt.Fprintf(hash, "%d|%s|%s", index, day, decision)
	return hash.Sum32()
}
