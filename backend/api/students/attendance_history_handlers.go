package students

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
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
	Date                string                 `json:"date"` // YYYY-MM-DD
	Attendance          *attendanceDayRecord   `json:"attendance"`
	RoomDetailAvailable bool                   `json:"room_detail_available"`
	Visits              []attendanceVisitEntry `json:"visits"`
}

type attendanceDayRecord struct {
	CheckInTime     time.Time  `json:"check_in_time"`
	CheckOutTime    *time.Time `json:"check_out_time,omitempty"`
	DurationMinutes *int       `json:"duration_minutes,omitempty"`
	CheckedInBy     int64      `json:"checked_in_by"`
	CheckedOutBy    *int64     `json:"checked_out_by,omitempty"`
	DeviceID        int64      `json:"device_id"`
}

type attendanceVisitEntry struct {
	RoomID          *int64     `json:"room_id,omitempty"`
	RoomName        string     `json:"room_name,omitempty"`
	EntryTime       time.Time  `json:"entry_time"`
	ExitTime        *time.Time `json:"exit_time,omitempty"`
	DurationMinutes *int       `json:"duration_minutes,omitempty"`
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
		renderError(w, r, ErrorForbidden(errors.New("feature_disabled")))
		return
	}

	// 2. Scope check
	scope := configService.ResolveStringOrDefault(ctx, rs.SettingsService, configModel.KeyAttendanceLogScope, configModel.AttendanceLogScopeGroupSupervisorsOnly, logger)
	if !rs.attendanceHistoryScopeAllows(r, student, scope) {
		renderError(w, r, ErrorForbidden(errors.New("not_group_supervisor")))
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
	today := timezone.Today()
	endOfToday := today.Add(24 * time.Hour).Add(-time.Second)
	defaultStart := today.AddDate(0, 0, -(attendanceCap - 1))

	start, end, err := parseAttendanceHistoryRange(r, defaultStart, endOfToday)
	if err != nil {
		renderError(w, r, ErrorInvalidRequest(err))
		return
	}

	clamped := false
	maxDuration := time.Duration(attendanceCap) * 24 * time.Hour
	if end.Sub(start) > maxDuration {
		start = end.Add(-maxDuration).Add(time.Second)
		clamped = true
	}

	// 5. Load attendance rows
	attendanceRows, err := rs.AttendanceRepo.FindByStudentAndDateRange(ctx, student.ID, start, end)
	if err != nil {
		logger.Error("attendance history query failed",
			slog.Int64("student_id", student.ID),
			slog.String("error", err.Error()),
		)
		renderError(w, r, ErrorInternalServer(errors.New("failed to load attendance history")))
		return
	}

	// 6. Conditionally load visits for days within the room-detail cap
	roomCutoff := today.AddDate(0, 0, -(roomCap - 1))
	visitsByDate := map[string][]*active.Visit{}
	if roomCap > 0 && !start.After(endOfToday) {
		visitStart := start
		if roomCutoff.After(visitStart) {
			visitStart = roomCutoff
		}
		visits, visitErr := rs.VisitRepo.FindByStudentAndTimeRange(ctx, student.ID, visitStart, end)
		if visitErr != nil {
			logger.Warn("visit history query failed, falling back to attendance-only",
				slog.Int64("student_id", student.ID),
				slog.String("error", visitErr.Error()),
			)
		} else {
			for _, v := range visits {
				key := timezone.DateOf(v.EntryTime).Format("2006-01-02")
				visitsByDate[key] = append(visitsByDate[key], v)
			}
		}
	}

	// 7. Assemble per-day response
	days := buildAttendanceHistoryDays(attendanceRows, visitsByDate, roomCutoff)

	resp := attendanceHistoryResponse{
		StudentID: strconv.FormatInt(student.ID, 10),
		Days:      days,
		Range:     attendanceHistoryRange{Start: start, End: end},
		Clamped:   clamped,
		Caps:      attendanceHistoryCaps{AttendanceDays: attendanceCap, RoomDetailDays: roomCap},
	}

	// 8. Fire-and-forget audit write
	rs.writeAttendanceHistoryAudit(r, student.ID, start, end, logger)

	common.Respond(w, r, http.StatusOK, resp, "Attendance history retrieved successfully")
}

// attendanceHistoryLogger returns a scoped logger, falling back to slog.Default.
func (rs *Resource) attendanceHistoryLogger() *slog.Logger {
	if rs.Logger != nil {
		return rs.Logger.With("handler", "attendance_history")
	}
	return slog.Default().With("handler", "attendance_history")
}

// attendanceHistoryScopeAllows returns true if the requesting staff member may
// view this student's attendance history according to the configured scope.
// Admins always pass.
func (rs *Resource) attendanceHistoryScopeAllows(r *http.Request, student *users.Student, scope string) bool {
	perms := jwt.PermissionsFromCtx(r.Context())
	if hasAdminPermissions(perms) {
		return true
	}
	if scope == configModel.AttendanceLogScopeAllStaff {
		return true
	}
	// group_supervisors_only (default)
	if student.GroupID == nil {
		return false
	}
	return isGroupSupervisor(r.Context(), *student.GroupID, rs.UserContextService)
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

// buildAttendanceHistoryDays groups attendance rows by date and merges matching
// visit timelines. Days older than roomCutoff have RoomDetailAvailable=false.
func buildAttendanceHistoryDays(rows []*active.Attendance, visitsByDate map[string][]*active.Visit, roomCutoff time.Time) []attendanceHistoryDay {
	days := make([]attendanceHistoryDay, 0, len(rows))
	for _, row := range rows {
		dateKey := timezone.DateOf(row.Date).Format("2006-01-02")

		attendance := &attendanceDayRecord{
			CheckInTime:  row.CheckInTime,
			CheckOutTime: row.CheckOutTime,
			CheckedInBy:  row.CheckedInBy,
			CheckedOutBy: row.CheckedOutBy,
			DeviceID:     row.DeviceID,
		}
		if row.CheckOutTime != nil {
			mins := int(row.CheckOutTime.Sub(row.CheckInTime).Minutes())
			attendance.DurationMinutes = &mins
		}

		day := attendanceHistoryDay{
			Date:       dateKey,
			Attendance: attendance,
			Visits:     []attendanceVisitEntry{},
		}

		// Room detail cap: only include visits if this day is on/after the cutoff
		if !timezone.DateOf(row.Date).Before(timezone.DateOf(roomCutoff)) {
			day.RoomDetailAvailable = true
			if vs, ok := visitsByDate[dateKey]; ok {
				for _, v := range vs {
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
					day.Visits = append(day.Visits, entry)
				}
			}
		}

		days = append(days, day)
	}
	return days
}

// writeAttendanceHistoryAudit records a view event to audit.data_access_log.
// Errors are logged but never block the response.
func (rs *Resource) writeAttendanceHistoryAudit(r *http.Request, studentID int64, start, end time.Time, logger *slog.Logger) {
	if rs.DataAccessLogRepo == nil {
		return
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

	if err := rs.DataAccessLogRepo.Create(r.Context(), entry); err != nil {
		logger.Warn("audit log write failed",
			slog.Int64("student_id", studentID),
			slog.String("resource_type", auditModels.ResourceTypeAttendanceHistory),
			slog.String("error", err.Error()),
		)
	}
}
