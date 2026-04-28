// Package timetable — POST /api/timetable/instances handler.
//
// Creates a planned activity instance outside the materialization flow.
// When activity_group_id is omitted the instance is purely spontaneous
// (is_spontaneous=true server-side); when set it is bound to a template
// for an extra date the materializer would not have emitted. Permission:
// SchedulesManage.
package timetable

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// createInstanceRequest is the JSON body accepted by POST /instances.
//
// Date is YYYY-MM-DD; start_time and end_time are HH:MM. The handler
// converts them into the time.Time shapes the model layer stores
// (date anchored at UTC midnight, times anchored at 2000-01-01 UTC) so
// callers do not have to think about that detail.
type createInstanceRequest struct {
	Date            string  `json:"date"`
	StartTime       string  `json:"start_time"`
	EndTime         string  `json:"end_time"`
	Title           string  `json:"title"`
	Description     *string `json:"description,omitempty"`
	Notes           *string `json:"notes,omitempty"`
	RoomID          int64   `json:"room_id"`
	ActivityGroupID *int64  `json:"activity_group_id,omitempty"`
	StaffIDs        []int64 `json:"staff_ids,omitempty"`
	StudentIDs      []int64 `json:"student_ids,omitempty"`
}

// Bind runs cheap presence checks. Format errors (date / time) bubble up
// from the handler so the user sees a precise message ("invalid start_time
// format, expected HH:MM") instead of a generic 400.
func (req *createInstanceRequest) Bind(_ *http.Request) error {
	if req.Date == "" {
		return errors.New("date is required (YYYY-MM-DD)")
	}
	if req.Title == "" {
		return errors.New("title is required")
	}
	if len(req.Title) > 255 {
		return errors.New("title cannot exceed 255 characters")
	}
	if req.StartTime == "" {
		return errors.New("start_time is required (HH:MM)")
	}
	if req.EndTime == "" {
		return errors.New("end_time is required (HH:MM)")
	}
	if req.RoomID <= 0 {
		return errors.New("room_id is required")
	}
	return nil
}

// parseClockTime turns "HH:MM" into a time.Time anchored at 2000-01-01 UTC.
// Mirrors test/fixtures.go.parseTimeHHMM so created rows look identical
// to seeded ones — the Postgres TIME column only stores the clock part.
func parseClockTime(hhmm string) (time.Time, error) {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(2000, 1, 1, t.Hour(), t.Minute(), 0, 0, time.UTC), nil
}

// createInstance handles POST /api/timetable/instances.
func (rs *Resource) createInstance(w http.ResponseWriter, r *http.Request) {
	if rs.instanceService == nil || rs.activityInstanceRepo == nil ||
		rs.instanceStaffRepo == nil || rs.instanceStudentRepo == nil {
		common.RenderError(w, r, common.ErrorInternalServer(
			errors.New("timetable resource not fully wired")))
		return
	}

	req := &createInstanceRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	date, err := berlinDate(req.Date)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("invalid date format, expected YYYY-MM-DD")))
		return
	}
	startTime, err := parseClockTime(req.StartTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("invalid start_time format, expected HH:MM")))
		return
	}
	endTime, err := parseClockTime(req.EndTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("invalid end_time format, expected HH:MM")))
		return
	}
	if !endTime.After(startTime) {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("end_time must be after start_time")))
		return
	}

	createdBy := rs.resolveStartedByStaffID(r.Context())
	var createdByPtr *int64
	if createdBy > 0 {
		createdByCopy := createdBy
		createdByPtr = &createdByCopy
	}

	inst, err := rs.instanceService.Create(r.Context(), scheduleSvc.CreateInstanceInput{
		Date:             timezone.DateOfUTC(date),
		StartTime:        startTime,
		EndTime:          endTime,
		Title:            req.Title,
		Description:      req.Description,
		Notes:            req.Notes,
		RoomID:           req.RoomID,
		ActivityGroupID:  req.ActivityGroupID,
		StaffIDs:         req.StaffIDs,
		StudentIDs:       req.StudentIDs,
		CreatedByStaffID: createdByPtr,
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap(
			"create instance failed", err))
		return
	}

	// Re-fetch as enriched payload so the frontend can drop the fresh row
	// straight into its SWR cache without a round-trip refetch.
	roomCache := make(map[int64]string)
	typeCache := make(map[int64]string)
	enriched, err := rs.enrichInstance(r.Context(), inst, roomCache, typeCache)
	if err != nil {
		// Insert succeeded; enrichment is informational. Fall through with
		// a partial response rather than failing the whole request.
		common.Respond(w, r, http.StatusCreated, map[string]any{
			"id":             inst.ID,
			"date":           inst.Date.Format(dateLayout),
			"start_time":     inst.StartTime.Format("15:04"),
			"end_time":       inst.EndTime.Format("15:04"),
			"title":          inst.Title,
			"status":         inst.Status,
			"is_spontaneous": inst.IsSpontaneous,
			"room_id":        inst.RoomID,
			"enrich_warning": fmt.Sprintf("enrichment failed: %v", err),
		}, "Instance created (without enrichment)")
		return
	}

	common.Respond(w, r, http.StatusCreated, enriched, "Instance created")
}
