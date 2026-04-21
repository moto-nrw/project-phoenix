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
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
)

// maxWeekRangeDays caps /week at 14 days inclusive. Longer spans are
// rejected at 400 — the response assembly is O(days × instances) and the
// frontend has no use case for a longer horizon today.
const maxWeekRangeDays = 14

// berlinDate parses a YYYY-MM-DD input anchored in Berlin timezone. The
// distinction matters for the 00:00–02:00 CET/UTC gap: a UTC-anchored parse
// of "2026-04-22" would be midnight UTC (02:00 Berlin), and a Berlin-DateOf
// round-trip would land on the previous day. Using ParseInLocation sidesteps
// that entirely.
func berlinDate(input string) (time.Time, error) {
	return time.ParseInLocation(dateLayout, input, timezone.Berlin)
}

// isoWeekday returns 1..7 (Mon..Sun) for the Berlin-local weekday of t.
func isoWeekday(t time.Time) int {
	return int((int(t.In(timezone.Berlin).Weekday())+6)%7 + 1)
}

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

	day, err := rs.buildStudentDay(ctx, student.ID, date)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("build student day failed", err))
		return
	}

	rs.getLogger().Info("timetable student day",
		slog.Int64("student_id", student.ID),
		slog.String("date", day.Date),
	)
	common.Respond(w, r, http.StatusOK, day, "Student day retrieved")
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
	rangeDays := int(to.Sub(from).Hours()/24) + 1
	if rangeDays > maxWeekRangeDays {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			fmt.Errorf("date range exceeds maximum of %d days", maxWeekRangeDays)))
		return
	}

	student, ok := rs.resolveStudentForRead(w, r, studentID)
	if !ok {
		return
	}

	days := make([]StudentDayResponse, 0, rangeDays)
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		dayResp, err := rs.buildStudentDay(ctx, student.ID, d)
		if err != nil {
			common.RenderError(w, r, common.ErrorInternalServerWrap("build student week day failed", err))
			return
		}
		days = append(days, *dayResp)
	}

	resp := StudentWeekResponse{
		StudentID: student.ID,
		From:      from.Format(dateLayout),
		To:        to.Format(dateLayout),
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

	if rs.studentRepo == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("student repo not wired")))
		return nil, false
	}

	student, err := rs.studentRepo.FindByID(ctx, studentID)
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

// buildStudentDay assembles the per-day response. Four reads:
//  1. Enrolled instances + attendance rows (1 query, joined).
//  2. All activity_instances in the tenant for the date (1 query) — needed
//     to resolve visit.active_group_id → instance for is_unplanned entries.
//  3. Student's visits scoped to those active_group_ids (1 query).
//  4. Arrival/pickup schedule + exception (up to 4 queries).
//
// That is O(1) in the number of instances per day — no N+1.
func (rs *Resource) buildStudentDay(ctx context.Context, studentID int64, date time.Time) (*StudentDayResponse, error) {
	dateUTC := timezone.DateOfUTC(date)

	if rs.studentDayRepo == nil {
		return nil, errors.New("studentDayRepo not wired")
	}

	// 1) Enrolled instances for the student on this date.
	enrolledRows, err := rs.studentDayRepo.FindInstancesWithAttendanceByStudentAndDateRange(
		ctx, studentID, dateUTC, dateUTC,
	)
	if err != nil {
		return nil, fmt.Errorf("load enrolled instances: %w", err)
	}

	// 2) All instances in the tenant on this date — needed to map visit
	//    active_group_ids back to an instance for is_unplanned entries.
	allInstances, err := rs.activityInstanceRepo.FindByTenantAndDateRange(ctx, dateUTC, dateUTC)
	if err != nil {
		return nil, fmt.Errorf("load all tenant instances: %w", err)
	}

	// Index instances by active_group_id for the active/completed subset.
	// Planned/cancelled never have visits bridged (per WP-B11 scope), so we
	// skip them in the index.
	instByActiveGroupID := make(map[int64]*scheduleModel.ActivityInstance, len(allInstances))
	activeGroupIDs := make([]int64, 0, len(allInstances))
	for _, inst := range allInstances {
		if inst.ActiveGroupID == nil {
			continue
		}
		if inst.Status != scheduleModel.InstanceStatusActive && inst.Status != scheduleModel.InstanceStatusCompleted {
			continue
		}
		instByActiveGroupID[*inst.ActiveGroupID] = inst
		activeGroupIDs = append(activeGroupIDs, *inst.ActiveGroupID)
	}

	// 3) Student visits scoped to the active/completed instances we know
	//    about. Empty list short-circuits with no DB call.
	var studentVisits []*activeModel.Visit
	if len(activeGroupIDs) > 0 && rs.visitRepo != nil {
		studentVisits, err = rs.visitRepo.FindByStudentAndActiveGroupIDs(ctx, studentID, activeGroupIDs)
		if err != nil {
			return nil, fmt.Errorf("load student visits: %w", err)
		}
	}

	// Build the base instance list from enrolled rows.
	enrolledInstanceIDs := make(map[int64]bool, len(enrolledRows))
	instances := make([]InstanceDayResponse, 0, len(enrolledRows)+len(studentVisits))
	for _, row := range enrolledRows {
		instances = append(instances, mapEnrolledInstance(row))
		enrolledInstanceIDs[row.Instance.ID] = true
	}

	// Append is_unplanned entries: visits whose instance is NOT in the
	// enrolled set. Dedup by active_group_id in case of (rare) multi-visit
	// against the same bridge.
	seenActiveGroupID := make(map[int64]bool, len(studentVisits))
	for _, v := range studentVisits {
		if v == nil {
			continue
		}
		inst, ok := instByActiveGroupID[v.ActiveGroupID]
		if !ok {
			continue
		}
		if enrolledInstanceIDs[inst.ID] {
			continue
		}
		if seenActiveGroupID[v.ActiveGroupID] {
			continue
		}
		seenActiveGroupID[v.ActiveGroupID] = true
		instances = append(instances, mapUnplannedInstance(inst, v))
	}

	sort.SliceStable(instances, func(i, j int) bool {
		return instances[i].StartTime < instances[j].StartTime
	})

	// 4) Arrival / pickup slots.
	arrival, err := rs.resolveArrivalSlot(ctx, studentID, dateUTC)
	if err != nil {
		return nil, fmt.Errorf("resolve arrival: %w", err)
	}
	pickup, err := rs.resolvePickupSlot(ctx, studentID, dateUTC)
	if err != nil {
		return nil, fmt.Errorf("resolve pickup: %w", err)
	}

	return &StudentDayResponse{
		StudentID: studentID,
		Date:      date.Format(dateLayout),
		Weekday:   isoWeekday(date),
		Arrival:   arrival,
		Instances: instances,
		Pickup:    pickup,
	}, nil
}

// resolveArrivalSlot implements the exception-over-schedule precedence rule.
// An arrival exception with a non-nil time wins over any weekday schedule.
// An exception with nil time means "absent" on that date — still an
// exception, still wins (source="exception", expected_time=null).
func (rs *Resource) resolveArrivalSlot(ctx context.Context, studentID int64, date time.Time) (SlotResponse, error) {
	if rs.arrivalExceptionRepo != nil {
		exc, err := rs.arrivalExceptionRepo.FindByStudentIDAndDate(ctx, studentID, date)
		if err != nil {
			return SlotResponse{}, err
		}
		if exc != nil {
			return mapArrivalExceptionSlot(exc), nil
		}
	}

	if rs.arrivalScheduleRepo != nil {
		wd := isoWeekday(date)
		if wd >= scheduleModel.WeekdayMonday && wd <= scheduleModel.WeekdayFriday {
			sched, err := rs.arrivalScheduleRepo.FindByStudentIDAndWeekday(ctx, studentID, wd)
			if err != nil {
				return SlotResponse{}, err
			}
			if sched != nil {
				return mapArrivalScheduleSlot(sched), nil
			}
		}
	}

	return SlotResponse{Source: SlotSourceNone}, nil
}

// resolvePickupSlot mirrors resolveArrivalSlot.
func (rs *Resource) resolvePickupSlot(ctx context.Context, studentID int64, date time.Time) (SlotResponse, error) {
	if rs.pickupExceptionRepo != nil {
		exc, err := rs.pickupExceptionRepo.FindByStudentIDAndDate(ctx, studentID, date)
		if err != nil {
			return SlotResponse{}, err
		}
		if exc != nil {
			return mapPickupExceptionSlot(exc), nil
		}
	}

	if rs.pickupScheduleRepo != nil {
		wd := isoWeekday(date)
		if wd >= scheduleModel.WeekdayMonday && wd <= scheduleModel.WeekdayFriday {
			sched, err := rs.pickupScheduleRepo.FindByStudentIDAndWeekday(ctx, studentID, wd)
			if err != nil {
				return SlotResponse{}, err
			}
			if sched != nil {
				return mapPickupScheduleSlot(sched), nil
			}
		}
	}

	return SlotResponse{Source: SlotSourceNone}, nil
}

// Compile-time hint for future readers: ScheduledInstanceRow must stay
// exported from the repo package because mapEnrolledInstance takes it.
var _ = (*scheduleRepo.ScheduledInstanceRow)(nil)
