// Package staffshifts exposes admin CRUD for planned per-date staff shifts
// (Dienstplan, #1376 core slice). Shifts carry the concrete wall-clock times
// the auto-checkout job (#1798) closes forgotten work sessions against.
// Staff members read their own shifts via /api/time-tracking/shifts.
package staffshifts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
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
	"github.com/moto-nrw/project-phoenix/tenant"
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
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Put("/{id}/move", rs.move)
		r.With(authorize.RequiresPermission(permissions.TimeTrackingManage), withTx).Put("/{id}/cancellation", rs.cancellation)
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
	// open, #1841). Honoured only on create (defaults to false). A plain update
	// always preserves the stored flag and ignores this field — flipping the
	// cancellation state must go through PUT /{id}/cancellation so the
	// replacement set is maintained atomically.
	Cancelled optionalBool `json:"cancelled"`
	// ChangeReason is the optional "why" for a flexible daily change.
	// Presence-aware: an omitted key preserves the stored reason on update, an
	// explicit null clears it, a string replaces it.
	ChangeReason optionalString `json:"change_reason"`
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

// optionalBool captures whether a bool field was present in the JSON payload so
// an omitted cancelled key (preserve the stored flag on update) is
// distinguishable from an explicit false (reactivate). Same presence trick as
// optionalID: UnmarshalJSON only runs when the key is present.
type optionalBool struct {
	Present bool
	Value   bool
}

func (o *optionalBool) UnmarshalJSON(data []byte) error {
	o.Present = true
	// A JSON null must not be read as an explicit false: encoding/json leaves the
	// bool at its zero value (false) for "null" and produces no error, which on
	// the cancellation endpoint would reactivate a shift and delete every
	// replacement for a request that only ever meant to omit the field. Reject it
	// so a malformed/stale null is a 400, never a silent destructive reactivation.
	if string(data) == "null" {
		return errors.New("cancelled must be true or false, not null")
	}
	return json.Unmarshal(data, &o.Value)
}

// optionalInt captures whether an int field was present in the JSON payload so an
// omitted break_minutes is distinguishable from an explicit 0. The cancellation
// endpoint refuses a partial origin edit rather than silently zeroing the stored
// break, so it must know whether the client actually sent the field. Same presence
// trick as optionalID: UnmarshalJSON only runs when the key is present.
type optionalInt struct {
	Present bool
	Value   int
}

func (o *optionalInt) UnmarshalJSON(data []byte) error {
	o.Present = true
	// A JSON null is not a valid break length; reject it rather than reading it as
	// an ambiguous 0 that a partial-payload guard could not distinguish from omitted.
	if string(data) == "null" {
		return errors.New("break_minutes must be a number, not null")
	}
	return json.Unmarshal(data, &o.Value)
}

// optionalString captures whether a nullable string field was present in the
// JSON payload, distinguishing an omitted change_reason (preserve the stored
// value) from an explicit null (clear it) from a string (replace it). A plain
// *string cannot: encoding/json leaves it nil for both omitted and null.
type optionalString struct {
	Present bool
	Value   *string
}

func (o *optionalString) UnmarshalJSON(data []byte) error {
	o.Present = true
	if string(data) == "null" {
		o.Value = nil
		return nil
	}
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	o.Value = &v
	return nil
}

// ShiftResponse is the wire format returned to clients.
type ShiftResponse struct {
	ID           int64  `json:"id"`
	StaffID      int64  `json:"staff_id"`
	Date         string `json:"date"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	BreakMinutes int    `json:"break_minutes"`
	ShiftTypeID  *int64 `json:"shift_type_id,omitempty"`
	// ShiftTypeName/ShiftTypeColor carry the resolved Schichtart so a staff
	// member who cannot read the admin-only /api/shift-types endpoint still sees
	// the label and color on their own shifts (#1844). Present only when the
	// shift has a type and the service resolved it.
	ShiftTypeName  *string `json:"shift_type_name,omitempty"`
	ShiftTypeColor *string `json:"shift_type_color,omitempty"`
	Notes          string  `json:"notes,omitempty"`
	SeriesID       *int64  `json:"series_id,omitempty"`
	Detached       bool    `json:"detached"`
	Cancelled      bool    `json:"cancelled"`
	ChangeReason   *string `json:"change_reason,omitempty"`
	OriginShiftID  *int64  `json:"origin_shift_id,omitempty"`
}

// ToShiftResponse maps a shift onto the wire format. Exported for the
// time-tracking self endpoint, which serves the same shape.
func ToShiftResponse(s *scheduleModels.StaffShift) ShiftResponse {
	resp := ShiftResponse{
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
	if s.ShiftType != nil {
		name := s.ShiftType.Name
		color := s.ShiftType.Color
		resp.ShiftTypeName = &name
		resp.ShiftTypeColor = &color
	}
	return resp
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
		Cancelled:     req.Cancelled.Value,
		ChangeReason:  req.ChangeReason.Value,
		OriginShiftID: req.OriginShiftID,
	}, nil
}

// actorAccountID reads the acting account id straight from the JWT claims for
// the Änderungsprotokoll (#1884). Nil when claims are missing so the audit row
// stores NULL instead of a fabricated id.
func actorAccountID(ctx context.Context) *int64 {
	claims := jwt.ClaimsFromCtx(ctx)
	if claims.ID == 0 {
		return nil
	}
	id := int64(claims.ID)
	return &id
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
	case errors.Is(err, scheduleSvc.ErrShiftOverlap), errors.Is(err, scheduleSvc.ErrShiftConflict):
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
	// Optional staff_id narrows the week grid to one staff member — the admin
	// staff-detail Plan|Ist view needs exactly that person's planned shifts
	// (#1844) rather than the whole tenant's. A filter param, not a second route.
	shifts, err := rs.listShifts(r, from, to)
	if err != nil {
		renderServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, ToShiftResponses(shifts), "Staff shifts retrieved")
}

// listShifts dispatches to the per-staff or all-staff service read depending on
// the optional staff_id query parameter.
func (rs *Resource) listShifts(r *http.Request, from, to timezone.Date) ([]*scheduleModels.StaffShift, error) {
	if staffStr := r.URL.Query().Get("staff_id"); staffStr != "" {
		staffID, err := strconv.ParseInt(staffStr, 10, 64)
		if err != nil || staffID <= 0 {
			return nil, fmt.Errorf("%w: staff_id must be a positive integer", scheduleSvc.ErrShiftInvalid)
		}
		return rs.Service.ListShiftsForStaff(r.Context(), staffID, from, to)
	}
	return rs.Service.ListShifts(r.Context(), from, to)
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
		PreserveExistingChangeReason: !req.ChangeReason.Present,
		// The ordinary update never flips the cancellation state: doing so would
		// change the origin flag without maintaining its replacement set — a
		// reactivation would leave other people's covers active (double-counting
		// the plan) and a cancel could target a replacement row. Cancel /
		// reactivate always goes through PUT /{id}/cancellation, which rebuilds
		// the cover set atomically (#1841). Any cancelled key on a plain PUT is
		// ignored here.
		PreserveExistingCancelled: true,
	})
	if err != nil {
		renderServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, ToShiftResponse(saved), "Staff shift updated")
}

// MoveShiftRequest is the complete desired slot for PUT /{id}/move. The
// source owner makes cross-person retries distinguishable from stale moves.
type MoveShiftRequest struct {
	SourceStaffID int64      `json:"source_staff_id"`
	TargetStaffID int64      `json:"target_staff_id"`
	Date          string     `json:"date"`
	StartTime     string     `json:"start_time"`
	EndTime       string     `json:"end_time"`
	BreakMinutes  int        `json:"break_minutes"`
	ShiftTypeID   optionalID `json:"shift_type_id"`
}

func (rs *Resource) move(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	var req MoveShiftRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if !req.ShiftTypeID.Present {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("shift_type_id is required")))
		return
	}
	date, err := timezone.ParseDate(req.Date)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("date must be YYYY-MM-DD")))
		return
	}
	start, end, err := ParseShiftTimes(req.StartTime, req.EndTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	editorID, err := rs.editorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	saved, err := rs.Service.MoveShift(r.Context(), scheduleSvc.MoveShiftInput{
		ShiftID:        id,
		SourceStaffID:  req.SourceStaffID,
		TargetStaffID:  req.TargetStaffID,
		Date:           date,
		StartTime:      start,
		EndTime:        end,
		BreakMinutes:   req.BreakMinutes,
		ShiftTypeID:    req.ShiftTypeID.Value,
		ActorStaffID:   editorID,
		ActorAccountID: actorAccountID(r.Context()),
	})
	if err != nil {
		// A series exception may already have been written before a later
		// validation/persistence failure. Roll back every part of the move even
		// when the client-facing result is a 4xx.
		tenant.MarkRollback(r.Context())
		renderServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, ToShiftResponse(saved), "Staff shift moved")
}

// CancellationRequest is the payload for PUT /{id}/cancellation: flip the
// shift's cancelled flag, carry the origin shift's own (possibly edited) window
// and type, record a reason, and (when cancelling) declare the full set of
// replacement covers. The backend applies all of it in one transaction (#1841).
type CancellationRequest struct {
	// Cancelled is presence-tracked: this endpoint is destructive (a
	// reactivation deletes every replacement), so an omitted key is rejected
	// rather than defaulting to false and silently reactivating a shift a stale
	// or malformed client only meant to tweak.
	Cancelled    optionalBool `json:"cancelled"`
	ChangeReason *string      `json:"change_reason"`
	// StartTime/EndTime/BreakMinutes/ShiftTypeID are the origin shift's own values
	// as the admin sees them. An origin edit is all-or-nothing: applying it
	// overwrites the stored window, break AND type, so a partial payload (window
	// sent, break_minutes or shift_type_id omitted) would silently reset the missing
	// fields to 0/null. break_minutes and shift_type_id are therefore presence-tracked
	// and the handler requires the complete set whenever any origin field is present;
	// omitting all four preserves the stored window/type/break (#1841).
	StartTime    string               `json:"start_time"`
	EndTime      string               `json:"end_time"`
	BreakMinutes optionalInt          `json:"break_minutes"`
	ShiftTypeID  optionalID           `json:"shift_type_id"`
	Replacements []ReplacementRequest `json:"replacements"`
}

// ReplacementRequest is one person covering part of a cancelled shift's gap.
type ReplacementRequest struct {
	StaffID      int64  `json:"staff_id"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	BreakMinutes int    `json:"break_minutes"`
	ShiftTypeID  *int64 `json:"shift_type_id"`
}

// CancellationResponse returns the updated origin plus the created covers.
type CancellationResponse struct {
	Shift        ShiftResponse   `json:"shift"`
	Replacements []ShiftResponse `json:"replacements"`
}

// cancellation handles PUT /{id}/cancellation — the atomic cancel/reactivate +
// replacement-set operation. The whole request already runs in one tenant
// transaction; on any service error we explicitly mark it for rollback so a
// non-5xx result (overlap conflict, invalid input) still discards the partial
// writes instead of committing a half-applied change.
func (rs *Resource) cancellation(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	var req CancellationRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	// The cancelled flag drives a destructive operation (a reactivation removes
	// every replacement), so it must be explicit — never inferred as false from
	// an omitted key.
	if !req.Cancelled.Present {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("cancelled is required")))
		return
	}
	editorID, err := rs.editorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	input := scheduleSvc.CancelShiftInput{
		ShiftID:      id,
		Cancelled:    req.Cancelled.Value,
		ChangeReason: req.ChangeReason,
		ActorStaffID: editorID,
	}
	// Apply the origin's own edited window/type when the client supplies it (the
	// admin modal always sends the full set), so a time/type change made alongside
	// the cancellation is not silently dropped (#1841). The edit is all-or-nothing —
	// applying it overwrites the stored window, break AND type — so a partial payload
	// (e.g. a window change with break_minutes or shift_type_id omitted) would reset
	// the omitted fields to 0/null. Require the complete set whenever any origin
	// field is present; omitting all four preserves the stored origin values.
	originEditIntended := req.StartTime != "" || req.EndTime != "" ||
		req.BreakMinutes.Present || req.ShiftTypeID.Present
	if originEditIntended {
		if req.StartTime == "" || req.EndTime == "" || !req.BreakMinutes.Present || !req.ShiftTypeID.Present {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(
				"origin shift edits require start_time, end_time, break_minutes and shift_type_id together")))
			return
		}
		start, end, err := ParseShiftTimes(req.StartTime, req.EndTime)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		input.ApplyOriginEdits = true
		input.StartTime = start
		input.EndTime = end
		input.BreakMinutes = req.BreakMinutes.Value
		input.ShiftTypeID = req.ShiftTypeID.Value
	}
	for _, rep := range req.Replacements {
		start, end, err := ParseShiftTimes(rep.StartTime, rep.EndTime)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		input.Replacements = append(input.Replacements, scheduleSvc.ShiftReplacementInput{
			StaffID:      rep.StaffID,
			StartTime:    start,
			EndTime:      end,
			BreakMinutes: rep.BreakMinutes,
			ShiftTypeID:  rep.ShiftTypeID,
		})
	}

	result, err := rs.Service.ApplyCancellation(r.Context(), input)
	if err != nil {
		// Roll back the enclosing tenant transaction so an overlap/invalid error
		// mid-operation does not commit the writes that already succeeded.
		tenant.MarkRollback(r.Context())
		renderServiceError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, CancellationResponse{
		Shift:        ToShiftResponse(result.Shift),
		Replacements: ToShiftResponses(result.Replacements),
	}, "Staff shift cancellation applied")
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
