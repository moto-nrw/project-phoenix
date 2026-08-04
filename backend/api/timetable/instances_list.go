// Package timetable — WP-F2 backend prerequisite: weekly instance list.
//
//	GET /api/timetable/instances?from=YYYY-MM-DD&to=YYYY-MM-DD
//
// Lists all materialized activity instances in the requested window for the
// current tenant, enriched with room name, activity-group type, staffing
// counts, and expected/present student counts. Powers the admin weekly
// planner UI (database/timetables). Permission: SchedulesRead.
//
// The window is capped at 56 days (8 weeks) to bound the response size and
// prevent accidental DoS via wide range queries. Individual instance lookups
// follow an N+1 pattern for staff/student rows; this is acceptable at the
// expected scale (~30 instances per week per tenant). If the planner ever
// extends to a multi-week overview, replace per-instance loads with batched
// repo helpers similar to instanceStaffRepo.CountNonAbsentByInstanceIDs.
package timetable

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// maxInstanceListRangeDays caps the /instances list window. 56 days = 8 weeks
// covers any planning horizon the OGS-Office realistically needs in one
// request; longer ranges should be paginated by the client.
const maxInstanceListRangeDays = 56

// instanceStaffSummary is one staff assignment as it appears on an enriched
// instance. Names are intentionally omitted at the list level to keep the
// payload compact; the slide-over detail view fetches them on demand.
type instanceStaffSummary struct {
	StaffID       int64   `json:"staff_id"`
	IsPrimary     bool    `json:"is_primary"`
	IsAbsent      bool    `json:"is_absent"`
	IsSubstitute  bool    `json:"is_substitute"`
	AbsenceReason *string `json:"absence_reason,omitempty"`
}

// instanceStudentSummary carries the editable attendance state for one child.
// The legacy student_ids array stays in the payload for older clients, but new
// planner UI should use Students so it can group expected/present/absent rows.
type instanceStudentSummary struct {
	StudentID   int64   `json:"student_id"`
	Status      string  `json:"status"`
	Substatus   *string `json:"substatus,omitempty"`
	Note        *string `json:"note,omitempty"`
	CheckedInAt *string `json:"checked_in_at,omitempty"`
	// CareDayStatus is the same per-child care-day verdict the active-
	// supervision roster carries: "scheduled" | "not_scheduled" | "cancelled"
	// | "unknown" (#1747). The counts below exclude the non-expected ones, so
	// the row has to say so too — otherwise the planner lists a child under
	// "Erwartet" that its own header count leaves out.
	CareDayStatus scheduleSvc.CareDayStatus `json:"care_day_status"`
}

// enrichedInstance is the per-instance payload returned in the list response.
//
// Status values mirror scheduleModel.InstanceStatus* constants:
// "planned" | "active" | "completed" | "cancelled".
//
// activity_type values mirror activitiesModel.GroupType* constants:
// "activity" | "care" | "external". Spontaneous instances without a template
// fall back to "activity" so the frontend has a deterministic colour key.
type enrichedInstance struct {
	ID                     int64                    `json:"id"`
	Date                   string                   `json:"date"`
	StartTime              string                   `json:"start_time"`
	EndTime                string                   `json:"end_time"`
	Title                  string                   `json:"title"`
	Description            *string                  `json:"description,omitempty"`
	Notes                  *string                  `json:"notes,omitempty"`
	SeriesNotes            *string                  `json:"series_notes,omitempty"`
	Status                 string                   `json:"status"`
	IsSpontaneous          bool                     `json:"is_spontaneous"`
	IsLive                 bool                     `json:"is_live"`
	ActivityGroupID        *int64                   `json:"activity_group_id,omitempty"`
	ListKind               *string                  `json:"list_kind,omitempty"`
	ActivityType           string                   `json:"activity_type"`
	PlanningTrackID        *int64                   `json:"planning_track_id,omitempty"`
	PlanningTrackName      string                   `json:"planning_track_name,omitempty"`
	PlanningTrackColor     string                   `json:"planning_track_color,omitempty"`
	PlanningTrackSortOrder *int                     `json:"planning_track_sort_order,omitempty"`
	RoomID                 int64                    `json:"room_id"`
	RoomName               string                   `json:"room_name"`
	Staff                  []instanceStaffSummary   `json:"staff"`
	StudentIDs             []int64                  `json:"student_ids"`
	Students               []instanceStudentSummary `json:"students"`
	StaffCount             int                      `json:"staff_count"`
	AbsentStaffCount       int                      `json:"absent_staff_count"`
	UnderstaffedAck        bool                     `json:"understaffed_ack"`
	UnderstaffedNote       *string                  `json:"understaffed_note,omitempty"`
	CancelReason           *string                  `json:"cancel_reason,omitempty"`
	ExpectedStudentsCount  int                      `json:"expected_students_count"`
	PresentStudentsCount   int                      `json:"present_students_count"`
	// NotScheduledCount is how many assigned children are not in care here on
	// this day (#1747) — not booked on this weekday, or the day was cancelled.
	// Excluded from ExpectedStudentsCount and from the staffing maths;
	// surfaced so the planner can show why the expected number is lower than
	// the assignment list. Which children those are is on the per-row
	// care_day_status.
	NotScheduledCount int `json:"not_scheduled_students_count"`
	// RequiredStaffCount and AssignedStaffCount drive the Betreuungsplan
	// capacity indicator (issue #1838): required is
	// ceil(children/Betreuungsschlüssel), assigned is the non-absent staff
	// count already computed above (StaffCount - AbsentStaffCount). The
	// frontend derives "understaffed" as assigned < required, the same
	// pattern already used for every other count on this payload — this is
	// intentionally not modeled as a ConflictWarning (see
	// services/schedule/capacity_service.go).
	RequiredStaffCount int `json:"required_staff_count"`
	AssignedStaffCount int `json:"assigned_staff_count"`
	// RequiredStaffOverride is the raw per-occurrence Personalbedarf pin
	// (#1839), nil when the block inherits: template-backed instances fall
	// back to the template's override, then to the Betreuungsschlüssel. The
	// edit form needs the raw value to distinguish "inherit" from a pinned
	// number; RequiredStaffCount above already folds the inheritance in.
	RequiredStaffOverride *int                                  `json:"required_staff_override,omitempty"`
	ConflictWarnings      []scheduleSvc.InstanceConflictWarning `json:"conflict_warnings"`
}

// weeklyInstancesResponse is the 200 body for GET /instances.
type weeklyInstancesResponse struct {
	From      string             `json:"from"`
	To        string             `json:"to"`
	Instances []enrichedInstance `json:"instances"`
}

// listInstances handles GET /api/timetable/instances?from=&to=.
func (rs *Resource) listInstances(w http.ResponseWriter, r *http.Request) {
	if rs.TimetableData == nil {
		common.RenderError(w, r, common.ErrorInternalServer(
			errors.New("timetable resource not fully wired")))
		return
	}

	q := r.URL.Query()
	fromStr := q.Get("from")
	toStr := q.Get("to")
	if fromStr == "" || toStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("from and to query params are required (YYYY-MM-DD)")))
		return
	}

	from, err := berlinDate(fromStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("invalid from format, expected YYYY-MM-DD")))
		return
	}
	to, err := berlinDate(toStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("invalid to format, expected YYYY-MM-DD")))
		return
	}

	if to.Before(from) {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("'to' must be on or after 'from'")))
		return
	}
	if inclusiveDayCount(from, to) > maxInstanceListRangeDays {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			fmt.Errorf("date range exceeds maximum of %d days", maxInstanceListRangeDays)))
		return
	}

	ctx := r.Context()

	instances, err := rs.TimetableData.GetActivityInstancesByDateRange(ctx, from, to)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap(
			"load instances failed", err))
		return
	}

	// Cache room and activity-group lookups for the request. ~5-8 unique
	// rooms and templates per week — caching turns 30 lookups into ~10.
	roomCache := make(map[int64]string)
	typeCache := make(map[int64]templateMeta)
	planningTrackCache := make(map[int64]*scheduleModel.PlanningTrack)

	// Resolved once per request (not per instance) — the Betreuungsschlüssel
	// setting is tenant-wide, not per-block.
	ratio := rs.childrenPerStaffRatio(ctx)

	careDays, err := rs.resolveCareDays(ctx, instances, from, to)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap(
			"resolve care days failed", err))
		return
	}

	enriched := make([]enrichedInstance, 0, len(instances))
	conflictInputs := make([]scheduleSvc.WindowConflictInput, 0, len(instances))
	for _, inst := range instances {
		item, staffRows, studentRows, err := rs.enrichInstance(ctx, inst, roomCache, typeCache, planningTrackCache, ratio, careDays)
		if err != nil {
			common.RenderError(w, r, common.ErrorInternalServerWrap(
				"enrich instance failed", err))
			return
		}
		enriched = append(enriched, item)
		conflictInputs = append(conflictInputs, scheduleSvc.WindowConflictInput{
			Instance: inst,
			Staff:    staffRows,
			Students: studentRows,
		})
	}

	// Window-wide person double-bookings (#2139): the banner's "diese Woche" /
	// "diesen Monat" claim holds because detection covers exactly the
	// requested window, not just today. The rows were already loaded for
	// enrichment, so this adds no queries.
	conflictsByInstance := scheduleSvc.DetectWindowConflicts(conflictInputs)
	for i := range enriched {
		if warnings, ok := conflictsByInstance[enriched[i].ID]; ok {
			enriched[i].ConflictWarnings = warnings
		}
	}

	resp := weeklyInstancesResponse{
		From:      from.Format(dateLayout),
		To:        to.Format(dateLayout),
		Instances: enriched,
	}

	rs.getLogger().Info("timetable instances list",
		slog.String("from", resp.From),
		slog.String("to", resp.To),
		slog.Int("instance_count", len(enriched)),
	)
	common.Respond(w, r, http.StatusOK, resp, "Instances retrieved")
}

// enrichInstance loads room name, activity-group type, staff list, and
// student counts for a single instance. Room and type lookups consult the
// per-request caches to avoid duplicate queries when many instances share a
// template (e.g. the daily Mensa).
// resolveCareDays derives, for the whole window at once, which assigned
// children the care plan actually places in the OGS on which day (#1747).
//
// The pre-pass asks for the rows whose verdict can still change something:
// still 'expected', or flipped to 'absent' by a broad day status that still
// owns them. A sick report lands on every expected row of the day, including
// the days the child was never booked into care, so restricting the pre-pass
// to 'expected' would leave exactly those rows unresolved and displayed as
// ordinary absences (#1747). The window is capped at 56 days, but the cost does
// not scale with it: weekly care plans are recurring, so the derivation loads
// them once and combines them with the window's exceptions in memory.
//
// An unwired service yields an empty map, which reads as "unknown" everywhere
// and leaves every count exactly as it was before this feature.
func (rs *Resource) resolveCareDays(
	ctx context.Context,
	instances []*scheduleModel.ActivityInstance,
	from, to timezone.Date,
) (map[int64]map[timezone.Date]scheduleSvc.CareDayStatus, error) {
	empty := map[int64]map[timezone.Date]scheduleSvc.CareDayStatus{}
	if rs.CareDayService == nil || rs.TimetableData == nil || len(instances) == 0 {
		return empty, nil
	}

	instanceIDs := make([]int64, 0, len(instances))
	for _, inst := range instances {
		if inst != nil {
			instanceIDs = append(instanceIDs, inst.ID)
		}
	}

	rows, err := rs.TimetableData.GetCareDayCandidateStudentsByInstanceIDs(ctx, instanceIDs)
	if err != nil {
		return nil, fmt.Errorf("load care-day candidate students: %w", err)
	}

	seen := make(map[int64]bool, len(rows))
	studentIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if !seen[row.StudentID] {
			seen[row.StudentID] = true
			studentIDs = append(studentIDs, row.StudentID)
		}
	}
	if len(studentIDs) == 0 {
		return empty, nil
	}

	return rs.CareDayService.ResolveForRange(ctx, studentIDs, from, to)
}

// careDaysForInstance resolves the care-day map for a single instance, used by
// the create/update paths that re-enrich one row.
//
// A failure is returned, never swallowed (#1747 review). An empty map does not
// mean "no verdict yet" to the reader — it reads as unknown, which is the
// verdict that puts every assigned child back into "Erwartet". Degrading to it
// would answer a successful write with counts that silently contradict the
// planner the very next reload corrects, and the caller cannot tell the two
// apart. Both call sites already have a path for "the write committed, the
// enrichment did not" and route this into it.
func (rs *Resource) careDaysForInstance(
	ctx context.Context, inst *scheduleModel.ActivityInstance,
) (map[int64]map[timezone.Date]scheduleSvc.CareDayStatus, error) {
	if inst == nil {
		return map[int64]map[timezone.Date]scheduleSvc.CareDayStatus{}, nil
	}
	careDays, err := rs.resolveCareDays(ctx, []*scheduleModel.ActivityInstance{inst}, inst.Date, inst.Date)
	if err != nil {
		return nil, fmt.Errorf("resolve care days for instance %d: %w", inst.ID, err)
	}
	return careDays, nil
}

// instanceStudentCareDay picks the care-day verdict reported for one
// assignment row — the single source both the per-child payload and the
// counts read, so a child can never be listed as "Erwartet" while the header
// count leaves them out (#1747 review).
//
// The rules live in scheduleSvc.AttendanceRowCareDay, shared with the operation
// roster and the planned-now cards; this only looks up the plan verdict for the
// instance's date.
func instanceStudentCareDay(
	inst *scheduleModel.ActivityInstance,
	row *scheduleModel.InstanceStudent,
	careDays map[int64]map[timezone.Date]scheduleSvc.CareDayStatus,
) scheduleSvc.CareDayStatus {
	return scheduleSvc.AttendanceRowCareDay(
		inst.Status == scheduleModel.InstanceStatusCompleted,
		row,
		careDays[row.StudentID][inst.Date],
	)
}

// instanceAttendanceSummary is the per-instance student payload plus the three
// counts derived from it. They are built together so the rows and the header
// counts can never disagree about the same child (#1747 review).
type instanceAttendanceSummary struct {
	studentIDs   []int64
	students     []instanceStudentSummary
	expected     int
	present      int
	notScheduled int
}

// summarizeInstanceStudents groups one instance's attendance rows by the
// care-day verdict instanceStudentCareDay reports for each of them.
func summarizeInstanceStudents(
	inst *scheduleModel.ActivityInstance,
	studentRows []*scheduleModel.InstanceStudent,
	careDays map[int64]map[timezone.Date]scheduleSvc.CareDayStatus,
) instanceAttendanceSummary {
	out := instanceAttendanceSummary{
		studentIDs: make([]int64, 0, len(studentRows)),
		students:   make([]instanceStudentSummary, 0, len(studentRows)),
	}
	for _, row := range studentRows {
		out.studentIDs = append(out.studentIDs, row.StudentID)
		var checkedInAt *string
		if row.CheckedInAt != nil {
			formatted := row.CheckedInAt.UTC().Format("2006-01-02T15:04:05Z07:00")
			checkedInAt = &formatted
		}
		careDayStatus := instanceStudentCareDay(inst, row, careDays)
		out.students = append(out.students, instanceStudentSummary{
			StudentID:     row.StudentID,
			Status:        row.Status,
			Substatus:     row.Substatus,
			Note:          row.Note,
			CheckedInAt:   checkedInAt,
			CareDayStatus: careDayStatus,
		})
		switch row.Status {
		case scheduleModel.AttendanceStatusExpected:
			// Assigned but not in care today: counted separately, and left out
			// of the staffing maths — planning for children who are not there
			// that day inflates the Betreuungsschlüssel (#1747). The row carries
			// the same verdict, so the slide-over can group it exactly the way
			// this count does.
			if !careDayStatus.Expected() {
				out.notScheduled++
				continue
			}
			out.expected++
		case scheduleModel.AttendanceStatusPresent:
			out.present++
		case scheduleModel.AttendanceStatusAbsent:
			// A broad day status wrote this absence onto a day the care plan
			// never booked — the block has not ended yet, so nothing has undone
			// it. Group it where the verdict says it belongs instead of showing
			// a school an absence from care that was never owed (#1747).
			// instanceStudentCareDay hands out this verdict for no other absent
			// row, so a manual absence is untouched.
			if careDayStatus == scheduleSvc.CareDayNotScheduled {
				out.notScheduled++
			}
		}
	}
	return out
}

// enrichInstance additionally returns the raw staff and student rows it
// loaded so the caller can feed them into the window-wide conflict detection
// (#2139) without a second round of queries.
func (rs *Resource) enrichInstance(
	ctx context.Context,
	inst *scheduleModel.ActivityInstance,
	roomCache map[int64]string,
	metaCache map[int64]templateMeta,
	planningTrackCache map[int64]*scheduleModel.PlanningTrack,
	childrenPerStaffRatio int,
	careDays map[int64]map[timezone.Date]scheduleSvc.CareDayStatus,
) (enrichedInstance, []*scheduleModel.InstanceStaff, []*scheduleModel.InstanceStudent, error) {
	if inst == nil {
		return enrichedInstance{}, nil, nil, errors.New("nil instance")
	}

	roomName := rs.lookupRoomName(ctx, inst.RoomID, roomCache)
	meta := rs.lookupTemplateMeta(ctx, inst.ActivityGroupID, metaCache, planningTrackCache)

	staffRows, err := rs.TimetableData.GetInstanceStaff(ctx, inst.ID)
	if err != nil {
		return enrichedInstance{}, nil, nil, fmt.Errorf("load staff for instance %d: %w", inst.ID, err)
	}
	staff := make([]instanceStaffSummary, 0, len(staffRows))
	absentCount := 0
	for _, row := range staffRows {
		if row.IsAbsent {
			absentCount++
		}
		staff = append(staff, instanceStaffSummary{
			StaffID:       row.StaffID,
			IsPrimary:     row.IsPrimary,
			IsAbsent:      row.IsAbsent,
			IsSubstitute:  row.IsSubstitute,
			AbsenceReason: row.AbsenceReason,
		})
	}

	studentRows, err := rs.TimetableData.GetInstanceStudents(ctx, inst.ID)
	if err != nil {
		return enrichedInstance{}, nil, nil, fmt.Errorf("load students for instance %d: %w", inst.ID, err)
	}
	attendance := summarizeInstanceStudents(inst, studentRows, careDays)

	assignedStaff := len(staffRows) - absentCount
	childrenCount := attendance.expected + attendance.present

	item := enrichedInstance{
		ID:                     inst.ID,
		Date:                   inst.Date.Format(dateLayout),
		StartTime:              inst.StartTime.Format("15:04"),
		EndTime:                inst.EndTime.Format("15:04"),
		Title:                  inst.Title,
		Description:            inst.Description,
		Notes:                  inst.Notes,
		SeriesNotes:            meta.seriesNotes,
		Status:                 inst.Status,
		IsSpontaneous:          inst.IsSpontaneous,
		IsLive:                 inst.Status == scheduleModel.InstanceStatusActive && inst.ActiveGroupID != nil,
		ActivityGroupID:        inst.ActivityGroupID,
		ListKind:               inst.ListKind,
		ActivityType:           meta.activityType,
		PlanningTrackID:        meta.planningTrackID,
		PlanningTrackName:      meta.planningTrackName,
		PlanningTrackColor:     meta.planningTrackColor,
		PlanningTrackSortOrder: meta.planningTrackSortOrder,
		RoomID:                 inst.RoomID,
		RoomName:               roomName,
		Staff:                  staff,
		StudentIDs:             attendance.studentIDs,
		Students:               attendance.students,
		StaffCount:             len(staffRows),
		AbsentStaffCount:       absentCount,
		UnderstaffedAck:        inst.UnderstaffedAck,
		UnderstaffedNote:       inst.UnderstaffedNote,
		CancelReason:           inst.CancelReason,
		ExpectedStudentsCount:  attendance.expected,
		PresentStudentsCount:   attendance.present,
		NotScheduledCount:      attendance.notScheduled,
		RequiredStaffCount:     scheduleSvc.EffectiveRequiredStaff(instanceRequiredStaffOverride(inst.RequiredStaff, meta.requiredStaff), childrenCount, childrenPerStaffRatio),
		AssignedStaffCount:     assignedStaff,
		RequiredStaffOverride:  inst.RequiredStaff,
		ConflictWarnings:       []scheduleSvc.InstanceConflictWarning{},
	}
	return item, staffRows, studentRows, nil
}

// dayConflictWarningsFor computes the #2139 window conflicts for ONE instance
// against every other planned/active instance of its day. Used by the
// create/update paths that re-enrich a single row; the list path batches the
// same detection over its whole window instead. Degrades to an empty slice on
// load errors — conflicts are advisory and must never fail a committed write.
func (rs *Resource) dayConflictWarningsFor(
	ctx context.Context,
	inst *scheduleModel.ActivityInstance,
) []scheduleSvc.InstanceConflictWarning {
	empty := []scheduleSvc.InstanceConflictWarning{}
	if inst == nil || rs.TimetableData == nil {
		return empty
	}
	dayInstances, err := rs.TimetableData.GetActivityInstancesByDateRange(ctx, inst.Date, inst.Date)
	if err != nil {
		rs.getLogger().Warn("day conflict detection: load day instances failed",
			slog.Int64("instance_id", inst.ID),
			slog.String("date", inst.Date.String()),
			slog.String("error", err.Error()),
		)
		return empty
	}
	inputs := make([]scheduleSvc.WindowConflictInput, 0, len(dayInstances))
	for _, dayInst := range dayInstances {
		staffRows, err := rs.TimetableData.GetInstanceStaff(ctx, dayInst.ID)
		if err != nil {
			rs.getLogger().Warn("day conflict detection: load instance_staff failed",
				slog.Int64("instance_id", dayInst.ID),
				slog.String("error", err.Error()),
			)
			return empty
		}
		studentRows, err := rs.TimetableData.GetInstanceStudents(ctx, dayInst.ID)
		if err != nil {
			rs.getLogger().Warn("day conflict detection: load instance_students failed",
				slog.Int64("instance_id", dayInst.ID),
				slog.String("error", err.Error()),
			)
			return empty
		}
		inputs = append(inputs, scheduleSvc.WindowConflictInput{
			Instance: dayInst,
			Staff:    staffRows,
			Students: studentRows,
		})
	}
	if warnings, ok := scheduleSvc.DetectWindowConflicts(inputs)[inst.ID]; ok {
		return warnings
	}
	return empty
}

// lookupRoomName resolves a room id to its display name, with per-request
// memoisation. Returns an empty string if the repo is unwired or the lookup
// fails — the planner shows "Raum #ID" in that case so the user is not blocked.
func (rs *Resource) lookupRoomName(ctx context.Context, roomID int64, cache map[int64]string) string {
	if name, ok := cache[roomID]; ok {
		return name
	}
	if rs.TimetableData == nil {
		cache[roomID] = ""
		return ""
	}
	room, err := rs.TimetableData.GetRoom(ctx, roomID)
	if err != nil || room == nil {
		// Logged at debug only — a missing room reference here is recoverable.
		rs.getLogger().Debug("instance list: room lookup failed",
			slog.Int64("room_id", roomID),
		)
		cache[roomID] = ""
		return ""
	}
	cache[roomID] = room.Name
	return room.Name
}

// lookupActivityType resolves an activity-group id to its type field
// ("activity" | "care" | "external"). For spontaneous instances without an
// activity-group reference, falls back to GroupTypeActivity so the frontend
// always has a deterministic colour key.
// templateMeta caches the per-template fields enrichInstance needs so many
// instances sharing a template (e.g. the daily Mensa) cost one group lookup.
type templateMeta struct {
	activityType           string
	requiredStaff          *int
	planningTrackID        *int64
	planningTrackName      string
	planningTrackColor     string
	planningTrackSortOrder *int
	// seriesNotes is the template's durable Wochennotiz (#1837 follow-up),
	// joined onto each materialized instance at read time so it shows on every
	// occurrence and survives Re-Plan/Split without an instance column.
	seriesNotes *string
}

func (rs *Resource) lookupTemplateMeta(
	ctx context.Context,
	activityGroupID *int64,
	cache map[int64]templateMeta,
	planningTrackCache map[int64]*scheduleModel.PlanningTrack,
) templateMeta {
	fallback := templateMeta{activityType: activitiesModel.GroupTypeActivity}
	if activityGroupID == nil {
		return fallback
	}
	if meta, ok := cache[*activityGroupID]; ok {
		return meta
	}
	if rs.TimetableData == nil {
		cache[*activityGroupID] = fallback
		return fallback
	}
	group, err := rs.TimetableData.GetActivityGroup(ctx, *activityGroupID)
	if err != nil || group == nil {
		rs.getLogger().Debug("instance list: activity group lookup failed",
			slog.Int64("activity_group_id", *activityGroupID),
		)
		cache[*activityGroupID] = fallback
		return fallback
	}
	meta := templateMeta{activityType: group.Type, requiredStaff: group.RequiredStaff, seriesNotes: group.Notes, planningTrackID: group.PlanningTrackID}
	if group.PlanningTrackID != nil && rs.PlanningTrackService != nil {
		track, cached := planningTrackCache[*group.PlanningTrackID]
		if !cached {
			var trackErr error
			track, trackErr = rs.PlanningTrackService.GetPlanningTrack(ctx, *group.PlanningTrackID)
			planningTrackCache[*group.PlanningTrackID] = track
			if trackErr != nil {
				rs.getLogger().Debug("instance list: planning track lookup failed",
					slog.Int64("planning_track_id", *group.PlanningTrackID),
				)
			}
		}
		if track != nil {
			meta.planningTrackName = track.Name
			meta.planningTrackColor = track.Color
			sortOrder := track.SortOrder
			meta.planningTrackSortOrder = &sortOrder
		}
	}
	cache[*activityGroupID] = meta
	return meta
}

// instanceRequiredStaffOverride resolves the override EffectiveRequiredStaff
// should see for an instance: the per-occurrence pin when set, otherwise the
// template's Personalbedarf override — materialized rows leave the column
// NULL and inherit the template value at read time (#1839).
func instanceRequiredStaffOverride(pin, templateOverride *int) *int {
	if pin != nil {
		return pin
	}
	return templateOverride
}
