package students

import (
	"context"
	"errors"
	"fmt"
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
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
)

// Group day log ("Tagesauswertung", issue #1456): for one calendar day and one
// or all permitted groups, every child with a single day verdict — present,
// sick, class trip, excused, absent, or not scheduled. Read-only; shares the
// GDPR envelope of the per-student attendance history (feature gate, scope,
// audit-or-refuse).

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

	date, err := parseDayLogDate(r)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	groups, err := rs.resolveDayLogGroups(r, date, logger)
	if err != nil {
		renderDayLogGroupError(w, r, err, logger)
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

// parseDayLogDate reads the date query param (default: today). The student
// record stores only its current group assignment, so a retrospective group
// roster cannot be reconstructed safely after a transfer. Until group
// assignments have a dated source of truth, the day log is deliberately a
// live-day view rather than attributing historical attendance to a wrong group.
func parseDayLogDate(r *http.Request) (timezone.Date, error) {
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
	if date.Before(today) {
		return timezone.Date{}, errors.New("historical day logs require dated group assignments")
	}
	return date, nil
}

// errDayLogGroupsUnavailable marks a dependency failure (group service /
// database) while resolving the caller's groups. It must surface as a 5xx —
// mapping it to 403 would disguise a transient outage as a privacy restriction.
var errDayLogGroupsUnavailable = errors.New("failed to resolve permitted groups")
var errInvalidDayLogGroupID = errors.New("invalid group_id")

// resolveDayLogGroups returns the groups the caller may evaluate, optionally
// narrowed to the requested group_id. Admins see all groups. Every other caller
// must have a linked staff record first: the all_staff scope widens what STAFF
// may read, so an account holding users:read without a person/staff row must
// not inherit it. Only then does the scope decide between all groups and the
// groups supervised on the requested date.
func (rs *Resource) resolveDayLogGroups(r *http.Request, date timezone.Date, logger *slog.Logger) ([]*educationModel.Group, error) {
	ctx := r.Context()

	groups, err := rs.permittedDayLogGroups(ctx, date, logger)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errDayLogGroupsUnavailable, err)
	}

	groups, err = filterDayLogGroups(r, groups)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups, nil
}

// permittedDayLogGroups yields the unfiltered permitted set, or an empty set
// when the caller is not staff at all. Errors here are dependency failures.
func (rs *Resource) permittedDayLogGroups(ctx context.Context, date timezone.Date, logger *slog.Logger) ([]*educationModel.Group, error) {
	if authorize.HasAdminWildcard(jwt.PermissionsFromCtx(ctx)) {
		return rs.EducationService.ListGroups(ctx, nil)
	}

	staff, err := rs.dayLogCurrentStaff(ctx)
	if err != nil {
		return nil, err
	}
	if staff == nil {
		// No linked staff record: supervises nothing, and no scope may widen
		// that. Renders as a 403 via filterDayLogGroups, mirroring the
		// room-history endpoint.
		return nil, nil
	}

	scope := configService.ResolveStringOrDefault(ctx, rs.SettingsService, configModel.KeyAttendanceLogScope, configModel.AttendanceLogScopeGroupSupervisorsOnly, logger)
	if scope == configModel.AttendanceLogScopeAllStaff {
		return rs.EducationService.ListGroups(ctx, nil)
	}
	return rs.dayLogGroupsForDate(ctx, staff.ID, date)
}

// dayLogCurrentStaff returns the caller's staff record, or (nil, nil) when the
// account has no person/staff link. That is an expected state (e.g.
// admin-created accounts), not a dependency failure.
func (rs *Resource) dayLogCurrentStaff(ctx context.Context) (*usersModel.Staff, error) {
	staff, err := rs.UserContextService.GetCurrentStaff(ctx)
	if err != nil {
		if errors.Is(err, userContextService.ErrUserNotLinkedToStaff) ||
			errors.Is(err, userContextService.ErrUserNotLinkedToPerson) {
			return nil, nil
		}
		return nil, err
	}
	return staff, nil
}

// dayLogGroupsForDate resolves permanent teacher groups plus only those
// substitutions that cover the requested log date. GetMyGroups intentionally
// uses today's substitutions for live views and must not authorize history.
func (rs *Resource) dayLogGroupsForDate(ctx context.Context, staffID int64, date timezone.Date) ([]*educationModel.Group, error) {
	groupsByID := make(map[int64]*educationModel.Group)
	teacher, err := rs.UserContextService.GetCurrentTeacher(ctx)
	if err == nil {
		groups, groupsErr := rs.EducationService.GetTeacherGroups(ctx, teacher.ID)
		if groupsErr != nil {
			return nil, groupsErr
		}
		addDayLogGroups(groupsByID, groups)
	} else if !errors.Is(err, userContextService.ErrUserNotLinkedToTeacher) {
		return nil, err
	}
	substitutions, err := rs.EducationService.GetActiveSubstitutions(ctx, date)
	if err != nil {
		return nil, err
	}
	addDayLogSubstitutionGroups(groupsByID, substitutions, staffID)
	groups := make([]*educationModel.Group, 0, len(groupsByID))
	for _, group := range groupsByID {
		groups = append(groups, group)
	}
	return groups, nil
}

func addDayLogGroups(groupsByID map[int64]*educationModel.Group, groups []*educationModel.Group) {
	for _, group := range groups {
		if group != nil {
			groupsByID[group.ID] = group
		}
	}
}

func addDayLogSubstitutionGroups(groupsByID map[int64]*educationModel.Group, substitutions []*educationModel.GroupSubstitution, staffID int64) {
	for _, substitution := range substitutions {
		if substitution != nil && substitution.SubstituteStaffID == staffID && substitution.Group != nil {
			groupsByID[substitution.Group.ID] = substitution.Group
		}
	}
}

// renderDayLogGroupError separates the two failure classes of
// resolveDayLogGroups: dependency failures become 500s, everything else is a
// genuine authorization denial and stays 403.
func renderDayLogGroupError(w http.ResponseWriter, r *http.Request, err error, logger *slog.Logger) {
	if errors.Is(err, errDayLogGroupsUnavailable) {
		logger.Error("day log group resolution failed",
			slog.String("error", err.Error()),
		)
		renderError(w, r, common.ErrorInternalServerWrap("failed to resolve permitted groups", err))
		return
	}
	if errors.Is(err, errInvalidDayLogGroupID) {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	renderError(w, r, common.ErrorForbidden(err))
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
		return nil, fmt.Errorf("%w: %v", errInvalidDayLogGroupID, err)
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
	arrivalTimes        map[int64]*scheduleService.EffectiveArrivalTime
	now                 time.Time
}

func (rs *Resource) loadDayLogData(ctx context.Context, groups []*educationModel.Group, date timezone.Date) (*dayLogData, error) {
	groupIDs := make([]int64, 0, len(groups))
	for _, group := range groups {
		groupIDs = append(groupIDs, group.ID)
	}

	students, err := rs.PersonService.GetEligibleStudentsByGroupIDsOnDate(ctx, groupIDs, date)
	if err != nil {
		return nil, err
	}
	// Student.GroupID is the current assignment. The data model does not retain
	// dated group assignments, so only enrollment eligibility can be evaluated
	// retrospectively here.

	now := rs.dayLogNow()
	data := &dayLogData{
		studentsByGroup:     make(map[int64][]*usersModel.Student, len(groups)),
		attendanceByStudent: map[int64][]*active.Attendance{},
		statusByStudent:     map[int64][]*active.StudentStatusDay{},
		careDays:            map[int64]scheduleService.CareDayStatus{},
		arrivalTimes:        map[int64]*scheduleService.EffectiveArrivalTime{},
		now:                 now,
	}
	studentIDs, personIDs := indexDayLogStudents(data, students)

	data.persons, err = rs.PersonService.GetByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	if err := rs.loadDayLogAttendance(ctx, data, date); err != nil {
		return nil, err
	}
	if err := rs.loadDayLogSignOffs(ctx, data, studentIDs, date, now); err != nil {
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
	studentIDs := make([]int64, 0, len(data.persons))
	for _, students := range data.studentsByGroup {
		for _, student := range students {
			studentIDs = append(studentIDs, student.ID)
		}
	}
	attendance, err := rs.StudentHistoryService.GetAttendanceForDateByStudentIDs(ctx, date, studentIDs)
	if err != nil {
		return err
	}
	for _, row := range attendance {
		data.attendanceByStudent[row.StudentID] = append(data.attendanceByStudent[row.StudentID], row)
	}
	return nil
}

func (rs *Resource) loadDayLogSignOffs(ctx context.Context, data *dayLogData, studentIDs []int64, date timezone.Date, now time.Time) error {
	if rs.StudentStatusDayService != nil {
		statuses, err := rs.StudentStatusDayService.GetSignedOffByStudentIDsAndDate(ctx, studentIDs, date)
		if err != nil {
			return err
		}
		for _, row := range statuses {
			data.statusByStudent[row.StudentID] = append(data.statusByStudent[row.StudentID], row)
		}
	}

	// Care plans are recurring and have no effective-date history. Derive them
	// only for today's live log; a later plan edit must not relabel a completed
	// day. Historical verdicts therefore use only persisted attendance and
	// status-day facts.
	if date == timezone.DateFromTime(now) && rs.CareDayService != nil {
		careDays, err := rs.CareDayService.ResolveForDate(ctx, studentIDs, date)
		if err != nil {
			return err
		}
		data.careDays = careDays
	}
	if date == timezone.DateFromTime(now) && rs.ArrivalScheduleService != nil {
		arrivalTimes, err := rs.ArrivalScheduleService.GetBulkEffectiveArrivalTimesForDate(ctx, studentIDs, date)
		if err != nil {
			return err
		}
		data.arrivalTimes = arrivalTimes
	}
	return nil
}

func (rs *Resource) dayLogNow() time.Time {
	if rs.Now != nil {
		return rs.Now()
	}
	return time.Now()
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
			if dayLogArrivalIsStillPending(row, data.careDays[student.ID], data.arrivalTimes[student.ID], date, data.now) {
				continue
			}
			entry.Counters.add(row.Status)
			entry.Students = append(entry.Students, row)
		}
		sortDayLogStudents(entry.Students)
		resp.Counters.merge(entry.Counters)
		resp.Groups = append(resp.Groups, entry)
	}
	return resp
}

// dayLogArrivalIsStillPending keeps a scheduled child out of today's live
// absence verdict until their effective planned arrival is due. Retrospective
// days always have a complete day and therefore never use this exception.
func dayLogArrivalIsStillPending(row dayLogStudent, careDay scheduleService.CareDayStatus, arrival *scheduleService.EffectiveArrivalTime, date timezone.Date, now time.Time) bool {
	if row.Status != dayLogStatusAbsent ||
		careDay != scheduleService.CareDayScheduled ||
		arrival == nil ||
		arrival.ArrivalTime == nil ||
		date != timezone.DateFromTime(now) {
		return false
	}
	return timezone.WallClock(now).Before(timezone.WallClock(*arrival.ArrivalTime))
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
