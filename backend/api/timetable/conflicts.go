// Package timetable — planning-time conflict probe.
//
//	GET /api/timetable/conflicts?date=YYYY-MM-DD&start_time=HH:MM&end_time=HH:MM
//	    [&room_id=N][&staff_ids=1,2][&student_ids=3,4][&exclude_instance_id=N]
//
// Checks a hypothetical slot against the day's planned/active instances and
// returns advisory warnings (room double-booking, staff double-planning,
// student double-planning). Read-only and never blocking: the response is
// always 200 with a (possibly empty) warnings list. 400 only when date /
// start_time / end_time are missing or invalid, or when no resource
// parameter (room_id, staff_ids, student_ids) is given at all.
//
// Permission: SchedulesRead (information view, mirrors /exception-conflicts).
package timetable

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// PlannedConflictsResponse is the 200 body for GET /conflicts.
type PlannedConflictsResponse struct {
	Date      string                               `json:"date"`
	StartTime string                               `json:"start_time"`
	EndTime   string                               `json:"end_time"`
	Warnings  []scheduleSvc.PlannedConflictWarning `json:"warnings"`
}

// plannedConflictParams is the parsed query string of GET /conflicts.
type plannedConflictParams struct {
	date              timezone.Date
	startTime         time.Time
	endTime           time.Time
	roomID            *int64
	staffIDs          []int64
	studentIDs        []int64
	excludeInstanceID *int64
}

// getPlannedConflicts handles GET /api/timetable/conflicts.
func (rs *Resource) getPlannedConflicts(w http.ResponseWriter, r *http.Request) {
	if rs.TimetableData == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}

	params, ok := parsePlannedConflictParams(w, r)
	if !ok {
		return
	}

	warnings := rs.TimetableData.DetectPlannedConflicts(r.Context(), scheduleSvc.PlannedConflictQuery{
		Date:              params.date,
		StartTime:         params.startTime,
		EndTime:           params.endTime,
		RoomID:            params.roomID,
		StaffIDs:          params.staffIDs,
		StudentIDs:        params.studentIDs,
		ExcludeInstanceID: params.excludeInstanceID,
	}, rs.getLogger())

	rs.getLogger().Info("planned conflicts probed",
		slog.String("date", params.date.String()),
		slog.Int("warning_count", len(warnings)),
	)

	common.Respond(w, r, http.StatusOK, PlannedConflictsResponse{
		Date:      params.date.String(),
		StartTime: params.startTime.Format("15:04"),
		EndTime:   params.endTime.Format("15:04"),
		Warnings:  warnings,
	}, "Planned conflicts retrieved")
}

// parsePlannedConflictParams validates the query string. Renders a 400 and
// returns ok=false on the first violation; the 400 surface is exactly:
// missing/invalid date, start_time, end_time (including end <= start),
// malformed numeric resource params, or no resource param at all.
func parsePlannedConflictParams(w http.ResponseWriter, r *http.Request) (plannedConflictParams, bool) {
	q := r.URL.Query()
	var out plannedConflictParams

	dateStr := q.Get("date")
	if dateStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("date is required")))
		return out, false
	}
	date, err := berlinDate(dateStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid date format, expected YYYY-MM-DD")))
		return out, false
	}

	startStr := q.Get("start_time")
	if startStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("start_time is required (HH:MM)")))
		return out, false
	}
	startTime, err := parseClockTime(startStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid start_time format, expected HH:MM")))
		return out, false
	}

	endStr := q.Get("end_time")
	if endStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("end_time is required (HH:MM)")))
		return out, false
	}
	endTime, err := parseClockTime(endStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid end_time format, expected HH:MM")))
		return out, false
	}
	if !endTime.After(startTime) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("end_time must be after start_time")))
		return out, false
	}

	roomID, ok := parseOptionalQueryID(w, r, q.Get("room_id"), "room_id")
	if !ok {
		return out, false
	}
	excludeID, ok := parseOptionalQueryID(w, r, q.Get("exclude_instance_id"), "exclude_instance_id")
	if !ok {
		return out, false
	}
	staffIDs, ok := parseQueryIDList(w, r, q.Get("staff_ids"), "staff_ids")
	if !ok {
		return out, false
	}
	studentIDs, ok := parseQueryIDList(w, r, q.Get("student_ids"), "student_ids")
	if !ok {
		return out, false
	}

	if roomID == nil && len(staffIDs) == 0 && len(studentIDs) == 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("at least one of room_id, staff_ids, student_ids is required")))
		return out, false
	}

	out.date = date
	out.startTime = startTime
	out.endTime = endTime
	out.roomID = roomID
	out.staffIDs = staffIDs
	out.studentIDs = studentIDs
	out.excludeInstanceID = excludeID
	return out, true
}

// parseOptionalQueryID parses an optional positive int64 query value. Empty
// input yields (nil, true); a malformed or non-positive value renders 400.
func parseOptionalQueryID(w http.ResponseWriter, r *http.Request, raw, name string) (*int64, bool) {
	if raw == "" {
		return nil, true
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid "+name)))
		return nil, false
	}
	return &id, true
}

// parseQueryIDList parses a comma-separated list of positive int64 IDs.
// Empty input yields (nil, true); any malformed or non-positive entry
// renders 400 — silently dropping entries would hide a resource from the
// probe and produce a misleading empty warnings list.
func parseQueryIDList(w http.ResponseWriter, r *http.Request, raw, name string) ([]int64, bool) {
	if raw == "" {
		return nil, true
	}
	parts := strings.Split(raw, ",")
	out := make([]int64, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || id <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid "+name)))
			return nil, false
		}
		out = append(out, id)
	}
	return out, true
}
