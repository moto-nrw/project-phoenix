// Package staffshifts exposes admin CRUD for planned per-date staff shifts
// (Dienstplan, #1376 core slice). Shifts carry the concrete wall-clock times
// the auto-checkout job (#1798) closes forgotten work sessions against.
// Staff members read their own shifts via /api/time-tracking/shifts.
package staffshifts

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

// Resource bundles the dependencies for the staff-shift HTTP handlers.
type Resource struct {
	Service       scheduleSvc.StaffShiftService
	SeriesService scheduleSvc.StaffShiftSeriesService
	Overview      scheduleSvc.StaffScheduleOverviewGetter
	PersonService usersSvc.PersonService
	db            *bun.DB
	logger        *slog.Logger
}

// NewResource wires the dependencies.
func NewResource(service scheduleSvc.StaffShiftService, seriesService scheduleSvc.StaffShiftSeriesService, overview scheduleSvc.StaffScheduleOverviewGetter, personService usersSvc.PersonService, db *bun.DB, logger *slog.Logger) *Resource {
	return &Resource{Service: service, SeriesService: seriesService, Overview: overview, PersonService: personService, db: db, logger: logger}
}

// Router returns the chi sub-router for /api/staff-shifts.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {

		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Get("/", rs.list)
		r.With(
			authorize.RequiresPermission(permissions.TimeTrackingManage),
			authorize.RequiresPermission(permissions.SchedulesRead),
			authorize.RequiresPermission(permissions.UsersRead),
			withTx,
		).Get("/overview", rs.overview)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Post("/", rs.create)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Put("/{id}", rs.update)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Delete("/{id}", rs.delete)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Post("/series", rs.createSeries)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Put("/series/{id}/split", rs.splitSeries)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Delete("/series/{id}", rs.endSeries)
	})

	return r
}

// ShiftRequest is the create/update payload. Times are "HH:MM" wall-clock
// strings, the date is "YYYY-MM-DD". StaffID is ignored on update (the shift
// stays with its staff member).
type ShiftRequest struct {
	StaffID      int64      `json:"staff_id"`
	Date         string     `json:"date"`
	StartTime    string     `json:"start_time"`
	EndTime      string     `json:"end_time"`
	BreakMinutes int        `json:"break_minutes"`
	ShiftTypeID  optionalID `json:"shift_type_id"`
	Notes        *string    `json:"notes"`
	// Cancelled marks a shift that does not take place (staff absent / gap left
	// open, #1841). Defaults to false when omitted.
	Cancelled bool `json:"cancelled"`
	// ChangeReason is the optional "why" for a flexible daily change; omitted
	// (nil) preserves the stored reason on update, explicit null clears it.
	ChangeReason *string `json:"change_reason"`
	// OriginShiftID marks this shift as a replacement covering another shift
	// (#1841). Only honoured on create; a plain edit never re-points it.
	OriginShiftID *int64 `json:"origin_shift_id"`
}

// optionalID captures whether a nullable ID field was present in the JSON
// payload, so an omitted shift_type_id (preserve the existing value on update)
// is distinguishable from an explicit null (clear the type). encoding/json only
// invokes UnmarshalJSON when the key is present, so Present stays false when the
// field is absent — the case a stale client or third-party consumer produces.
type optionalID struct {
	Present bool
	Value   *int64
}

func (o *optionalID) UnmarshalJSON(data []byte) error {
	o.Present = true
	if string(data) == "null" {
		o.Value = nil
		return nil
	}
	var v int64
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	o.Value = &v
	return nil
}

// ShiftResponse is the wire format returned to clients.
type ShiftResponse struct {
	ID            int64   `json:"id"`
	StaffID       int64   `json:"staff_id"`
	Date          string  `json:"date"`
	StartTime     string  `json:"start_time"`
	EndTime       string  `json:"end_time"`
	BreakMinutes  int     `json:"break_minutes"`
	ShiftTypeID   *int64  `json:"shift_type_id,omitempty"`
	Notes         string  `json:"notes,omitempty"`
	SeriesID      *int64  `json:"series_id,omitempty"`
	Detached      bool    `json:"detached"`
	Cancelled     bool    `json:"cancelled"`
	ChangeReason  *string `json:"change_reason,omitempty"`
	OriginShiftID *int64  `json:"origin_shift_id,omitempty"`
}

// ToShiftResponse maps a shift onto the wire format. Exported for the
// time-tracking self endpoint, which serves the same shape.
func ToShiftResponse(s *scheduleModels.StaffShift) ShiftResponse {
	return ShiftResponse{
		ID:            s.ID,
		StaffID:       s.StaffID,
		Date:          s.Date.String(),
		StartTime:     timezone.WallClock(s.StartTime).Format("15:04"),
		EndTime:       timezone.WallClock(s.EndTime).Format("15:04"),
		BreakMinutes:  s.BreakMinutes,
		ShiftTypeID:   s.ShiftTypeID,
		Notes:         s.Notes,
		SeriesID:      s.SeriesID,
		Detached:      s.Detached,
		Cancelled:     s.Cancelled,
		ChangeReason:  s.ChangeReason,
		OriginShiftID: s.OriginShiftID,
	}
}

// ToShiftResponses maps a slice of shifts onto the wire format.
func ToShiftResponses(shifts []*scheduleModels.StaffShift) []ShiftResponse {
	out := make([]ShiftResponse, 0, len(shifts))
	for _, s := range shifts {
		out = append(out, ToShiftResponse(s))
	}
	return out
}

// ParseShiftTimes parses "HH:MM" start/end strings into wall-clock times.
func ParseShiftTimes(startStr, endStr string) (start, end time.Time, err error) {
	start, err = time.Parse("15:04", startStr)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("start_time must be HH:MM")
	}
	end, err = time.Parse("15:04", endStr)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("end_time must be HH:MM")
	}
	return timezone.WallClock(start), timezone.WallClock(end), nil
}

func (rs *Resource) buildShift(req ShiftRequest) (*scheduleModels.StaffShift, error) {
	date, err := timezone.ParseDate(req.Date)
	if err != nil {
		return nil, errors.New("date must be YYYY-MM-DD")
	}
	start, end, err := ParseShiftTimes(req.StartTime, req.EndTime)
	if err != nil {
		return nil, err
	}
	notes := ""
	if req.Notes != nil {
		notes = *req.Notes
	}
	return &scheduleModels.StaffShift{
		StaffID:       req.StaffID,
		Date:          date,
		StartTime:     start,
		EndTime:       end,
		BreakMinutes:  req.BreakMinutes,
		ShiftTypeID:   req.ShiftTypeID.Value,
		Notes:         notes,
		Cancelled:     req.Cancelled,
		ChangeReason:  req.ChangeReason,
		OriginShiftID: req.OriginShiftID,
	}, nil
}

// editorStaffID resolves the acting admin's staff record from the JWT claims.
func (rs *Resource) editorStaffID(ctx context.Context) (int64, error) {
	claims := jwt.ClaimsFromCtx(ctx)
	if claims.ID == 0 {
		return 0, errors.New("invalid token")
	}
	person, err := rs.PersonService.FindByAccountID(ctx, int64(claims.ID))
	if err != nil {
		return 0, errors.New("person not found for account")
	}
	staff, err := rs.PersonService.GetStaffByPersonID(ctx, person.ID)
	if err != nil {
		return 0, errors.New("staff record not found")
	}
	return staff.ID, nil
}

func renderServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, scheduleSvc.ErrShiftOverlap):
		common.RenderError(w, r, common.ErrorConflict(err))
	case errors.Is(err, scheduleSvc.ErrShiftNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, scheduleSvc.ErrShiftRangeTooLarge), errors.Is(err, scheduleSvc.ErrShiftInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, scheduleSvc.ErrShiftTypeNotFound), errors.Is(err, scheduleSvc.ErrShiftTypeInactive):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// parseDateRange extracts "from" and "to" query parameters as calendar dates.
func parseDateRange(w http.ResponseWriter, r *http.Request) (from, to timezone.Date, ok bool) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	if fromStr == "" || toStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("from and to query parameters are required")))
		return timezone.Date{}, timezone.Date{}, false
	}
	from, err := timezone.ParseDate(fromStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid from date format, expected YYYY-MM-DD")))
		return timezone.Date{}, timezone.Date{}, false
	}
	to, err = timezone.ParseDate(toStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid to date format, expected YYYY-MM-DD")))
		return timezone.Date{}, timezone.Date{}, false
	}
	return from, to, true
}

func (rs *Resource) list(w http.ResponseWriter, r *http.Request) {
	from, to, ok := parseDateRange(w, r)
	if !ok {
		return
	}
	shifts, err := rs.Service.ListShifts(r.Context(), from, to)
	if err != nil {
		renderServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, ToShiftResponses(shifts), "Staff shifts retrieved")
}

func (rs *Resource) create(w http.ResponseWriter, r *http.Request) {
	var req ShiftRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	shift, err := rs.buildShift(req)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	editorID, err := rs.editorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	shift.CreatedBy = editorID

	saved, err := rs.Service.CreateShift(r.Context(), shift)
	if err != nil {
		renderServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, ToShiftResponse(saved), "Staff shift created")
}

func (rs *Resource) update(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	var req ShiftRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	shift, err := rs.buildShift(req)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	editorID, err := rs.editorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	shift.ID = id
	shift.UpdatedBy = &editorID

	saved, err := rs.Service.UpdateShiftWithOptions(r.Context(), shift, scheduleSvc.StaffShiftUpdateOptions{
		PreserveExistingNotes:        req.Notes == nil,
		PreserveExistingShiftType:    !req.ShiftTypeID.Present,
		PreserveExistingChangeReason: req.ChangeReason == nil,
	})
	if err != nil {
		renderServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, ToShiftResponse(saved), "Staff shift updated")
}

func (rs *Resource) delete(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.Service.DeleteShift(r.Context(), id); err != nil {
		renderServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]any{"id": id}, "Staff shift deleted")
}
