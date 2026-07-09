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
	ID                    int64                                 `json:"id"`
	Date                  string                                `json:"date"`
	StartTime             string                                `json:"start_time"`
	EndTime               string                                `json:"end_time"`
	Title                 string                                `json:"title"`
	Description           *string                               `json:"description,omitempty"`
	Notes                 *string                               `json:"notes,omitempty"`
	Status                string                                `json:"status"`
	IsSpontaneous         bool                                  `json:"is_spontaneous"`
	IsLive                bool                                  `json:"is_live"`
	ActivityGroupID       *int64                                `json:"activity_group_id,omitempty"`
	ActivityType          string                                `json:"activity_type"`
	RoomID                int64                                 `json:"room_id"`
	RoomName              string                                `json:"room_name"`
	Staff                 []instanceStaffSummary                `json:"staff"`
	StudentIDs            []int64                               `json:"student_ids"`
	Students              []instanceStudentSummary              `json:"students"`
	StaffCount            int                                   `json:"staff_count"`
	AbsentStaffCount      int                                   `json:"absent_staff_count"`
	UnderstaffedAck       bool                                  `json:"understaffed_ack"`
	UnderstaffedNote      *string                               `json:"understaffed_note,omitempty"`
	CancelReason          *string                               `json:"cancel_reason,omitempty"`
	ExpectedStudentsCount int                                   `json:"expected_students_count"`
	PresentStudentsCount  int                                   `json:"present_students_count"`
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
	typeCache := make(map[int64]string)

	enriched := make([]enrichedInstance, 0, len(instances))
	for _, inst := range instances {
		item, err := rs.enrichInstance(ctx, inst, roomCache, typeCache)
		if err != nil {
			common.RenderError(w, r, common.ErrorInternalServerWrap(
				"enrich instance failed", err))
			return
		}
		enriched = append(enriched, item)
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
func (rs *Resource) enrichInstance(
	ctx context.Context,
	inst *scheduleModel.ActivityInstance,
	roomCache map[int64]string,
	typeCache map[int64]string,
) (enrichedInstance, error) {
	if inst == nil {
		return enrichedInstance{}, errors.New("nil instance")
	}

	roomName := rs.lookupRoomName(ctx, inst.RoomID, roomCache)
	activityType := rs.lookupActivityType(ctx, inst.ActivityGroupID, typeCache)

	staffRows, err := rs.TimetableData.GetInstanceStaff(ctx, inst.ID)
	if err != nil {
		return enrichedInstance{}, fmt.Errorf("load staff for instance %d: %w", inst.ID, err)
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
		return enrichedInstance{}, fmt.Errorf("load students for instance %d: %w", inst.ID, err)
	}
	expected := 0
	present := 0
	studentIDs := make([]int64, 0, len(studentRows))
	students := make([]instanceStudentSummary, 0, len(studentRows))
	for _, row := range studentRows {
		studentIDs = append(studentIDs, row.StudentID)
		var checkedInAt *string
		if row.CheckedInAt != nil {
			formatted := row.CheckedInAt.UTC().Format("2006-01-02T15:04:05Z07:00")
			checkedInAt = &formatted
		}
		students = append(students, instanceStudentSummary{
			StudentID:   row.StudentID,
			Status:      row.Status,
			Substatus:   row.Substatus,
			Note:        row.Note,
			CheckedInAt: checkedInAt,
		})
		switch row.Status {
		case scheduleModel.AttendanceStatusExpected:
			expected++
		case scheduleModel.AttendanceStatusPresent:
			present++
		}
	}

	return enrichedInstance{
		ID:                    inst.ID,
		Date:                  inst.Date.Format(dateLayout),
		StartTime:             inst.StartTime.Format("15:04"),
		EndTime:               inst.EndTime.Format("15:04"),
		Title:                 inst.Title,
		Description:           inst.Description,
		Notes:                 inst.Notes,
		Status:                inst.Status,
		IsSpontaneous:         inst.IsSpontaneous,
		IsLive:                inst.Status == scheduleModel.InstanceStatusActive && inst.ActiveGroupID != nil,
		ActivityGroupID:       inst.ActivityGroupID,
		ActivityType:          activityType,
		RoomID:                inst.RoomID,
		RoomName:              roomName,
		Staff:                 staff,
		StudentIDs:            studentIDs,
		Students:              students,
		StaffCount:            len(staffRows),
		AbsentStaffCount:      absentCount,
		UnderstaffedAck:       inst.UnderstaffedAck,
		UnderstaffedNote:      inst.UnderstaffedNote,
		CancelReason:          inst.CancelReason,
		ExpectedStudentsCount: expected,
		PresentStudentsCount:  present,
		ConflictWarnings:      rs.instanceConflictWarnings(ctx, inst),
	}, nil
}

func (rs *Resource) instanceConflictWarnings(
	ctx context.Context,
	inst *scheduleModel.ActivityInstance,
) []scheduleSvc.InstanceConflictWarning {
	if inst == nil || inst.Status != scheduleModel.InstanceStatusPlanned {
		return []scheduleSvc.InstanceConflictWarning{}
	}
	if inst.Date != timezone.TodayDate() {
		return []scheduleSvc.InstanceConflictWarning{}
	}
	if rs.TimetableData == nil {
		return []scheduleSvc.InstanceConflictWarning{}
	}
	return rs.TimetableData.DetectInstanceStartConflicts(ctx, inst, rs.getLogger())
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
func (rs *Resource) lookupActivityType(ctx context.Context, activityGroupID *int64, cache map[int64]string) string {
	if activityGroupID == nil {
		return activitiesModel.GroupTypeActivity
	}
	if t, ok := cache[*activityGroupID]; ok {
		return t
	}
	if rs.TimetableData == nil {
		cache[*activityGroupID] = activitiesModel.GroupTypeActivity
		return activitiesModel.GroupTypeActivity
	}
	group, err := rs.TimetableData.GetActivityGroup(ctx, *activityGroupID)
	if err != nil || group == nil {
		rs.getLogger().Debug("instance list: activity group lookup failed",
			slog.Int64("activity_group_id", *activityGroupID),
		)
		cache[*activityGroupID] = activitiesModel.GroupTypeActivity
		return activitiesModel.GroupTypeActivity
	}
	cache[*activityGroupID] = group.Type
	return group.Type
}
