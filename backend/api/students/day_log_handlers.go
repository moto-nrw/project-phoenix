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
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// Group day log ("Tagesauswertung", issue #1456): for one calendar day and one
// or all permitted groups, every child with a single day verdict — present,
// sick, class trip, excused, absent, or not scheduled. Read-only; shares the
// GDPR envelope of the per-student attendance history (feature gate, scope,
// visibility cap, audit-or-refuse).

const (
	dayLogStatusPresent      = "present"
	dayLogStatusSick         = "sick"
	dayLogStatusClassTrip    = "class_trip"
	dayLogStatusExcused      = "excused"
	dayLogStatusAbsent       = "absent"
	dayLogStatusNotScheduled = "not_scheduled"

	// dayLogSourceCancelledCareDay marks an excused verdict derived from a
	// same-day "Kommt heute nicht" care-plan exception instead of a status day.
	dayLogSourceCancelledCareDay = "care_plan_cancelled"
)

type dayLogResponse struct {
	Date     string         `json:"date"`
	Groups   []dayLogGroup  `json:"groups"`
	Counters dayLogCounters `json:"counters"`
	Caps     dayLogCaps     `json:"caps"`
}

type dayLogCaps struct {
	AttendanceDays int `json:"attendance_days"`
}

type dayLogGroup struct {
	GroupID  string          `json:"group_id"`
	Name     string          `json:"name"`
	Students []dayLogStudent `json:"students"`
	Counters dayLogCounters  `json:"counters"`
}

type dayLogStudent struct {
	StudentID   string `json:"student_id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	SchoolClass string `json:"school_class"`
	Status      string `json:"status"`
	Label       string `json:"label"`
	// CheckInTime/CheckOutTime carry the day's first arrival and last
	// departure; a present child without CheckOutTime is still checked in.
	CheckInTime  *time.Time `json:"check_in_time,omitempty"`
	CheckOutTime *time.Time `json:"check_out_time,omitempty"`
	// ReportedAt/Source describe the sign-off behind an absence verdict.
	ReportedAt *time.Time `json:"reported_at,omitempty"`
	Source     string     `json:"source,omitempty"`
	// Hint flags a present child who also has a sign-off on file.
	Hint string `json:"hint,omitempty"`
}

type dayLogCounters struct {
	Present      int `json:"present"`
	Sick         int `json:"sick"`
	ClassTrip    int `json:"class_trip"`
	Excused      int `json:"excused"`
	Absent       int `json:"absent"`
	NotScheduled int `json:"not_scheduled"`
	Total        int `json:"total"`
}

func (c *dayLogCounters) add(status string) {
	c.Total++
	switch status {
	case dayLogStatusPresent:
		c.Present++
	case dayLogStatusSick:
		c.Sick++
	case dayLogStatusClassTrip:
		c.ClassTrip++
	case dayLogStatusExcused:
		c.Excused++
	case dayLogStatusNotScheduled:
		c.NotScheduled++
	default:
		c.Absent++
	}
}

func (c *dayLogCounters) merge(other dayLogCounters) {
	c.Present += other.Present
	c.Sick += other.Sick
	c.ClassTrip += other.ClassTrip
	c.Excused += other.Excused
	c.Absent += other.Absent
	c.NotScheduled += other.NotScheduled
	c.Total += other.Total
}

// getStudentsDayLog handles GET /students/day-log?date=YYYY-MM-DD&group_id=N.
// Without group_id it returns every group the caller may see.
func (rs *Resource) getStudentsDayLog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := rs.dayLogLogger()

	if !configService.ResolveBoolOrDefault(ctx, rs.SettingsService, configModel.KeyAttendanceLogEnabled, false, logger) {
		renderError(w, r, common.ErrorForbidden(errors.New("feature_disabled")))
		return
	}

	attendanceCap := configService.ResolveIntOrDefault(ctx, rs.SettingsService, configModel.KeyAttendanceVisibleDays, 30, logger)
	date, err := parseDayLogDate(r, attendanceCap)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	groups, err := rs.resolveDayLogGroups(r, logger)
	if err != nil {
		renderError(w, r, common.ErrorForbidden(err))
		return
	}

	data, err := rs.loadDayLogData(ctx, groups, date)
	if err != nil {
		logger.Error("day log source query failed",
			slog.String("date", date.String()),
			slog.String("error", err.Error()),
		)
		renderError(w, r, common.ErrorInternalServerWrap("failed to load day log", err))
		return
	}

	resp := buildDayLogResponse(date, groups, data)
	resp.Caps = dayLogCaps{AttendanceDays: attendanceCap}

	if err := rs.writeDayLogAudit(r, date, groups, logger); err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to record audit trail", err))
		return
	}

	common.Respond(w, r, http.StatusOK, resp, "Day log retrieved successfully")
}

func (rs *Resource) dayLogLogger() *slog.Logger {
	if rs.Logger != nil {
		return rs.Logger.With("handler", "day_log")
	}
	return slog.Default().With("handler", "day_log")
}

// parseDayLogDate reads the date query param (default: today) and enforces the
// retrospective visibility cap. Future dates are rejected — this is a log, not
// a planner.
func parseDayLogDate(r *http.Request, attendanceCap int) (timezone.Date, error) {
	today := timezone.TodayDate()
	date := today
	if raw := strings.TrimSpace(r.URL.Query().Get("date")); raw != "" {
		parsed, err := timezone.ParseDate(raw)
		if err != nil {
			return timezone.Date{}, errors.New("invalid date format, expected YYYY-MM-DD")
		}
		date = parsed
	}
	if date.After(today) {
		return timezone.Date{}, errors.New("date must not be in the future")
	}
	if attendanceCap > 0 && date.Before(today.AddDays(-(attendanceCap - 1))) {
		return timezone.Date{}, errors.New("date is outside the visible attendance window")
	}
	return date, nil
}

// resolveDayLogGroups returns the groups the caller may evaluate, optionally
// narrowed to the requested group_id. Admins and (with scope all_staff) every
// staff member see all groups; otherwise only supervised groups are visible.
func (rs *Resource) resolveDayLogGroups(r *http.Request, logger *slog.Logger) ([]*educationModel.Group, error) {
	ctx := r.Context()

	var groups []*educationModel.Group
	var err error
	if rs.dayLogSeesAllGroups(r, logger) {
		groups, err = rs.EducationService.ListGroups(ctx, nil)
	} else {
		groups, err = rs.UserContextService.GetMyGroups(ctx)
	}
	if err != nil {
		return nil, errors.New("failed to resolve permitted groups")
	}

	groups, err = filterDayLogGroups(r, groups)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups, nil
}

func (rs *Resource) dayLogSeesAllGroups(r *http.Request, logger *slog.Logger) bool {
	if authorize.HasAdminWildcard(jwt.PermissionsFromCtx(r.Context())) {
		return true
	}
	scope := configService.ResolveStringOrDefault(r.Context(), rs.SettingsService, configModel.KeyAttendanceLogScope, configModel.AttendanceLogScopeGroupSupervisorsOnly, logger)
	return scope == configModel.AttendanceLogScopeAllStaff
}

func filterDayLogGroups(r *http.Request, groups []*educationModel.Group) ([]*educationModel.Group, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("group_id"))
	if raw == "" {
		if len(groups) == 0 {
			return nil, errors.New("no_permitted_groups")
		}
		return groups, nil
	}
	groupID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, errors.New("invalid group_id")
	}
	for _, group := range groups {
		if group != nil && group.ID == groupID {
			return []*educationModel.Group{group}, nil
		}
	}
	return nil, errors.New("not_group_supervisor")
}

// dayLogData carries the bulk-loaded facts of one day.
type dayLogData struct {
	studentsByGroup     map[int64][]*usersModel.Student
	persons             map[int64]*usersModel.Person
	attendanceByStudent map[int64][]*active.Attendance
	statusByStudent     map[int64][]*active.StudentStatusDay
	careDays            map[int64]scheduleService.CareDayStatus
}

func (rs *Resource) loadDayLogData(ctx context.Context, groups []*educationModel.Group, date timezone.Date) (*dayLogData, error) {
	groupIDs := make([]int64, 0, len(groups))
	for _, group := range groups {
		groupIDs = append(groupIDs, group.ID)
	}

	students, err := rs.PersonService.GetStudentsByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}

	data := &dayLogData{
		studentsByGroup:     make(map[int64][]*usersModel.Student, len(groups)),
		attendanceByStudent: map[int64][]*active.Attendance{},
		statusByStudent:     map[int64][]*active.StudentStatusDay{},
		careDays:            map[int64]scheduleService.CareDayStatus{},
	}
	studentIDs, personIDs := indexDayLogStudents(data, students)

	data.persons, err = rs.PersonService.GetByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	if err := rs.loadDayLogAttendance(ctx, data, date); err != nil {
		return nil, err
	}
	if err := rs.loadDayLogSignOffs(ctx, data, studentIDs, date); err != nil {
		return nil, err
	}
	return data, nil
}

func indexDayLogStudents(data *dayLogData, students []*usersModel.Student) (studentIDs, personIDs []int64) {
	studentIDs = make([]int64, 0, len(students))
	personIDs = make([]int64, 0, len(students))
	for _, student := range students {
		if student == nil || student.GroupID == nil {
			continue
		}
		data.studentsByGroup[*student.GroupID] = append(data.studentsByGroup[*student.GroupID], student)
		studentIDs = append(studentIDs, student.ID)
		personIDs = append(personIDs, student.PersonID)
	}
	return studentIDs, personIDs
}

func (rs *Resource) loadDayLogAttendance(ctx context.Context, data *dayLogData, date timezone.Date) error {
	attendance, err := rs.StudentHistoryService.GetAttendanceForDate(ctx, date)
	if err != nil {
		return err
	}
	rostered := make(map[int64]bool, len(data.persons))
	for _, students := range data.studentsByGroup {
		for _, student := range students {
			rostered[student.ID] = true
		}
	}
	for _, row := range attendance {
		if rostered[row.StudentID] {
			data.attendanceByStudent[row.StudentID] = append(data.attendanceByStudent[row.StudentID], row)
		}
	}
	return nil
}

func (rs *Resource) loadDayLogSignOffs(ctx context.Context, data *dayLogData, studentIDs []int64, date timezone.Date) error {
	if rs.StudentStatusDayService != nil {
		statuses, err := rs.StudentStatusDayService.GetSignedOffByStudentIDsAndDate(ctx, studentIDs, date)
		if err != nil {
			return err
		}
		for _, row := range statuses {
			data.statusByStudent[row.StudentID] = append(data.statusByStudent[row.StudentID], row)
		}
	}

	// Optional: without the care-day derivation every unexplained child stays
	// plain "absent" (bare test Resources run without it).
	if rs.CareDayService != nil {
		careDays, err := rs.CareDayService.ResolveForDate(ctx, studentIDs, date)
		if err != nil {
			return err
		}
		data.careDays = careDays
	}
	return nil
}

func buildDayLogResponse(date timezone.Date, groups []*educationModel.Group, data *dayLogData) dayLogResponse {
	resp := dayLogResponse{Date: date.String(), Groups: make([]dayLogGroup, 0, len(groups))}
	for _, group := range groups {
		entry := dayLogGroup{
			GroupID:  strconv.FormatInt(group.ID, 10),
			Name:     group.Name,
			Students: make([]dayLogStudent, 0, len(data.studentsByGroup[group.ID])),
		}
		for _, student := range data.studentsByGroup[group.ID] {
			row := buildDayLogStudent(student, data)
			entry.Counters.add(row.Status)
			entry.Students = append(entry.Students, row)
		}
		sortDayLogStudents(entry.Students)
		resp.Counters.merge(entry.Counters)
		resp.Groups = append(resp.Groups, entry)
	}
	return resp
}

func sortDayLogStudents(students []dayLogStudent) {
	sort.SliceStable(students, func(i, j int) bool {
		if students[i].LastName != students[j].LastName {
			return students[i].LastName < students[j].LastName
		}
		return students[i].FirstName < students[j].FirstName
	})
}

func buildDayLogStudent(student *usersModel.Student, data *dayLogData) dayLogStudent {
	row := dayLogStudent{
		StudentID:   strconv.FormatInt(student.ID, 10),
		SchoolClass: student.SchoolClass,
	}
	if person := data.persons[student.PersonID]; person != nil {
		row.FirstName = person.FirstName
		row.LastName = person.LastName
	}
	classifyDayLogStudent(&row,
		data.attendanceByStudent[student.ID],
		data.statusByStudent[student.ID],
		data.careDays[student.ID],
	)
	row.Label = dayLogStatusLabel(row.Status, row.Source)
	return row
}

// classifyDayLogStudent derives the single day verdict: an attendance row
// wins (present), then the status-day precedence (sick > class trip >
// excused), then a cancelled care day (reported absence), then a non-booked
// day, and only the unexplained rest is absent.
func classifyDayLogStudent(row *dayLogStudent, attendance []*active.Attendance, statuses []*active.StudentStatusDay, careDay scheduleService.CareDayStatus) {
	eff := activeService.ResolveEffectiveStatus(statuses)

	if len(attendance) > 0 {
		row.Status = dayLogStatusPresent
		checkIn, checkOut := mergeDayLogAttendance(attendance)
		row.CheckInTime = &checkIn
		row.CheckOutTime = checkOut
		row.Hint = dayLogPresentHint(eff)
		return
	}

	if applyDayLogSignOff(row, eff, statuses) {
		return
	}

	switch careDay {
	case scheduleService.CareDayCancelled:
		row.Status = dayLogStatusExcused
		row.Source = dayLogSourceCancelledCareDay
	case scheduleService.CareDayNotScheduled:
		row.Status = dayLogStatusNotScheduled
	default:
		row.Status = dayLogStatusAbsent
	}
}

func applyDayLogSignOff(row *dayLogStudent, eff activeService.EffectiveStatus, statuses []*active.StudentStatusDay) bool {
	var status string
	var since *time.Time
	switch {
	case eff.Sick:
		status, since = active.StudentStatusDaySick, eff.SickSince
		row.Status = dayLogStatusSick
	case eff.ClassTrip:
		status, since = active.StudentStatusDayClassTrip, eff.ClassTripSince
		row.Status = dayLogStatusClassTrip
	case eff.Excused:
		status, since = active.StudentStatusDayExcused, eff.ExcusedSince
		row.Status = dayLogStatusExcused
	default:
		return false
	}
	row.ReportedAt = since
	if winner := latestDayLogRowOfStatus(statuses, status); winner != nil {
		row.Source = winner.Source
	}
	return true
}

func latestDayLogRowOfStatus(statuses []*active.StudentStatusDay, status string) *active.StudentStatusDay {
	var winner *active.StudentStatusDay
	for _, row := range statuses {
		if row.Status != status {
			continue
		}
		if winner == nil || row.ReportedAt.After(winner.ReportedAt) {
			winner = row
		}
	}
	return winner
}

// mergeDayLogAttendance folds a day's sessions into first arrival and last
// departure; an open session (nil check-out) keeps the departure open.
func mergeDayLogAttendance(rows []*active.Attendance) (time.Time, *time.Time) {
	checkIn := rows[0].CheckInTime
	checkOut := rows[0].CheckOutTime
	stillPresent := checkOut == nil
	for _, row := range rows[1:] {
		if row.CheckInTime.Before(checkIn) {
			checkIn = row.CheckInTime
		}
		if row.CheckOutTime == nil {
			stillPresent = true
			continue
		}
		if checkOut == nil || row.CheckOutTime.After(*checkOut) {
			checkOut = row.CheckOutTime
		}
	}
	if stillPresent {
		return checkIn, nil
	}
	return checkIn, checkOut
}

func dayLogPresentHint(eff activeService.EffectiveStatus) string {
	switch {
	case eff.Sick:
		return "Krankmeldung liegt vor"
	case eff.ClassTrip:
		return "Klassenfahrt gemeldet"
	case eff.Excused:
		return "Abmeldung liegt vor"
	default:
		return ""
	}
}

func dayLogStatusLabel(status, source string) string {
	switch status {
	case dayLogStatusPresent:
		return "Anwesend"
	case dayLogStatusSick:
		return "Krank"
	case dayLogStatusClassTrip:
		return "Klassenfahrt"
	case dayLogStatusExcused:
		if source == dayLogSourceCancelledCareDay {
			return "Abgemeldet"
		}
		return "Entschuldigt"
	case dayLogStatusNotScheduled:
		return "Nicht eingeplant"
	default:
		return "Abwesend"
	}
}

// writeDayLogAudit records the group-scoped access in audit.data_access_log.
// Like the per-student history: no audit record, no data.
func (rs *Resource) writeDayLogAudit(r *http.Request, date timezone.Date, groups []*educationModel.Group, logger *slog.Logger) error {
	if rs.StudentHistoryService == nil {
		logger.Error("audit log repo not configured, refusing to serve day log")
		return errors.New("audit log repository not configured")
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	actorRole := strings.Join(claims.Roles, ",")
	if actorRole == "" {
		actorRole = "unknown"
	}
	groupIDs := make([]int64, 0, len(groups))
	for _, group := range groups {
		groupIDs = append(groupIDs, group.ID)
	}

	entry := &auditModels.DataAccessLog{
		ActorAccountID: int64(claims.ID),
		ActorRole:      actorRole,
		ResourceType:   auditModels.ResourceTypeAttendanceDayLog,
		RangeStart:     date.BerlinMidnight(),
		RangeEnd:       date.EndOfDay(),
		AccessedAt:     time.Now(),
	}
	entry.SetMetadata("group_ids", groupIDs)
	entry.SetMetadata("date", date.String())

	if err := rs.StudentHistoryService.RecordDataAccess(r.Context(), entry); err != nil {
		logger.Error("audit log write failed, refusing to serve day log",
			slog.String("resource_type", auditModels.ResourceTypeAttendanceDayLog),
			slog.String("error", err.Error()),
		)
		return err
	}
	return nil
}
