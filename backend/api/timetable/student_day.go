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
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
)

// maxWeekRangeDays caps /week at 14 days inclusive. Longer spans are
// rejected at 400 — the frontend has no use case for a longer horizon today.
const maxWeekRangeDays = 14

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
	if rangeDays > maxWeekRangeDays {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			fmt.Errorf("date range exceeds maximum of %d days", maxWeekRangeDays)))
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

	if rs.timetableData == nil || rs.personService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("student repo not wired")))
		return nil, false
	}

	student, err := rs.personService.GetStudentByID(ctx, studentID)
	if err != nil {
		if isNotFoundDBError(err) {
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

	if !authorize.CanReadStudent(
		ctx,
		jwt.PermissionsFromCtx(ctx),
		student,
		rs.userContextService,
		rs.settingsService,
		rs.getLogger(),
	) {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("forbidden")))
		return nil, false
	}

	return student, true
}

// weekPreload is the result of the range-wide batch queries used to assemble
// the response for one or more days. All lookups are keyed by
// Berlin-local YYYY-MM-DD (dateKey) except activeGroupInst which keys by
// active_group_id (not date-specific).
type weekPreload struct {
	enrolledByDate       map[string][]*scheduleModel.ScheduledInstanceRow
	instancesByDate      map[string][]*scheduleModel.ActivityInstance
	visitsByActiveGroup  map[int64][]*activeModel.Visit
	arrivalSchedByWeekly map[int]*scheduleModel.StudentArrivalSchedule
	arrivalExcByDate     map[string]*scheduleModel.StudentArrivalException
	pickupSchedByWeekly  map[int]*scheduleModel.StudentPickupSchedule
	pickupExcByDate      map[string]*scheduleModel.StudentPickupException
}

// preloadWeek issues one DB query per category for the full [from, to] range
// and returns data indexed by date. Replaces the previous N+1 pattern where
// each day re-queried the same tables.
func (rs *Resource) preloadWeek(ctx context.Context, studentID int64, from, to timezone.Date) (*weekPreload, error) {
	out := &weekPreload{
		enrolledByDate:       map[string][]*scheduleModel.ScheduledInstanceRow{},
		instancesByDate:      map[string][]*scheduleModel.ActivityInstance{},
		visitsByActiveGroup:  map[int64][]*activeModel.Visit{},
		arrivalSchedByWeekly: map[int]*scheduleModel.StudentArrivalSchedule{},
		arrivalExcByDate:     map[string]*scheduleModel.StudentArrivalException{},
		pickupSchedByWeekly:  map[int]*scheduleModel.StudentPickupSchedule{},
		pickupExcByDate:      map[string]*scheduleModel.StudentPickupException{},
	}

	if rs.timetableData == nil {
		return nil, errors.New("timetable data service not wired")
	}
	enrolledRows, err := rs.timetableData.GetStudentInstancesWithAttendance(
		ctx, studentID, from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("load enrolled instances: %w", err)
	}
	for _, row := range enrolledRows {
		k := dateKey(row.Instance.Date)
		out.enrolledByDate[k] = append(out.enrolledByDate[k], row)
	}

	allInstances, err := rs.timetableData.GetActivityInstancesByDateRange(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("load all tenant instances: %w", err)
	}

	// Collect active_group_ids across the entire range for a single visits
	// lookup. Only active/completed instances are relevant — planned/cancelled
	// never have visits bridged (per WP-B11 scope).
	activeGroupIDs := make([]int64, 0, len(allInstances))
	seenAGID := make(map[int64]bool, len(allInstances))
	for _, inst := range allInstances {
		k := dateKey(inst.Date)
		out.instancesByDate[k] = append(out.instancesByDate[k], inst)
		if inst.ActiveGroupID == nil {
			continue
		}
		if inst.Status != scheduleModel.InstanceStatusActive &&
			inst.Status != scheduleModel.InstanceStatusCompleted {
			continue
		}
		if !seenAGID[*inst.ActiveGroupID] {
			seenAGID[*inst.ActiveGroupID] = true
			activeGroupIDs = append(activeGroupIDs, *inst.ActiveGroupID)
		}
	}

	if len(activeGroupIDs) > 0 {
		visits, err := rs.timetableData.GetVisitsByStudentAndActiveGroupIDs(ctx, studentID, activeGroupIDs)
		if err != nil {
			return nil, fmt.Errorf("load student visits: %w", err)
		}
		for _, v := range visits {
			if v == nil {
				continue
			}
			out.visitsByActiveGroup[v.ActiveGroupID] = append(out.visitsByActiveGroup[v.ActiveGroupID], v)
		}
	}

	// Arrival/pickup weekly schedules: one query each returns up to 5 rows
	// total per student (one per weekday). Cheaper than per-weekday queries.
	arrivalSchedules, err := rs.timetableData.GetArrivalSchedulesByStudent(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("load arrival schedules: %w", err)
	}
	for _, s := range arrivalSchedules {
		out.arrivalSchedByWeekly[s.Weekday] = s
	}
	pickupSchedules, err := rs.timetableData.GetPickupSchedulesByStudent(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("load pickup schedules: %w", err)
	}
	for _, s := range pickupSchedules {
		out.pickupSchedByWeekly[s.Weekday] = s
	}

	// Arrival/pickup exceptions: range-scoped (avoid loading unbounded
	// history).
	arrivalExcs, err := rs.timetableData.GetArrivalExceptionsByStudentAndDateRange(ctx, studentID, from, to)
	if err != nil {
		return nil, fmt.Errorf("load arrival exceptions: %w", err)
	}
	for _, e := range arrivalExcs {
		out.arrivalExcByDate[dateKey(e.ExceptionDate)] = e
	}
	pickupExcs, err := rs.timetableData.GetPickupExceptionsByStudentAndDateRange(ctx, studentID, from, to)
	if err != nil {
		return nil, fmt.Errorf("load pickup exceptions: %w", err)
	}
	for _, e := range pickupExcs {
		out.pickupExcByDate[dateKey(e.ExceptionDate)] = e
	}

	return out, nil
}

// buildStudentDays assembles one StudentDayResponse per day in the inclusive
// [from, to] range. A single batch round trip per category fills weekPreload;
// from there the per-day assembly is a pure in-memory slice.
//
// Query count (constant in range length): 1 enrolled + 1 all-instances +
// 1 visits (if any active groups) + 2 schedules + 2 exceptions = max 7.
func (rs *Resource) buildStudentDays(ctx context.Context, studentID int64, from, to timezone.Date) ([]StudentDayResponse, error) {
	pre, err := rs.preloadWeek(ctx, studentID, from, to)
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
func buildStudentDayFromPreload(pre *weekPreload, studentID int64, date timezone.Date) StudentDayResponse {
	k := dateKey(date)

	enrolledRows := pre.enrolledByDate[k]
	dayInstances := pre.instancesByDate[k]

	// Build the base instance list from enrolled rows.
	enrolledInstanceIDs := make(map[int64]bool, len(enrolledRows))
	instances := make([]InstanceDayResponse, 0, len(enrolledRows)+len(dayInstances))
	for _, row := range enrolledRows {
		instances = append(instances, mapEnrolledInstance(row))
		enrolledInstanceIDs[row.Instance.ID] = true
	}

	// Index today's active/completed instances by active_group_id so we can
	// map visits back to their host instance.
	instByActiveGroupID := make(map[int64]*scheduleModel.ActivityInstance, len(dayInstances))
	for _, inst := range dayInstances {
		if inst.ActiveGroupID == nil {
			continue
		}
		if inst.Status != scheduleModel.InstanceStatusActive &&
			inst.Status != scheduleModel.InstanceStatusCompleted {
			continue
		}
		instByActiveGroupID[*inst.ActiveGroupID] = inst
	}

	// Append is_unplanned entries: the student has a visit on one of today's
	// active/completed instances, but no instance_students row. active_group_id
	// is 1:1 with activity_instance (each instance creates its own group on
	// start), so the map lookup already scopes visits to this date.
	for agID, inst := range instByActiveGroupID {
		if enrolledInstanceIDs[inst.ID] {
			continue
		}
		visits := pre.visitsByActiveGroup[agID]
		if len(visits) == 0 {
			continue
		}
		// Pick the first non-nil visit — dedup handled by one-entry-per-agID.
		var v *activeModel.Visit
		for _, candidate := range visits {
			if candidate != nil {
				v = candidate
				break
			}
		}
		if v == nil {
			continue
		}
		instances = append(instances, mapUnplannedInstance(inst, v))
	}

	sort.SliceStable(instances, func(i, j int) bool {
		return instances[i].StartTime < instances[j].StartTime
	})

	arrival := resolveArrivalSlotFromPreload(pre, date)
	pickup := resolvePickupSlotFromPreload(pre, date)

	return StudentDayResponse{
		StudentID: studentID,
		Date:      date.String(),
		Weekday:   isoWeekday(date),
		Arrival:   arrival,
		Instances: instances,
		Pickup:    pickup,
	}
}

// resolveArrivalSlotFromPreload implements the exception-over-schedule rule
// against already-loaded data. An exception on the date wins even when its
// time is nil (absence signal).
func resolveArrivalSlotFromPreload(pre *weekPreload, date timezone.Date) SlotResponse {
	if exc, ok := pre.arrivalExcByDate[dateKey(date)]; ok && exc != nil {
		return mapArrivalExceptionSlot(exc)
	}
	wd := isoWeekday(date)
	if wd >= scheduleModel.WeekdayMonday && wd <= scheduleModel.WeekdayFriday {
		if sched, ok := pre.arrivalSchedByWeekly[wd]; ok && sched != nil {
			return mapArrivalScheduleSlot(sched)
		}
	}
	return SlotResponse{Source: SlotSourceNone}
}

// resolvePickupSlotFromPreload mirrors resolveArrivalSlotFromPreload.
func resolvePickupSlotFromPreload(pre *weekPreload, date timezone.Date) SlotResponse {
	if exc, ok := pre.pickupExcByDate[dateKey(date)]; ok && exc != nil {
		return mapPickupExceptionSlot(exc)
	}
	wd := isoWeekday(date)
	if wd >= scheduleModel.WeekdayMonday && wd <= scheduleModel.WeekdayFriday {
		if sched, ok := pre.pickupSchedByWeekly[wd]; ok && sched != nil {
			return mapPickupScheduleSlot(sched)
		}
	}
	return SlotResponse{Source: SlotSourceNone}
}
