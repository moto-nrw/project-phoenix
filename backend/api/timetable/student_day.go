// Package timetable — per-student day and week read endpoints (WP-B11).
//
//	GET /api/timetable/student/{id}/day?date=YYYY-MM-DD
//	GET /api/timetable/student/{id}/week?from=YYYY-MM-DD&to=YYYY-MM-DD
//
// Both routes return the student's planned instances for the requested
// date(s), each enriched with the student's attendance row, plus the
// arrival/pickup slot for that day (schedule or exception, or "none").
// For active/completed instances the student was physically in but NOT
// enrolled, the response adds a separate "is_unplanned" entry.
package timetable

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// isoWeekday returns 1..7 (Mon..Sun) for the weekday of d.
func isoWeekday(d timezone.Date) int {
	return int((int(d.Weekday())+6)%7 + 1)
}

// dateKey formats a calendar date as YYYY-MM-DD for use as a map key.
func dateKey(d timezone.Date) string { return d.String() }

// getStudentDay handles GET /api/timetable/student/{id}/day.
func (rs *Resource) getStudentDay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	studentID, ok := parseStudentIDParam(w, r)
	if !ok {
		return
	}
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("date is required")))
		return
	}
	date, err := berlinDate(dateStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid date format, expected YYYY-MM-DD")))
		return
	}

	student, ok := rs.resolveStudentForRead(w, r, studentID)
	if !ok {
		return
	}

	// /day is /week with from == to: one round of batch queries covers it.
	days, err := rs.buildStudentDays(ctx, student.ID, date, date)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("build student day failed", err))
		return
	}
	if len(days) != 1 {
		common.RenderError(w, r, common.ErrorInternalServer(
			fmt.Errorf("buildStudentDays returned %d days, expected 1", len(days))))
		return
	}

	rs.getLogger().Info("timetable student day",
		slog.Int64("student_id", student.ID),
		slog.String("date", days[0].Date),
	)
	common.Respond(w, r, http.StatusOK, days[0], "Student day retrieved")
}

// getStudentWeek handles GET /api/timetable/student/{id}/week. The date
// range is inclusive on both ends; max 14 days.
func (rs *Resource) getStudentWeek(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	studentID, ok := parseStudentIDParam(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	fromStr := q.Get("from")
	toStr := q.Get("to")
	if fromStr == "" || toStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("from and to are required")))
		return
	}
	from, err := berlinDate(fromStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid from format, expected YYYY-MM-DD")))
		return
	}
	to, err := berlinDate(toStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid to format, expected YYYY-MM-DD")))
		return
	}
	if from.After(to) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("'from' must be before or equal to 'to'")))
		return
	}
	rangeDays := inclusiveDayCount(from, to)
	if rangeDays > scheduleModel.MaxTimetableReadRangeDays {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			fmt.Errorf("date range exceeds maximum of %d days", scheduleModel.MaxTimetableReadRangeDays)))
		return
	}

	student, ok := rs.resolveStudentForRead(w, r, studentID)
	if !ok {
		return
	}

	days, err := rs.buildStudentDays(ctx, student.ID, from, to)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("build student week failed", err))
		return
	}

	resp := StudentWeekResponse{
		StudentID: student.ID,
		From:      from.String(),
		To:        to.String(),
		Days:      days,
	}

	rs.getLogger().Info("timetable student week",
		slog.Int64("student_id", student.ID),
		slog.String("from", resp.From),
		slog.String("to", resp.To),
		slog.Int("days", len(days)),
	)
	common.Respond(w, r, http.StatusOK, resp, "Student week retrieved")
}

// parseStudentIDParam extracts {id} from the path. 400 on garbage or <=0.
func parseStudentIDParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid student ID")))
		return 0, false
	}
	return id, true
}

// resolveStudentForRead fetches the student and enforces the read-access
// rules. Cross-tenant ("student doesn't exist in caller's tenant") is 404;
// same-tenant but no access is 403. The 404-vs-403 split matters: returning
// 403 for a tenant-B student would leak the student's existence to tenant A.
func (rs *Resource) resolveStudentForRead(w http.ResponseWriter, r *http.Request, studentID int64) (*usersModel.Student, bool) {
	ctx := r.Context()

	if rs.TimetableData == nil || rs.PersonService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("student repo not wired")))
		return nil, false
	}

	student, err := rs.PersonService.GetStudentByID(ctx, studentID)
	if err != nil {
		if base.IsNoRows(err) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("student not found")))
			return nil, false
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("load student failed", err))
		return nil, false
	}
	if student == nil || student.ID == 0 {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("student not found")))
		return nil, false
	}

	// A graduated (alumnus) student is soft-deleted: invisible to every staff
	// read. GetStudentByID is unfiltered and CanReadStudent only decides group /
	// permission scope, so without this gate an administrator or group
	// supervisor with a bookmarked student ID could keep reading a departed
	// child's day/week timetable. Same 404 as the students resource applies
	// (api/students.parseAndGetStudent) (#405 review).
	if student.Status == usersModel.StudentStatusAlumnus {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("student not found")))
		return nil, false
	}

	// A child whose care has ended leaves the operational timetable the same
	// way (#2487). Their past days stay in the history exports; the live
	// day/week view is for children who still attend.
	if student.CareEndedOn(rs.todayDate()) {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("student not found")))
		return nil, false
	}

	if !authorize.CanReadStudent(
		ctx,
		jwt.PermissionsFromCtx(ctx),
		student,
		rs.UserContextService,
	) {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("forbidden")))
		return nil, false
	}

	return student, true
}

// buildStudentDays assembles one StudentDayResponse per day in the inclusive
// [from, to] range. A single batch round trip per category fills the preload
// (owned by the TimetableData facade); from there the per-day assembly is a
// pure in-memory slice.
//
// Query count (constant in range length): 1 enrolled + 1 all-instances +
// 1 visits (if any active groups) + 2 schedules + 2 exceptions = max 7.
func (rs *Resource) buildStudentDays(ctx context.Context, studentID int64, from, to timezone.Date) ([]StudentDayResponse, error) {
	if rs.TimetableData == nil {
		return nil, errors.New("timetable data service not wired")
	}
	pre, err := rs.TimetableData.PreloadStudentWeek(ctx, studentID, from, to)
	if err != nil {
		return nil, err
	}

	dayCount := inclusiveDayCount(from, to)
	days := make([]StudentDayResponse, 0, dayCount)
	for d := from; !d.After(to); d = d.AddDays(1) {
		day := buildStudentDayFromPreload(pre, studentID, d)
		days = append(days, day)
	}
	return days, nil
}

// buildStudentDayFromPreload assembles a single day's response from the
// already-preloaded data. No DB calls.
func buildStudentDayFromPreload(pre *scheduleSvc.StudentWeekPreload, studentID int64, date timezone.Date) StudentDayResponse {
	k := dateKey(date)

	enrolledRows := pre.EnrolledByDate[k]
	dayInstances := pre.InstancesByDate[k]

	// Build the base instance list from enrolled rows.
	enrolledInstanceIDs := make(map[int64]bool, len(enrolledRows))
	instances := make([]InstanceDayResponse, 0, len(enrolledRows)+len(dayInstances))
	for _, row := range enrolledRows {
		instances = append(instances, mapEnrolledInstance(row))
		enrolledInstanceIDs[row.Instance.ID] = true
	}

	instances = appendUnplannedInstances(instances, pre, dayInstances, enrolledInstanceIDs)

	sort.SliceStable(instances, func(i, j int) bool {
		return instances[i].StartTime < instances[j].StartTime
	})

	return StudentDayResponse{
		StudentID: studentID,
		Date:      date.String(),
		Weekday:   isoWeekday(date),
		Arrival:   resolveArrivalSlotFromPreload(pre, date),
		Instances: instances,
		Pickup:    resolvePickupSlotFromPreload(pre, date),
	}
}

// appendUnplannedInstances adds is_unplanned entries: a visit on one of today's
// active/completed instances with no instance_students row. active_group_id is
// 1:1 with activity_instance (each instance creates its own group on start), so
// the map lookup already scopes visits to this date.
func appendUnplannedInstances(
	instances []InstanceDayResponse,
	pre *scheduleSvc.StudentWeekPreload,
	dayInstances []*scheduleModel.ActivityInstance,
	enrolledInstanceIDs map[int64]bool,
) []InstanceDayResponse {
	for _, inst := range dayInstances {
		if inst.ActiveGroupID == nil || enrolledInstanceIDs[inst.ID] {
			continue
		}
		if inst.Status != scheduleModel.InstanceStatusActive &&
			inst.Status != scheduleModel.InstanceStatusCompleted {
			continue
		}
		if v := firstNonNilVisit(pre.VisitsByActiveGroup[*inst.ActiveGroupID]); v != nil {
			instances = append(instances, mapUnplannedInstance(inst, v))
		}
	}
	return instances
}

// firstNonNilVisit returns the first non-nil visit in the slice, or nil. Dedup
// across instances is handled upstream by the 1:1 active_group_id mapping.
func firstNonNilVisit(visits []*activeModel.Visit) *activeModel.Visit {
	for _, candidate := range visits {
		if candidate != nil {
			return candidate
		}
	}
	return nil
}

// resolveArrivalSlotFromPreload applies the shared exception-over-schedule rule
// (ResolveSlotSource) against already-loaded data. An exception on the date
// wins even when its time is nil (absence signal).
func resolveArrivalSlotFromPreload(pre *scheduleSvc.StudentWeekPreload, date timezone.Date) SlotResponse {
	exc, hasExc := pre.ArrivalExcByDate[dateKey(date)]
	hasExc = hasExc && exc != nil
	wd := isoWeekday(date)
	sched, hasSched := pre.ArrivalSchedByDate[dateKey(date)]
	hasSched = hasSched && sched != nil

	switch scheduleSvc.ResolveSlotSource(hasExc, hasSched, wd) {
	case SlotSourceException:
		return mapArrivalExceptionSlot(exc)
	case SlotSourceSchedule:
		return mapArrivalScheduleSlot(sched)
	default:
		return SlotResponse{Source: SlotSourceNone}
	}
}

// resolvePickupSlotFromPreload mirrors resolveArrivalSlotFromPreload.
func resolvePickupSlotFromPreload(pre *scheduleSvc.StudentWeekPreload, date timezone.Date) SlotResponse {
	exc, hasExc := pre.PickupExcByDate[dateKey(date)]
	hasExc = hasExc && exc != nil
	wd := isoWeekday(date)
	sched, hasSched := pre.PickupSchedByDate[dateKey(date)]
	hasSched = hasSched && sched != nil

	switch scheduleSvc.ResolveSlotSource(hasExc, hasSched, wd) {
	case SlotSourceException:
		return mapPickupExceptionSlot(exc)
	case SlotSourceSchedule:
		return mapPickupScheduleSlot(sched)
	default:
		return SlotResponse{Source: SlotSourceNone}
	}
}
