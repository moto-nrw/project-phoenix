package students

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	configService "github.com/moto-nrw/project-phoenix/services/config"
)

// attendanceHistoryResponse is the shape returned by GET /students/{id}/attendance-history.
//
// It groups attendance records by date and optionally includes a per-day room
// movement timeline (subject to a second, independent visibility cap).
type attendanceHistoryResponse struct {
	StudentID string                 `json:"student_id"`
	Days      []attendanceHistoryDay `json:"days"`
	Range     attendanceHistoryRange `json:"range"`
	Clamped   bool                   `json:"clamped"`
	Caps      attendanceHistoryCaps  `json:"caps"`
}

type attendanceHistoryRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type attendanceHistoryCaps struct {
	AttendanceDays int `json:"attendance_days"`
	RoomDetailDays int `json:"room_detail_days"`
}

type attendanceHistoryDay struct {
	Date                string                  `json:"date"` // YYYY-MM-DD
	Attendance          *attendanceDayRecord    `json:"attendance"`
	StatusEntries       []attendanceStatusEntry `json:"status_entries"`
	RoomDetailAvailable bool                    `json:"room_detail_available"`
	Visits              []attendanceVisitEntry  `json:"visits"`
	Slots               []attendanceSlotEntry   `json:"slots"`
}

type attendanceDayRecord struct {
	CheckInTime     time.Time                 `json:"check_in_time"`
	CheckOutTime    *time.Time                `json:"check_out_time,omitempty"`
	DurationMinutes *int                      `json:"duration_minutes,omitempty"`
	CheckedInBy     int64                     `json:"checked_in_by"`
	CheckedOutBy    *int64                    `json:"checked_out_by,omitempty"`
	DeviceID        int64                     `json:"device_id"`
	Sessions        []attendanceSessionRecord `json:"sessions"`
}

type attendanceSessionRecord struct {
	CheckInTime     time.Time  `json:"check_in_time"`
	CheckOutTime    *time.Time `json:"check_out_time,omitempty"`
	DurationMinutes *int       `json:"duration_minutes,omitempty"`
}

// attendanceSlotEntry is one care-offering slot in a day's history. Synthetic
// "Ohne Zuordnung" entries (observed sessions no slot could claim) carry a
// negative decimal instance_id sentinel unique within the day — consumers must not
// treat it as a real schedule.activity_instances ID.
type attendanceSlotEntry struct {
	InstanceID     string  `json:"instance_id"`
	InstanceStatus string  `json:"instance_status,omitempty"`
	Title          string  `json:"title"`
	StartTime      string  `json:"start_time"`
	EndTime        string  `json:"end_time"`
	Status         string  `json:"status"`
	Substatus      *string `json:"substatus,omitempty"`
	// Note is the free remark a supervisor recorded for this child in this
	// block. Carrying it here is what makes the entry readable after the day
	// is over (#2898): it is written in the live roster, and this history is
	// the only place that shows it again.
	Note         *string    `json:"note,omitempty"`
	CheckedInAt  *time.Time `json:"checked_in_at,omitempty"`
	CheckedOutAt *time.Time `json:"checked_out_at,omitempty"`
	IsUnplanned  bool       `json:"is_unplanned"`
}

type attendanceVisitEntry struct {
	RoomID          *int64     `json:"room_id,omitempty"`
	RoomName        string     `json:"room_name,omitempty"`
	EntryTime       time.Time  `json:"entry_time"`
	ExitTime        *time.Time `json:"exit_time,omitempty"`
	DurationMinutes *int       `json:"duration_minutes,omitempty"`
}

type attendanceStatusEntry struct {
	Status     string     `json:"status"`
	Label      string     `json:"label"`
	ReportedAt time.Time  `json:"reported_at"`
	ClearedAt  *time.Time `json:"cleared_at,omitempty"`
}

type attendanceHistorySources struct {
	Attendance []*active.Attendance
	Statuses   []*active.StudentStatusDay
	Slots      []*scheduleModel.ScheduledInstanceRow
	// SlotExpectation reports whether assignment hints apply to the loaded
	// range at all (see resolveSlotExpectation).
	SlotExpectation bool
}

// getStudentAttendanceHistory returns the daily attendance log and (conditionally)
// per-day room movement for a single student.
//
// This endpoint is opt-in per tenant (`gdpr.attendance_log_enabled`) and respects
// two independent visibility caps and a scope restriction (see settings system).
// Each successful read is recorded in audit.data_access_log for GDPR traceability.
func (rs *Resource) getStudentAttendanceHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := rs.attendanceHistoryLogger()

	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	// 1. Feature gate
	if !configService.ResolveBoolOrDefault(ctx, rs.SettingsService, configModel.KeyAttendanceLogEnabled, false, logger) {
		renderError(w, r, common.ErrorForbidden(errors.New("feature_disabled")))
		return
	}

	// 2. Identity check (admin or verified staff)
	if !rs.attendanceHistoryScopeAllows(r, student) {
		renderError(w, r, common.ErrorForbidden(errors.New("not_group_supervisor")))
		return
	}

	// 3. Resolve caps
	attendanceCap := configService.ResolveIntOrDefault(ctx, rs.SettingsService, configModel.KeyAttendanceVisibleDays, 30, logger)
	roomCap := configService.ResolveIntOrDefault(ctx, rs.SettingsService, configModel.KeyRoomDetailVisibleDays, 7, logger)
	if roomCap > attendanceCap {
		// Room detail cap must never exceed the attendance cap — silently clamp.
		roomCap = attendanceCap
	}

	// 4. Parse requested range and clamp against cap
	todayDate := rs.todayDate()
	today := todayDate.BerlinMidnight()
	endOfToday := todayDate.EndOfDay()
	defaultStart := today.AddDate(0, 0, -(attendanceCap - 1))

	start, end, err := parseAttendanceHistoryRange(r, defaultStart, endOfToday)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	start, end, clamped, err := clampAttendanceHistoryRange(start, end, endOfToday, attendanceCap)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// 5. Load attendance rows (DATE-keyed queries take the Berlin calendar
	// days of the requested instant range)
	startDay := timezone.DateFromTime(start)
	endDay := timezone.DateFromTime(end)
	sources, err := rs.loadAttendanceHistorySources(ctx, student.ID, startDay, endDay)
	if err != nil {
		logger.Error("attendance history source query failed",
			slog.Int64("student_id", student.ID),
			slog.String("error", err.Error()),
		)
		renderError(w, r, common.ErrorInternalServerWrap("failed to load attendance history", err))
		return
	}

	// 6. Conditionally load visits for days within the room-detail cap
	roomCutoff := today.AddDate(0, 0, -(roomCap - 1))
	visitsByDate, visitQueryFailed := rs.loadRoomVisitsByDate(ctx, student.ID, start, end, endOfToday, roomCutoff, roomCap, logger)

	// 7. Assemble per-day response
	days := buildAttendanceHistoryDays(sources.Attendance, sources.Statuses, visitsByDate, roomCutoff, visitQueryFailed)
	days = attachSlotAttendance(days, sources.Slots, visitsByDate, roomCutoff, visitQueryFailed)
	days = attachUnassignedAttendance(days, sources.SlotExpectation)

	resp := attendanceHistoryResponse{
		StudentID: strconv.FormatInt(student.ID, 10),
		Days:      days,
		Range:     attendanceHistoryRange{Start: start, End: end},
		Clamped:   clamped,
		Caps:      attendanceHistoryCaps{AttendanceDays: attendanceCap, RoomDetailDays: roomCap},
	}

	// 8. Audit write — GDPR requires a trace for every data access.
	// If we cannot write the audit log, we must not expose the data.
	if err := rs.writeAttendanceHistoryAudit(r, student.ID, start, end, logger); err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to record audit trail", err))
		return
	}

	common.Respond(w, r, http.StatusOK, resp, "Attendance history retrieved successfully")
}

func (rs *Resource) loadAttendanceHistorySources(
	ctx context.Context, studentID int64, from, to timezone.Date,
) (attendanceHistorySources, error) {
	sources := attendanceHistorySources{Statuses: []*active.StudentStatusDay{}}
	var err error
	sources.Attendance, err = rs.StudentHistoryService.GetAttendanceByStudentAndDateRange(ctx, studentID, from, to)
	if err != nil {
		return sources, err
	}
	if rs.StudentStatusDayService != nil {
		sources.Statuses, err = rs.StudentStatusDayService.GetByStudentAndDateRange(ctx, studentID, from, to)
		if err != nil {
			return sources, err
		}
	}
	sources.Slots, err = rs.StudentHistoryService.GetSlotAttendanceByStudentAndDateRange(ctx, studentID, from, to)
	if err != nil {
		return sources, err
	}
	sources.SlotExpectation, err = rs.resolveSlotExpectation(ctx, sources.Slots, from, to)
	return sources, err
}

// loadRoomVisitsByDate loads the per-day room movement timeline for the days
// inside the room-detail cap and groups it by calendar date. It returns the
// empty map when room detail is disabled (roomCap <= 0) or the requested range
// is entirely in the future; the second return reports whether the visit query
// failed, in which case the caller falls back to attendance-only days.
func (rs *Resource) loadRoomVisitsByDate(
	ctx context.Context, studentID int64, start, end, endOfToday, roomCutoff time.Time, roomCap int, logger *slog.Logger,
) (map[string][]*active.Visit, bool) {
	visitsByDate := map[string][]*active.Visit{}
	if roomCap <= 0 || start.After(endOfToday) {
		return visitsByDate, false
	}
	visitStart := start
	if roomCutoff.After(visitStart) {
		visitStart = roomCutoff
	}
	visits, err := rs.StudentHistoryService.GetVisitsByStudentAndTimeRange(ctx, studentID, visitStart, end)
	if err != nil {
		logger.Warn("visit history query failed, falling back to attendance-only",
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return visitsByDate, true
	}
	for _, v := range visits {
		key := timezone.DateOf(v.EntryTime).Format("2006-01-02")
		visitsByDate[key] = append(visitsByDate[key], v)
	}
	return visitsByDate, false
}

// resolveSlotExpectation reports whether assignment hints — the synthetic
// "Ohne Zuordnung" history entries and the export's offering/assignment
// columns — apply to the requested range. A planned slot row of the student
// answers it directly; otherwise the tenant-wide check decides, so a student
// without a single booking still gets the hint at a school that maintains its
// care plan (their observed sessions ARE valid unassigned cases there).
// Walk-in rows (is_unplanned) never count as plan evidence: a spontaneous
// drop-in also happens at schools that plan nothing.
func (rs *Resource) resolveSlotExpectation(
	ctx context.Context, slots []*scheduleModel.ScheduledInstanceRow, from, to timezone.Date,
) (bool, error) {
	if hasPlannedSlotRow(slots) {
		return true, nil
	}
	return rs.StudentHistoryService.HasPlannedSlotsInRange(ctx, from, to)
}

// hasPlannedSlotRow reports whether any usable slot row (instance and
// attendance present) carries a planned booking. Walk-ins are excluded on
// purpose — see resolveSlotExpectation. Cancelled instances are excluded too:
// their instance_students rows survive the cancellation, but a booking on a
// cancelled-only occurrence is no usable slot to report assignments against.
func hasPlannedSlotRow(slots []*scheduleModel.ScheduledInstanceRow) bool {
	for _, row := range slots {
		if row != nil && row.Instance != nil && row.Attendance != nil &&
			!row.Attendance.IsUnplanned && row.Instance.Status != scheduleModel.InstanceStatusCancelled {
			return true
		}
	}
	return false
}

// attendanceHistoryLogger returns a scoped logger, falling back to slog.Default.
func (rs *Resource) attendanceHistoryLogger() *slog.Logger {
	if rs.Logger != nil {
		return rs.Logger.With("handler", "attendance_history")
	}
	return slog.Default().With("handler", "attendance_history")
}

// attendanceHistoryScopeAllows returns true if the requesting caller may view
// this student's attendance history: admin or verified staff (#2329).
func (rs *Resource) attendanceHistoryScopeAllows(r *http.Request, student *users.Student) bool {
	return authorize.CanReadStudent(r.Context(), jwt.PermissionsFromCtx(r.Context()), student, rs.UserContextService)
}

// parseAttendanceHistoryRange reads start/end query params and falls back to
// defaults. Both are optional; start must be <= end.
func parseAttendanceHistoryRange(r *http.Request, defaultStart, defaultEnd time.Time) (time.Time, time.Time, error) {
	start := defaultStart
	end := defaultEnd

	if raw := strings.TrimSpace(r.URL.Query().Get("start")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid start parameter, expected RFC3339")
		}
		start = parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("end")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid end parameter, expected RFC3339")
		}
		end = parsed
	}
	if start.After(end) {
		return time.Time{}, time.Time{}, errors.New("start must be before end")
	}
	return start, end, nil
}

// clampAttendanceHistoryRange prevents future schedule materializations from
// appearing as history and applies the tenant's retrospective visibility cap.
func clampAttendanceHistoryRange(start, end, endOfToday time.Time, attendanceCap int) (time.Time, time.Time, bool, error) {
	clamped := false
	if end.After(endOfToday) {
		end = endOfToday
		clamped = true
	}
	if start.After(end) {
		return time.Time{}, time.Time{}, false, errors.New("start must not be in the future")
	}

	maxDuration := time.Duration(attendanceCap) * 24 * time.Hour
	if end.Sub(start) > maxDuration {
		start = end.Add(-maxDuration).Add(time.Second)
		clamped = true
	}
	return start, end, clamped, nil
}

// buildAttendanceHistoryDays groups attendance rows by calendar date and merges
// matching visit timelines. Multiple attendance rows on the same day (e.g. a
// checkout followed by a re-check-in) are consolidated into one day entry with
// the earliest check-in, latest check-out, and total duration.
// Days older than roomCutoff have RoomDetailAvailable=false.
// If visitQueryFailed is true, all days are marked as RoomDetailAvailable=false
// because the visit data could not be loaded.
func buildAttendanceHistoryDays(rows []*active.Attendance, statusRows []*active.StudentStatusDay, visitsByDate map[string][]*active.Visit, roomCutoff time.Time, visitQueryFailed bool) []attendanceHistoryDay {
	dayMap, dayOrder := groupAttendanceRowsByDate(rows)
	dayOrder = appendStatusDays(dayMap, dayOrder, statusRows)

	sort.SliceStable(dayOrder, func(i, j int) bool {
		return dayOrder[i] > dayOrder[j]
	})

	return assembleAttendanceHistoryDays(dayMap, dayOrder, visitsByDate, roomCutoff, visitQueryFailed)
}

// groupAttendanceRowsByDate groups attendance rows by calendar date, preserving
// first-seen order in dayOrder. Multiple rows on the same day are consolidated
// into one attendance record (see mergeAttendanceRow).
func groupAttendanceRowsByDate(rows []*active.Attendance) (map[string]*attendanceHistoryDay, []string) {
	dayOrder := make([]string, 0, len(rows))
	dayMap := make(map[string]*attendanceHistoryDay, len(rows))

	for _, row := range rows {
		dateKey := row.Date.String()
		existing, seen := dayMap[dateKey]
		if !seen {
			dayMap[dateKey] = newAttendanceHistoryDay(dateKey, row)
			dayOrder = append(dayOrder, dateKey)
			continue
		}
		mergeAttendanceRow(existing.Attendance, row)
	}
	return dayMap, dayOrder
}

func newAttendanceHistoryDay(dateKey string, row *active.Attendance) *attendanceHistoryDay {
	day := newEmptyAttendanceHistoryDay(dateKey)
	day.Attendance = &attendanceDayRecord{
		CheckInTime:  row.CheckInTime,
		CheckOutTime: row.CheckOutTime,
		CheckedInBy:  row.CheckedInBy,
		CheckedOutBy: row.CheckedOutBy,
		DeviceID:     row.DeviceID,
		Sessions:     []attendanceSessionRecord{newAttendanceSession(row)},
	}
	return day
}

func newEmptyAttendanceHistoryDay(dateKey string) *attendanceHistoryDay {
	return &attendanceHistoryDay{
		Date:          dateKey,
		StatusEntries: []attendanceStatusEntry{},
		Visits:        []attendanceVisitEntry{},
		Slots:         []attendanceSlotEntry{},
	}
}

// mergeAttendanceRow folds an additional same-day attendance row into an
// existing record: earliest check-in wins, latest check-out wins, and a nil
// check-out (still checked in) takes precedence over any completed one.
func mergeAttendanceRow(rec *attendanceDayRecord, row *active.Attendance) {
	rec.Sessions = append(rec.Sessions, newAttendanceSession(row))
	if row.CheckInTime.Before(rec.CheckInTime) {
		rec.CheckInTime = row.CheckInTime
		rec.CheckedInBy = row.CheckedInBy
		rec.DeviceID = row.DeviceID
	}
	if rec.CheckOutTime == nil {
		return
	}
	if row.CheckOutTime == nil {
		rec.CheckOutTime = nil
		rec.CheckedOutBy = nil
		return
	}
	if row.CheckOutTime.After(*rec.CheckOutTime) {
		rec.CheckOutTime = row.CheckOutTime
		rec.CheckedOutBy = row.CheckedOutBy
	}
}

// appendStatusDays attaches status entries (sick/excused) to their day,
// creating a status-only day when no attendance row exists for that date.
func appendStatusDays(dayMap map[string]*attendanceHistoryDay, dayOrder []string, statusRows []*active.StudentStatusDay) []string {
	for _, row := range statusRows {
		dateKey := row.Date.String()
		day, seen := dayMap[dateKey]
		if !seen {
			day = newEmptyAttendanceHistoryDay(dateKey)
			dayMap[dateKey] = day
			dayOrder = append(dayOrder, dateKey)
		}
		day.StatusEntries = append(day.StatusEntries, attendanceStatusEntry{
			Status:     row.Status,
			Label:      studentStatusLabel(row.Status),
			ReportedAt: row.ReportedAt,
			ClearedAt:  row.ClearedAt,
		})
	}
	return dayOrder
}

// assembleAttendanceHistoryDays materializes the ordered days, computing each
// day's duration and attaching room-detail visits within the retention cutoff.
func assembleAttendanceHistoryDays(dayMap map[string]*attendanceHistoryDay, dayOrder []string, visitsByDate map[string][]*active.Visit, roomCutoff time.Time, visitQueryFailed bool) []attendanceHistoryDay {
	days := make([]attendanceHistoryDay, 0, len(dayOrder))
	for _, dateKey := range dayOrder {
		day := dayMap[dateKey]

		calculateAttendanceDuration(day.Attendance)

		// Room detail cap: only include visits if this day is on/after the cutoff
		// and the visit query succeeded.
		date, _ := time.Parse("2006-01-02", dateKey)
		if !visitQueryFailed && !timezone.DateOf(date).Before(timezone.DateOf(roomCutoff)) {
			day.RoomDetailAvailable = true
			day.Visits = append(day.Visits, attendanceVisitEntries(visitsByDate[dateKey])...)
		}

		days = append(days, *day)
	}
	return days
}

// attendanceVisitEntries maps visit rows to their response shape.
func attendanceVisitEntries(visits []*active.Visit) []attendanceVisitEntry {
	entries := make([]attendanceVisitEntry, 0, len(visits))
	for _, v := range visits {
		entry := attendanceVisitEntry{
			EntryTime: v.EntryTime,
			ExitTime:  v.ExitTime,
		}
		if v.ActiveGroup != nil {
			roomID := v.ActiveGroup.RoomID
			entry.RoomID = &roomID
			if v.ActiveGroup.Room != nil {
				entry.RoomName = v.ActiveGroup.Room.Name
			}
		}
		if v.ExitTime != nil {
			mins := int(v.ExitTime.Sub(v.EntryTime).Minutes())
			entry.DurationMinutes = &mins
		}
		entries = append(entries, entry)
	}
	return entries
}

func newAttendanceSession(row *active.Attendance) attendanceSessionRecord {
	return attendanceSessionRecord{CheckInTime: row.CheckInTime, CheckOutTime: row.CheckOutTime}
}

func calculateAttendanceDuration(attendance *attendanceDayRecord) {
	if attendance == nil || attendance.CheckOutTime == nil {
		return
	}
	minutes := 0
	for i := range attendance.Sessions {
		session := &attendance.Sessions[i]
		if session.CheckOutTime == nil {
			continue
		}
		duration := int(session.CheckOutTime.Sub(session.CheckInTime).Minutes())
		session.DurationMinutes = &duration
		minutes += duration
	}
	attendance.DurationMinutes = &minutes
}

func attachSlotAttendance(
	days []attendanceHistoryDay,
	rows []*scheduleModel.ScheduledInstanceRow,
	visitsByDate map[string][]*active.Visit,
	roomCutoff time.Time,
	visitQueryFailed bool,
) []attendanceHistoryDay {
	index := make(map[string]int, len(days))
	for i := range days {
		index[days[i].Date] = i
	}
	cutoffDay := timezone.DateFromTime(roomCutoff)
	for _, row := range rows {
		if row == nil || row.Instance == nil || row.Attendance == nil {
			continue
		}
		date := row.Instance.Date.String()
		i, ok := index[date]
		if !ok {
			day := attendanceHistoryDay{
				Date: date, StatusEntries: []attendanceStatusEntry{},
				Visits: []attendanceVisitEntry{}, Slots: []attendanceSlotEntry{},
			}
			// Slot-only days obey the same room-detail retention rule as
			// attendance-backed days — within the window, room details are
			// available (and any visits for the date get attached).
			if !visitQueryFailed && !row.Instance.Date.Before(cutoffDay) {
				day.RoomDetailAvailable = true
				day.Visits = append(day.Visits, attendanceVisitEntries(visitsByDate[date])...)
			}
			days = append(days, day)
			i = len(days) - 1
			index[date] = i
		}
		days[i].Slots = append(days[i].Slots, attendanceSlotEntry{
			InstanceID:     strconv.FormatInt(row.Instance.ID, 10),
			InstanceStatus: row.Instance.Status,
			Title:          row.Instance.Title,
			StartTime:      row.Instance.StartTime.Format("15:04"),
			EndTime:        row.Instance.EndTime.Format("15:04"),
			Status:         row.Attendance.Status,
			Substatus:      row.Attendance.Substatus,
			Note:           row.Attendance.Note,
			CheckedInAt:    row.Attendance.CheckedInAt,
			CheckedOutAt:   row.Attendance.CheckedOutAt,
			IsUnplanned:    row.Attendance.IsUnplanned,
		})
	}
	sort.SliceStable(days, func(i, j int) bool { return days[i].Date > days[j].Date })
	return days
}

// attachUnassignedAttendance exposes observed sessions that could not be
// matched to a concrete slot. This is intentional in binary mode when zero or
// multiple booked slots overlap: the session stays neutrally unassigned
// instead of the system guessing an offering (or claiming it was unbooked).
//
// Synthetic entries appear only when expectSlots is true — the caller resolves
// it tenant-wide via resolveSlotExpectation. Without any planned slot there is
// nothing a session could have been assigned to, so "Ohne Zuordnung" would
// label every single day of a school that does not keep a care plan — a
// permanent error state for a system working as configured.
func attachUnassignedAttendance(days []attendanceHistoryDay, expectSlots bool) []attendanceHistoryDay {
	for dayIndex := range days {
		day := &days[dayIndex]
		if expectSlots && day.Attendance != nil {
			coverages := slotEntryCoverages(day.Slots)
			for sessionIndex, session := range day.Attendance.Sessions {
				if sessionCoveredBySlots(coverages, session.CheckInTime) {
					continue
				}
				day.Slots = append(day.Slots, unassignedSlotEntry(sessionIndex, session))
			}
		}

		// Scheduled slots display their planned start time; synthetic slots
		// display the observed check-in time in the same field. Sorting the
		// merged list here keeps the response chronological across both sources.
		sort.SliceStable(day.Slots, func(i, j int) bool {
			return day.Slots[i].StartTime < day.Slots[j].StartTime
		})
	}
	return days
}

// slotCoverage describes one slot's capacity to claim an observed attendance
// session: an exact check-in timestamp match (attendance check_in_time and
// slot checked_in_at share the same visit.EntryTime / roomless "now" stamp),
// or — for present slots — a scheduled wall-clock window that contains the
// session's Berlin check-in clock. The window fallback covers re-entry into
// the same slot, which re-stamps checked_in_at and orphans the earlier
// session's exact match.
type slotCoverage struct {
	checkInNano int64
	hasCheckIn  bool
	present     bool
	startClock  string // "15:04" Berlin wall clock; empty = no window
	endClock    string
}

func slotEntryCoverages(slots []attendanceSlotEntry) []slotCoverage {
	coverages := make([]slotCoverage, 0, len(slots))
	for _, slot := range slots {
		coverage := slotCoverage{
			present:    slot.Status == scheduleModel.AttendanceStatusPresent,
			startClock: slot.StartTime,
			endClock:   slot.EndTime,
		}
		if slot.CheckedInAt != nil {
			coverage.checkInNano = slot.CheckedInAt.UnixNano()
			coverage.hasCheckIn = true
		}
		coverages = append(coverages, coverage)
	}
	return coverages
}

func sessionCoveredBySlots(coverages []slotCoverage, checkIn time.Time) bool {
	nano := checkIn.UnixNano()
	clock := checkIn.In(timezone.Berlin).Format("15:04")
	for _, coverage := range coverages {
		if coverage.hasCheckIn && coverage.checkInNano == nano {
			return true
		}
		if coverage.present && coverage.startClock != "" && coverage.endClock != "" &&
			coverage.startClock <= clock && clock < coverage.endClock {
			return true
		}
	}
	return false
}

func unassignedSlotEntry(index int, session attendanceSessionRecord) attendanceSlotEntry {
	// Not flagged is_unplanned: whether a booking existed is unknown here
	// (zero or several candidate slots) — the title alone marks the row as
	// unassigned. Persisted walk-in slot rows carry is_unplanned instead.
	entry := attendanceSlotEntry{
		InstanceID: strconv.FormatInt(-int64(index+1), 10), Title: "Ohne Zuordnung",
		StartTime: session.CheckInTime.In(timezone.Berlin).Format("15:04"),
		Status:    "present", CheckedInAt: &session.CheckInTime,
		CheckedOutAt: session.CheckOutTime,
	}
	if session.CheckOutTime != nil {
		entry.EndTime = session.CheckOutTime.In(timezone.Berlin).Format("15:04")
	}
	return entry
}

func studentStatusLabel(status string) string {
	switch status {
	case active.StudentStatusDaySick:
		return "Krank"
	case active.StudentStatusDayExcused:
		return "Abgemeldet"
	default:
		return status
	}
}

// writeAttendanceHistoryAudit records a view event to audit.data_access_log.
// Returns an error if the audit trail cannot be written — callers must not
// expose the requested data without a successful audit record.
func (rs *Resource) writeAttendanceHistoryAudit(r *http.Request, studentID int64, start, end time.Time, logger *slog.Logger) error {
	if rs.StudentHistoryService == nil {
		logger.Error("audit log repo not configured, refusing to serve attendance history",
			slog.Int64("student_id", studentID),
		)
		return errors.New("audit log repository not configured")
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	actorAccountID := int64(claims.ID)
	actorRole := strings.Join(claims.Roles, ",")
	if actorRole == "" {
		actorRole = "unknown"
	}

	studentIDPtr := studentID
	entry := &auditModels.DataAccessLog{
		ActorAccountID: actorAccountID,
		ActorRole:      actorRole,
		ResourceType:   auditModels.ResourceTypeAttendanceHistory,
		StudentID:      &studentIDPtr,
		RangeStart:     start,
		RangeEnd:       end,
		AccessedAt:     time.Now(),
	}

	if err := rs.StudentHistoryService.RecordDataAccess(r.Context(), entry); err != nil {
		logger.Error("audit log write failed, refusing to serve attendance history",
			slog.Int64("student_id", studentID),
			slog.String("resource_type", auditModels.ResourceTypeAttendanceHistory),
			slog.String("error", err.Error()),
		)
		return err
	}
	return nil
}
