// Package timetablehttp — WP-F2 backend prerequisite: weekly instance list.
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
package timetablehttp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
	IsSickAbsence bool    `json:"is_sick_absence"`
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
	CareDayStatus careplan.CareDayStatus `json:"care_day_status"`
	// EarlyPickupTime (HH:MM) is set when the child's day pickup cutoff falls
	// INSIDE this block (block 14:00-15:00, Abholung 14:45): the row stays
	// expected — the child attends the beginning — but leaves early, and must
	// not be silently misfiled either way (#2360). Blocks fully after the
	// cutoff are already absent/excused and carry no marker.
	EarlyPickupTime *string `json:"early_pickup_time,omitempty"`
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
	CalendarPeriodID       *int64                   `json:"calendar_period_id,omitempty"`
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
	EmptyRosterReason      *emptyRosterReason       `json:"empty_roster_reason,omitempty"`
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
	// modules/timetable/staffing.go).
	RequiredStaffCount int `json:"required_staff_count"`
	AssignedStaffCount int `json:"assigned_staff_count"`
	// RequiredStaffOverride is the raw per-occurrence Personalbedarf pin
	// (#1839), nil when the block inherits: template-backed instances fall
	// back to the template's override, then to the Betreuungsschlüssel. The
	// edit form needs the raw value to distinguish "inherit" from a pinned
	// number; RequiredStaffCount above already folds the inheritance in.
	RequiredStaffOverride *int                                `json:"required_staff_override,omitempty"`
	ConflictWarnings      []timetable.InstanceConflictWarning `json:"conflict_warnings"`
	CanReopen             bool                                `json:"can_reopen,omitempty"`
	CanComplete           bool                                `json:"can_complete"`
	CompleteAvailableAt   string                              `json:"complete_available_at"`
}

type emptyRosterReason struct {
	Kind             string `json:"kind"`
	PhaseName        string `json:"phase_name,omitempty"`
	ServiceStartDate string `json:"service_start_date,omitempty"`
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

	from, to, ok := parseInstanceListWindow(w, r)
	if !ok {
		return
	}

	ctx := r.Context()

	instances, err := rs.TimetableData.ListScheduledInstances(ctx, from, to)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap(
			"load instances failed", err))
		return
	}

	// Resolved once per request (not per instance) — the Betreuungsschlüssel
	// setting is tenant-wide, not per-block.
	ratio := rs.childrenPerStaffRatio(ctx)

	careDays, err := rs.resolveCareDays(ctx, instances, from, to)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap(
			"resolve care days failed", err))
		return
	}

	enriched, conflictInputs, err := rs.enrichInstances(ctx, instances, ratio, careDays)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap(
			"enrich instances failed", err))
		return
	}

	// Window-wide person double-bookings (#2139): the banner's "diese Woche" /
	// "diesen Monat" claim holds because detection covers exactly the
	// requested window, not just today. The rows were already loaded for
	// enrichment, so this adds no queries.
	applyWindowConflicts(enriched, rs.detectWindowConflicts(conflictInputs))

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

// parseInstanceListWindow reads the from/to window of GET /instances. It
// returns ok=false after rendering the 400 for a missing, malformed,
// reversed or too wide window.
func parseInstanceListWindow(w http.ResponseWriter, r *http.Request) (from, to calendar.Date, ok bool) {
	q := r.URL.Query()
	fromStr := q.Get("from")
	toStr := q.Get("to")
	if fromStr == "" || toStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("from and to query params are required (YYYY-MM-DD)")))
		return from, to, false
	}

	from, err := berlinDate(fromStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("invalid from format, expected YYYY-MM-DD")))
		return from, to, false
	}
	to, err = berlinDate(toStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("invalid to format, expected YYYY-MM-DD")))
		return from, to, false
	}

	if to.Before(from) {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("'to' must be on or after 'from'")))
		return from, to, false
	}
	if inclusiveDayCount(from, to) > maxInstanceListRangeDays {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			fmt.Errorf("date range exceeds maximum of %d days", maxInstanceListRangeDays)))
		return from, to, false
	}
	return from, to, true
}

// applyWindowConflicts attaches each instance's window conflicts.
func applyWindowConflicts(enriched []enrichedInstance, conflictsByInstance map[int64][]timetable.InstanceConflictWarning) {
	for i := range enriched {
		if warnings, ok := conflictsByInstance[enriched[i].ID]; ok {
			enriched[i].ConflictWarnings = warnings
		}
	}
}

// enrichWrittenInstance re-reads a block the create/update paths just wrote
// and projects it like one row of the list, with its day's conflicts.
//
// A failed care-day derivation is returned, never swallowed (#1747 review).
// An empty map does not mean "no verdict yet" to the reader — it reads as
// unknown, which is the verdict that puts every assigned child back into
// "Erwartet". Degrading to it would answer a successful write with counts that
// silently contradict the planner the very next reload corrects, and the
// caller cannot tell the two apart. Both call sites already have a path for
// "the write committed, the enrichment did not" and route this into it.
func (rs *Resource) enrichWrittenInstance(ctx context.Context, instanceID int64) (enrichedInstance, error) {
	if rs.TimetableData == nil {
		return enrichedInstance{}, errors.New("timetable resource not fully wired")
	}
	inst, err := rs.TimetableData.FindScheduledInstance(ctx, instanceID)
	if err != nil {
		return enrichedInstance{}, fmt.Errorf("load instance %d: %w", instanceID, err)
	}
	careDays, err := rs.resolveCareDays(ctx, []timetable.ScheduledInstance{inst}, inst.Date, inst.Date)
	if err != nil {
		return enrichedInstance{}, fmt.Errorf("resolve care days for instance %d: %w", inst.ID, err)
	}
	rows, err := rs.TimetableData.ListScheduledInstanceRows(ctx, []timetable.ScheduledInstance{inst})
	if err != nil {
		return enrichedInstance{}, fmt.Errorf("load instance rows: %w", err)
	}
	enriched, _, _, err := rs.enrichInstance(ctx, inst, rows, make(map[int64]string), make(map[int64]templateMeta), make(map[int64]*timetable.PlanningTrack), make(map[int64]timetable.EmptyOfferingRosterExplainer), rs.childrenPerStaffRatio(ctx), careDays)
	if err != nil {
		return enrichedInstance{}, err
	}
	enriched.ConflictWarnings = rs.dayConflictWarningsFor(ctx, inst)
	return enriched, nil
}

// enrichInstances projects a list window: the per-instance rows come from one
// batched read per kind (#2940), the caches are shared across the window.
func (rs *Resource) enrichInstances(
	ctx context.Context,
	instances []timetable.ScheduledInstance,
	childrenPerStaffRatio int,
	careDays map[int64]map[calendar.Date]careplan.CareDayStatus,
) ([]enrichedInstance, []timetable.WindowConflictBlock, error) {
	rows, err := rs.TimetableData.ListScheduledInstanceRows(ctx, instances)
	if err != nil {
		return nil, nil, fmt.Errorf("load instance rows: %w", err)
	}
	// Cache room and activity-group lookups for the request. ~5-8 unique
	// rooms and templates per week — caching turns 30 lookups into ~10.
	roomCache := make(map[int64]string)
	metaCache := make(map[int64]templateMeta)
	planningTrackCache := make(map[int64]*timetable.PlanningTrack)
	offeringSourceCache := make(map[int64]timetable.EmptyOfferingRosterExplainer)
	enriched := make([]enrichedInstance, 0, len(instances))
	conflictInputs := make([]timetable.WindowConflictBlock, 0, len(instances))
	for _, inst := range instances {
		item, staffRows, studentRows, err := rs.enrichInstance(ctx, inst, rows, roomCache, metaCache, planningTrackCache, offeringSourceCache, childrenPerStaffRatio, careDays)
		if err != nil {
			return nil, nil, err
		}
		enriched = append(enriched, item)
		conflictInputs = append(conflictInputs, windowConflictBlock(inst, staffRows, studentRows))
	}
	return enriched, conflictInputs, nil
}

// windowConflictBlock maps one listed block and its rows onto the input of
// the Timetable owner's window conflict detection (#2139).
func windowConflictBlock(inst timetable.ScheduledInstance, staffRows []timetable.InstanceStaff, studentRows []timetable.ScheduledParticipant) timetable.WindowConflictBlock {
	block := timetable.WindowConflictBlock{
		InstanceID: inst.ID,
		Date:       inst.Date,
		Title:      inst.Title,
		StartTime:  inst.StartTime,
		EndTime:    inst.EndTime,
		RoomID:     inst.RoomID,
		Status:     inst.Status,
		Staff:      make([]timetable.WindowConflictStaff, 0, len(staffRows)),
		Students:   make([]timetable.WindowConflictStudent, 0, len(studentRows)),
	}
	for _, row := range staffRows {
		block.Staff = append(block.Staff, timetable.WindowConflictStaff{StaffID: row.StaffID, RoomID: row.RoomID, IsAbsent: row.IsAbsent})
	}
	for _, row := range studentRows {
		block.Students = append(block.Students, timetable.WindowConflictStudent{StudentID: row.StudentID, Status: row.Status})
	}
	return block
}

// detectWindowConflicts asks the Timetable owner for the window's person
// double-bookings. Conflicts are advisory: an unwired detection yields none.
func (rs *Resource) detectWindowConflicts(blocks []timetable.WindowConflictBlock) map[int64][]timetable.InstanceConflictWarning {
	if rs.ConflictDetection == nil {
		return map[int64][]timetable.InstanceConflictWarning{}
	}
	return rs.ConflictDetection.DetectWindowConflicts(blocks)
}

// enrichInstance loads room name, activity-group type, staff list, and
// student counts for a single instance. Room and type lookups consult the
// per-request caches to avoid duplicate queries when many instances share a
// template (e.g. the daily Mensa).
// enrichInstance additionally returns the raw staff and student rows it
// loaded so the caller can feed them into the window-wide conflict detection
// (#2139) without a second round of queries.
func (rs *Resource) enrichInstance(
	ctx context.Context,
	inst timetable.ScheduledInstance,
	rows *timetable.ScheduledInstanceRows,
	roomCache map[int64]string,
	metaCache map[int64]templateMeta,
	planningTrackCache map[int64]*timetable.PlanningTrack,
	offeringSourceCache map[int64]timetable.EmptyOfferingRosterExplainer,
	childrenPerStaffRatio int,
	careDays map[int64]map[calendar.Date]careplan.CareDayStatus,
) (enrichedInstance, []timetable.InstanceStaff, []timetable.ScheduledParticipant, error) {
	roomName := rs.lookupRoomName(ctx, inst.RoomID, roomCache)
	meta := rs.lookupTemplateMeta(ctx, inst.ActivityGroupID, metaCache, planningTrackCache)

	staffRows := rows.Staff[inst.ID]
	staff, absentCount := summarizeInstanceStaff(staffRows)

	studentRows := rows.Participants[inst.ID]
	attendance := summarizeInstanceStudents(inst, studentRows, careDays, rows.Cutoffs[inst.Date])
	emptyRosterReason := rs.resolveEmptyRosterReason(ctx, inst, meta, studentRows, offeringSourceCache)

	availability, err := rs.completionAvailability(ctx, inst)
	if err != nil {
		return enrichedInstance{}, nil, nil, err
	}

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
		IsLive:                 inst.Status == timetable.InstanceStatusActive && inst.ActiveGroupID != nil,
		ActivityGroupID:        inst.ActivityGroupID,
		CalendarPeriodID:       inst.CalendarPeriodID,
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
		EmptyRosterReason:      emptyRosterReason,
		NotScheduledCount:      attendance.notScheduled,
		RequiredStaffCount:     timetable.EffectiveRequiredStaff(instanceRequiredStaffOverride(inst.RequiredStaff, meta.requiredStaff), attendance.expected+attendance.present, childrenPerStaffRatio),
		AssignedStaffCount:     len(staffRows) - absentCount,
		RequiredStaffOverride:  inst.RequiredStaff,
		ConflictWarnings:       []timetable.InstanceConflictWarning{},
		CanReopen:              reopenEligibility(ctx, inst, studentRows),
		CanComplete:            availability.CanComplete,
		CompleteAvailableAt:    availability.CompleteAvailableAt.Format(time.RFC3339),
	}
	return item, staffRows, studentRows, nil
}

// completionAvailability evaluates when the block may be completed under the
// tenant's planned-end policy.
func (rs *Resource) completionAvailability(ctx context.Context, inst timetable.ScheduledInstance) (timetable.LifecycleAvailability, error) {
	enforcePlannedEnd, err := rs.enforcePlannedEnd(ctx)
	if err != nil {
		return timetable.LifecycleAvailability{}, err
	}
	return timetable.EvaluateLifecycleAvailability(
		timetable.LifecycleWindow{Date: inst.Date, StartTime: inst.StartTime, EndTime: inst.EndTime, IsSpontaneous: inst.IsSpontaneous}, time.Now(), 0, enforcePlannedEnd,
	), nil
}

func reopenEligibility(ctx context.Context, inst timetable.ScheduledInstance, attendance []timetable.ScheduledParticipant) bool {
	claims := jwt.ClaimsFromCtx(ctx)
	return timetable.CanReopenInstance(inst, int64(claims.ID), common.HasEffectiveAdminScope(ctx), time.Now()) &&
		timetable.AttendanceUnchangedSinceCompletion(inst, attendance)
}

// dayConflictWarningsFor computes the #2139 window conflicts for ONE instance
// against every other planned/active instance of its day. Used by the
// create/update paths that re-enrich a single row; the list path batches the
// same detection over its whole window instead. Degrades to an empty slice on
// load errors — conflicts are advisory and must never fail a committed write.
func (rs *Resource) dayConflictWarningsFor(
	ctx context.Context,
	inst timetable.ScheduledInstance,
) []timetable.InstanceConflictWarning {
	empty := []timetable.InstanceConflictWarning{}
	if rs.TimetableData == nil || rs.ConflictDetection == nil {
		return empty
	}
	dayInstances, err := rs.TimetableData.ListScheduledInstances(ctx, inst.Date, inst.Date)
	if err != nil {
		rs.getLogger().Warn("day conflict detection: load day instances failed",
			slog.Int64("instance_id", inst.ID),
			slog.String("date", inst.Date.String()),
			slog.String("error", err.Error()),
		)
		return empty
	}
	rows, err := rs.TimetableData.ListScheduledInstanceRows(ctx, dayInstances)
	if err != nil {
		rs.getLogger().Warn("day conflict detection: load instance rows failed",
			slog.Int64("instance_id", inst.ID),
			slog.String("error", err.Error()),
		)
		return empty
	}
	inputs := make([]timetable.WindowConflictBlock, 0, len(dayInstances))
	for _, dayInst := range dayInstances {
		inputs = append(inputs, windowConflictBlock(dayInst, rows.Staff[dayInst.ID], rows.Participants[dayInst.ID]))
	}
	if warnings, ok := rs.ConflictDetection.DetectWindowConflicts(inputs)[inst.ID]; ok {
		return warnings
	}
	return empty
}

// lookupRoomName resolves a room id to its display name, with per-request
// memoisation. Returns an empty string if the owner is unwired or the lookup
// fails — the planner shows "Raum #ID" in that case so the user is not blocked.
func (rs *Resource) lookupRoomName(ctx context.Context, roomID int64, cache map[int64]string) string {
	if name, ok := cache[roomID]; ok {
		return name
	}
	if rs.TimetableData == nil {
		cache[roomID] = ""
		return ""
	}
	name, ok, err := rs.TimetableData.BlockRoomName(ctx, roomID)
	if err != nil || !ok {
		// Logged at debug only — a missing room reference here is recoverable.
		rs.getLogger().Debug("instance list: room lookup failed",
			slog.Int64("room_id", roomID),
		)
		cache[roomID] = ""
		return ""
	}
	cache[roomID] = name
	return name
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
	seriesNotes           *string
	sourceCareOfferingIDs []int64
}

func (rs *Resource) lookupTemplateMeta(
	ctx context.Context,
	activityGroupID *int64,
	cache map[int64]templateMeta,
	planningTrackCache map[int64]*timetable.PlanningTrack,
) templateMeta {
	fallback := templateMeta{activityType: timetable.GroupTypeActivity}
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
	group, err := rs.TimetableData.FindBlockTemplate(ctx, *activityGroupID)
	if err != nil {
		rs.getLogger().Debug("instance list: activity group lookup failed",
			slog.Int64("activity_group_id", *activityGroupID),
		)
		cache[*activityGroupID] = fallback
		return fallback
	}
	meta := templateMeta{
		activityType:          group.Type,
		requiredStaff:         group.RequiredStaff,
		seriesNotes:           group.Notes,
		planningTrackID:       group.PlanningTrackID,
		sourceCareOfferingIDs: append([]int64(nil), group.SourceCareOfferingIDs...),
	}
	if group.PlanningTrackID != nil {
		if track := rs.lookupPlanningTrack(ctx, *group.PlanningTrackID, planningTrackCache); track != nil {
			meta.planningTrackName = track.Name
			meta.planningTrackColor = track.Color
			sortOrder := track.SortOrder
			meta.planningTrackSortOrder = &sortOrder
		}
	}
	cache[*activityGroupID] = meta
	return meta
}

// lookupPlanningTrack resolves a template's planning track with per-request
// memoisation; nil when the administration is unwired or the lookup fails.
func (rs *Resource) lookupPlanningTrack(ctx context.Context, trackID int64, cache map[int64]*timetable.PlanningTrack) *timetable.PlanningTrack {
	if rs.PlanningTracks == nil {
		return nil
	}
	if track, cached := cache[trackID]; cached {
		return track
	}
	var track *timetable.PlanningTrack
	found, err := rs.PlanningTracks.GetPlanningTrack(ctx, trackID)
	if err == nil {
		track = &found
	} else {
		rs.getLogger().Debug("instance list: planning track lookup failed",
			slog.Int64("planning_track_id", trackID),
		)
	}
	cache[trackID] = track
	return track
}

func (rs *Resource) resolveEmptyRosterReason(
	ctx context.Context,
	inst timetable.ScheduledInstance,
	meta templateMeta,
	studentRows []timetable.ScheduledParticipant,
	cache map[int64]timetable.EmptyOfferingRosterExplainer,
) *emptyRosterReason {
	if len(studentRows) > 0 || len(meta.sourceCareOfferingIDs) == 0 || rs.OfferingSourceOptions == nil {
		return nil
	}
	periodKey := int64(0)
	if inst.CalendarPeriodID != nil {
		periodKey = *inst.CalendarPeriodID
	}
	explain, cached := cache[periodKey]
	if !cached {
		var err error
		explain, err = rs.OfferingSourceOptions.EmptyRosterExplainer(ctx, inst.CalendarPeriodID)
		if err != nil {
			rs.getLogger().Warn("instance list: offering-source explanation failed",
				slog.Int64("instance_id", inst.ID),
				slog.String("error", err.Error()),
			)
			return nil
		}
		cache[periodKey] = explain
	}
	explanation := explain(meta.sourceCareOfferingIDs, inst.Date)
	if explanation == nil {
		return nil
	}
	reason := &emptyRosterReason{Kind: explanation.Kind, PhaseName: explanation.PhaseName}
	if !explanation.ServiceStartDate.IsZero() {
		reason.ServiceStartDate = explanation.ServiceStartDate.String()
	}
	return reason
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
