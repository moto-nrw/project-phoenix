package timetable

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
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
	ListKind        *string `json:"list_kind,omitempty"`
	StaffIDs        []int64 `json:"staff_ids,omitempty"`
	StudentIDs      []int64 `json:"student_ids,omitempty"`
	// RequiredStaff is the optional per-occurrence Personalbedarf pin (#1839);
	// omitted/null clears the pin (inherit the template override, else derive
	// from the Betreuungsschlüssel), a value = pin for this occurrence.
	RequiredStaff *int `json:"required_staff,omitempty"`
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
	if rs.InstanceService == nil || rs.TimetableData == nil {
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
	if !rs.legacyInstanceDateIsAllowed(r.Context(), id, date) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("timetable entries can only be scheduled from Monday to Friday")))
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
	listKind, err := normalizeInstanceListKind(req.ListKind)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	inst, err := rs.InstanceService.UpdatePlanned(r.Context(), id, scheduleSvc.UpdateInstanceInput{
		Date:            date,
		StartTime:       startTime,
		EndTime:         endTime,
		Title:           req.Title,
		Description:     req.Description,
		Notes:           req.Notes,
		RoomID:          req.RoomID,
		ActivityGroupID: req.ActivityGroupID,
		ListKind:        listKind,
		StaffIDs:        req.StaffIDs,
		StudentIDs:      req.StudentIDs,
		RequiredStaff:   normalizeRequiredStaff(req.RequiredStaff),
	}, jwt.ActorAccountIDFromCtx(r.Context()))
	if err != nil {
		renderInstanceLifecycleError(w, r, err)
		return
	}
	roomCache := make(map[int64]string)
	typeCache := make(map[int64]templateMeta)
	// A failed care-day derivation fails the response rather than answering with
	// counts that read every assigned child as expected again (#1747 review) —
	// the same 500 the rest of the enrichment already returns here.
	careDays, err := rs.careDaysForInstance(r.Context(), inst)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("resolve care days failed", err))
		return
	}
	enriched, err := rs.enrichInstance(r.Context(), inst, roomCache, typeCache, rs.childrenPerStaffRatio(r.Context()), careDays)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("enrich instance failed", err))
		return
	}
	common.Respond(w, r, http.StatusOK, enriched, "Instance updated")
}

// legacyInstanceDateIsAllowed permits a legacy weekend instance to retain its
// date while preventing PUT from introducing or moving an instance to a
// weekend.
func (rs *Resource) legacyInstanceDateIsAllowed(ctx context.Context, id int64, date timezone.Date) bool {
	if validateTimetableWorkday(date) == nil {
		return true
	}
	existing, err := rs.TimetableData.GetActivityInstance(ctx, id)
	return err == nil && existing.Date == date
}
