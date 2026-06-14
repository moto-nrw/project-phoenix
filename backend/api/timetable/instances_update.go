package timetable

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

type updateInstanceRequest struct {
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

func (req *updateInstanceRequest) Bind(_ *http.Request) error {
	if req.Date == "" {
		return errors.New("date is required (YYYY-MM-DD)")
	}
	if req.Title == "" {
		return errors.New("title is required")
	}
	if len(req.Title) > 255 {
		return errors.New("title cannot exceed 255 characters")
	}
	if req.StartTime == "" || req.EndTime == "" {
		return errors.New("start_time and end_time are required")
	}
	if req.RoomID <= 0 {
		return errors.New("room_id is required")
	}
	return nil
}

func (rs *Resource) updateInstance(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid instance id")))
		return
	}
	if rs.instanceService == nil || rs.timetableData == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}
	req := &updateInstanceRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	date, err := berlinDate(req.Date)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid date format, expected YYYY-MM-DD")))
		return
	}
	startTime, err := parseClockTime(req.StartTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid start_time format, expected HH:MM")))
		return
	}
	endTime, err := parseClockTime(req.EndTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid end_time format, expected HH:MM")))
		return
	}
	if !endTime.After(startTime) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("end_time must be after start_time")))
		return
	}

	inst, err := rs.instanceService.UpdatePlanned(r.Context(), id, scheduleSvc.UpdateInstanceInput{
		Date:            date,
		StartTime:       startTime,
		EndTime:         endTime,
		Title:           req.Title,
		Description:     req.Description,
		Notes:           req.Notes,
		RoomID:          req.RoomID,
		ActivityGroupID: req.ActivityGroupID,
		StaffIDs:        req.StaffIDs,
		StudentIDs:      req.StudentIDs,
	})
	if err != nil {
		renderInstanceLifecycleError(w, r, err)
		return
	}
	roomCache := make(map[int64]string)
	typeCache := make(map[int64]string)
	enriched, err := rs.enrichInstance(r.Context(), inst, roomCache, typeCache)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("enrich instance failed", err))
		return
	}
	common.Respond(w, r, http.StatusOK, enriched, "Instance updated")
}
