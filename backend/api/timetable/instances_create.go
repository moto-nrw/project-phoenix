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
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
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
	ListKind        *string `json:"list_kind,omitempty"`
	StaffIDs        []int64 `json:"staff_ids,omitempty"`
	StudentIDs      []int64 `json:"student_ids,omitempty"`
	// RequiredStaff is the optional per-occurrence Personalbedarf pin (#1839);
	// omitted/null = no pin (inherit the template override, else derive from
	// the Betreuungsschlüssel), a value = pin for this occurrence.
	RequiredStaff *int `json:"required_staff,omitempty"`
}

// normalizeRequiredStaff cleans the optional required_staff override into a
// pointer: nil or a negative value -> nil ("no override, derive from the
// Betreuungsschlüssel"); a non-negative value is a manual override. Shared by
// the instance + template create/update handlers (#1839).
func normalizeRequiredStaff(v *int) *int {
	if v == nil || *v < 0 {
		return nil
	}
	return v
}

// normalizeNotes trims an optional note into a pointer: nil or a
// whitespace-only value becomes nil (no note); otherwise the trimmed text is
// returned. Shared by the timetable note fields (Tagesnotiz + Wochennotiz).
func normalizeNotes(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

type parsedCreateInstanceRequest struct {
	req       *createInstanceRequest
	date      timezone.Date
	startTime time.Time
	endTime   time.Time
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

func normalizeInstanceListKind(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	kind := strings.TrimSpace(*raw)
	if kind == "" {
		return nil, nil
	}
	if !activitiesModel.IsValidListKind(kind) {
		return nil, fmt.Errorf("invalid list_kind %q", kind)
	}
	return &kind, nil
}

func bindCreateInstanceRequest(w http.ResponseWriter, r *http.Request) (*parsedCreateInstanceRequest, bool) {
	req := &createInstanceRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return nil, false
	}

	date, err := berlinDate(req.Date)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("invalid date format, expected YYYY-MM-DD")))
		return nil, false
	}
	if err := validateTimetableWorkday(date); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return nil, false
	}
	startTime, err := parseClockTime(req.StartTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("invalid start_time format, expected HH:MM")))
		return nil, false
	}
	endTime, err := parseClockTime(req.EndTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("invalid end_time format, expected HH:MM")))
		return nil, false
	}
	if !endTime.After(startTime) {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("end_time must be after start_time")))
		return nil, false
	}

	return &parsedCreateInstanceRequest{
		req:       req,
		date:      date,
		startTime: startTime,
		endTime:   endTime,
	}, true
}

// createInstance handles POST /api/timetable/instances.
func (rs *Resource) createInstance(w http.ResponseWriter, r *http.Request) {
	if rs.InstanceService == nil || rs.TimetableData == nil {
		common.RenderError(w, r, common.ErrorInternalServer(
			errors.New("timetable resource not fully wired")))
		return
	}

	parsed, ok := bindCreateInstanceRequest(w, r)
	if !ok {
		return
	}
	req := parsed.req
	listKind, err := normalizeInstanceListKind(req.ListKind)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	createdBy := rs.resolveStartedByStaffID(r.Context())
	var createdByPtr *int64
	if createdBy > 0 {
		createdByCopy := createdBy
		createdByPtr = &createdByCopy
	}

	inst, err := rs.InstanceService.Create(r.Context(), scheduleSvc.CreateInstanceInput{
		Date:             parsed.date,
		StartTime:        parsed.startTime,
		EndTime:          parsed.endTime,
		Title:            req.Title,
		Description:      req.Description,
		Notes:            req.Notes,
		RoomID:           req.RoomID,
		ActivityGroupID:  req.ActivityGroupID,
		ListKind:         listKind,
		StaffIDs:         req.StaffIDs,
		StudentIDs:       req.StudentIDs,
		CreatedByStaffID: createdByPtr,
		RequiredStaff:    normalizeRequiredStaff(req.RequiredStaff),
	})
	if err != nil {
		renderCreateInstanceError(w, r, err)
		return
	}

	// Re-fetch as enriched payload so the frontend can drop the fresh row
	// straight into its SWR cache without a round-trip refetch.
	roomCache := make(map[int64]string)
	typeCache := make(map[int64]templateMeta)
	// A failed care-day derivation takes the same route as any other failed
	// enrichment: the payload is announced as incomplete instead of carrying
	// counts that quietly read every assigned child as expected again (#1747
	// review).
	var enriched enrichedInstance
	careDays, err := rs.careDaysForInstance(r.Context(), inst)
	if err == nil {
		enriched, _, _, err = rs.enrichInstance(r.Context(), inst, roomCache, typeCache, make(map[int64]*scheduleModel.PlanningTrack), make(map[int64][]enrollmentSvc.OfferingSourceOption), rs.childrenPerStaffRatio(r.Context()), careDays)
	}
	if err == nil {
		enriched.ConflictWarnings = rs.dayConflictWarningsFor(r.Context(), inst)
	}
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

func renderCreateInstanceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, scheduleSvc.ErrInvalidInstanceReference):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case base.IsUniqueViolationOn(err, "idx_activity_instances_template_unique"):
		common.RenderError(w, r, common.ErrorConflictWithCode(
			errors.New("instance already exists for this template/date/start_time"),
			"duplicate_instance",
		))
	default:
		common.RenderError(w, r, common.ErrorInternalServerWrap(
			"create instance failed", err))
	}
}
