// Package staffshifts exposes admin CRUD for planned per-date staff shifts
// (Dienstplan, #1376 core slice). Shifts carry the concrete wall-clock times
// the auto-checkout job (#1798) closes forgotten work sessions against.
// Staff members read their own shifts via /api/time-tracking/shifts.
//
// Authentication, transaction scoping, permission names, rendering, actor
// resolution and the rollback marker are supplied by the composition root
// through Runtime; the handlers call the public Workforce planning contract.
package staffshifts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// Middleware is the HTTP middleware shape the composition root supplies.
type Middleware = func(http.Handler) http.Handler

// FailureKind classifies a handler failure so the composition root can render
// it with the project's shared error envelope.
type FailureKind string

const (
	FailureInvalid      FailureKind = "invalid"
	FailureNotFound     FailureKind = "not_found"
	FailureConflict     FailureKind = "conflict"
	FailureUnauthorized FailureKind = "unauthorized"
	FailureForbidden    FailureKind = "forbidden"
	FailureInternal     FailureKind = "internal"
)

// ClientMessageError is an internal failure whose cause must not reach the
// client: the envelope renders Message and logs the cause.
type ClientMessageError struct {
	Message string
	Cause   error
}

func (e *ClientMessageError) Error() string { return e.Message }
func (e *ClientMessageError) Unwrap() error { return e.Cause }

// Actor is the acting admin behind a write: the staff record every plan
// change is attributed to and the account an audit entry names.
type Actor struct {
	StaffID   int64
	AccountID *int64
}

// Runtime carries the HTTP-platform behavior this adapter must not own. Every
// field is required, so missing production wiring fails at startup.
type Runtime struct {
	Protected  func(chi.Router, func(chi.Router, Middleware))
	Permission func(string) Middleware
	ParseID    func(*http.Request) (int64, error)
	Success    func(http.ResponseWriter, *http.Request, int, any, string)
	Failure    func(http.ResponseWriter, *http.Request, FailureKind, error)
	// ResolveActor identifies the acting admin; a request without a staff
	// record behind it is unauthorized.
	ResolveActor func(context.Context) (Actor, error)
	// MarkRollback discards the request's tenant transaction so a
	// client-facing 4xx never commits a half-applied plan change.
	MarkRollback func(context.Context)
	// CanExportInternalPlan reports whether the caller may receive the
	// internal plan variant with reasons and gaps.
	CanExportInternalPlan func(context.Context) bool
	// Permission names.
	TimeTrackingManage string
	SchedulesRead      string
	UsersRead          string
}

// Resource bundles the dependencies for the staff-shift HTTP handlers.
type Resource struct {
	planning workforce.StaffShiftPlanning
	runtime  Runtime
}

// NewResource wires the dependencies.
func NewResource(planning workforce.StaffShiftPlanning, runtime Runtime) *Resource {
	return &Resource{planning: planning, runtime: runtime}
}

// Router returns the chi sub-router for /api/staff-shifts.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	rs.runtime.Protected(r, func(r chi.Router, withTx Middleware) {
		manage := rs.runtime.Permission(rs.runtime.TimeTrackingManage)
		r.With(manage, withTx).Get("/", rs.list)
		// The overview and the printable week read the same projection, so
		// both need the permission triple; export.go additionally requires
		// schedules:manage for the sensitive internal variant.
		r.With(
			manage,
			rs.runtime.Permission(rs.runtime.SchedulesRead),
			rs.runtime.Permission(rs.runtime.UsersRead),
			withTx,
		).Get("/overview", rs.overview)
		r.With(
			manage,
			rs.runtime.Permission(rs.runtime.SchedulesRead),
			rs.runtime.Permission(rs.runtime.UsersRead),
			withTx,
		).Post("/export", rs.exportPlan)
		r.With(manage, withTx).Post("/", rs.create)
		r.With(manage, withTx).Put("/{id}", rs.update)
		r.With(manage, withTx).Put("/{id}/move", rs.move)
		r.With(manage, withTx).Put("/{id}/cancellation", rs.cancellation)
		r.With(manage, withTx).Delete("/{id}", rs.delete)
		r.With(manage, withTx).Post("/series", rs.createSeries)
		r.With(manage, withTx).Get("/series/{id}", rs.getSeries)
		r.With(manage, withTx).Put("/series/{id}/split", rs.splitSeries)
		r.With(manage, withTx).Delete("/series/{id}", rs.endSeries)
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
	OriginShiftID optionalID `json:"origin_shift_id"`
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
	var decimal string
	if err := json.Unmarshal(data, &decimal); err == nil {
		v, err := strconv.ParseInt(decimal, 10, 64)
		if err != nil {
			return fmt.Errorf("ID must be a signed 64-bit integer: %w", err)
		}
		o.Value = &v
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
	// IDs cross the JSON boundary as decimal strings. A JavaScript number cannot
	// represent every PostgreSQL bigint value without rounding it.
	ID           int64  `json:"id,string"`
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
	SeriesID       *int64  `json:"series_id,omitempty,string"`
	Detached       bool    `json:"detached"`
	Cancelled      bool    `json:"cancelled"`
	ChangeReason   *string `json:"change_reason,omitempty"`
	OriginShiftID  *int64  `json:"origin_shift_id,omitempty,string"`
	// SeriesOccurrenceDate is the immutable recurrence slot. It lets clients
	// apply a rule edit from a moved occurrence's source date, not its current
	// display date.
	SeriesOccurrenceDate *string `json:"series_occurrence_date,omitempty"`
}

// ToShiftResponse maps a shift onto the wire format. Exported for the
// time-tracking self endpoint, which serves the same shape.
func ToShiftResponse(s workforce.PlannedShift) ShiftResponse {
	resp := ShiftResponse{
		ID:            s.ID,
		StaffID:       s.StaffID,
		Date:          s.Date,
		StartTime:     FormatWallClock(s.StartTime),
		EndTime:       FormatWallClock(s.EndTime),
		BreakMinutes:  s.BreakMinutes,
		ShiftTypeID:   s.ShiftTypeID,
		Notes:         s.Notes,
		SeriesID:      s.SeriesID,
		Detached:      s.Detached,
		Cancelled:     s.Cancelled,
		ChangeReason:  s.ChangeReason,
		OriginShiftID: s.OriginShiftID,
	}
	if s.SeriesOccurrenceDate != "" {
		occurrenceDate := s.SeriesOccurrenceDate
		resp.SeriesOccurrenceDate = &occurrenceDate
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
func ToShiftResponses(shifts []workforce.PlannedShift) []ShiftResponse {
	out := make([]ShiftResponse, 0, len(shifts))
	for _, s := range shifts {
		out = append(out, ToShiftResponse(s))
	}
	return out
}

// FormatWallClock renders a ClockLayout wall clock as the "HH:MM" the
// clients read. Exported for the time-tracking self endpoint.
func FormatWallClock(value string) string {
	parsed, err := time.Parse(workforce.ClockLayout, value)
	if err != nil {
		return value
	}
	return parsed.Format("15:04")
}

// parseShiftTimes parses "HH:MM" start/end strings into ClockLayout wall
// clocks.
func parseShiftTimes(startStr, endStr string) (start, end string, err error) {
	startClock, err := time.Parse("15:04", startStr)
	if err != nil {
		return "", "", errors.New("start_time must be HH:MM")
	}
	endClock, err := time.Parse("15:04", endStr)
	if err != nil {
		return "", "", errors.New("end_time must be HH:MM")
	}
	return startClock.Format(workforce.ClockLayout), endClock.Format(workforce.ClockLayout), nil
}

// parseDate accepts a strict calendar day.
func parseDate(value string) (string, bool) {
	parsed, err := time.Parse(workforce.DateLayout, value)
	if err != nil || parsed.Format(workforce.DateLayout) != value {
		return "", false
	}
	return value, true
}

func buildShift(req ShiftRequest) (workforce.StaffShiftInput, error) {
	date, ok := parseDate(req.Date)
	if !ok {
		return workforce.StaffShiftInput{}, errors.New("date must be YYYY-MM-DD")
	}
	start, end, err := parseShiftTimes(req.StartTime, req.EndTime)
	if err != nil {
		return workforce.StaffShiftInput{}, err
	}
	notes := ""
	if req.Notes != nil {
		notes = *req.Notes
	}
	return workforce.StaffShiftInput{
		StaffID:       req.StaffID,
		Date:          date,
		StartTime:     start,
		EndTime:       end,
		BreakMinutes:  req.BreakMinutes,
		ShiftTypeID:   req.ShiftTypeID.Value,
		Notes:         notes,
		Cancelled:     req.Cancelled.Value,
		ChangeReason:  req.ChangeReason.Value,
		OriginShiftID: req.OriginShiftID.Value,
	}, nil
}

// failureRule pairs a capability error with the failure kind the envelope
// renders for it. The table is the declarative form of the error
// classification; anything not listed is an internal failure.
type failureRule struct {
	Target error
	Kind   FailureKind
}

var failureRules = []failureRule{
	{Target: workforce.ErrStaffShiftOverlap, Kind: FailureConflict},
	{Target: workforce.ErrStaffShiftConflict, Kind: FailureConflict},
	{Target: workforce.ErrStaffShiftNotFound, Kind: FailureNotFound},
	{Target: workforce.ErrStaffShiftRangeTooLarge, Kind: FailureInvalid},
	{Target: workforce.ErrInvalidStaffShift, Kind: FailureInvalid},
	{Target: workforce.ErrShiftTypeNotFound, Kind: FailureInvalid},
	{Target: workforce.ErrShiftTypeInactive, Kind: FailureInvalid},
	{Target: workforce.ErrShiftSeriesNotFound, Kind: FailureNotFound},
	{Target: workforce.ErrInvalidShiftSeries, Kind: FailureInvalid},
	{Target: workforce.ErrPlanExportInvalid, Kind: FailureInvalid},
	{Target: workforce.ErrPlanExportForbidden, Kind: FailureForbidden},
}

// classify maps a capability error to its failure kind through failureRules.
func classify(err error) FailureKind {
	for _, rule := range failureRules {
		if errors.Is(err, rule.Target) {
			return rule.Kind
		}
	}
	return FailureInternal
}

func (rs *Resource) renderError(w http.ResponseWriter, r *http.Request, err error) {
	rs.runtime.Failure(w, r, classify(err), err)
}

func (rs *Resource) invalid(w http.ResponseWriter, r *http.Request, err error) {
	rs.runtime.Failure(w, r, FailureInvalid, err)
}

// parseDateRange extracts "from" and "to" query parameters as calendar dates.
func (rs *Resource) parseDateRange(w http.ResponseWriter, r *http.Request) (from, to string, ok bool) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	if fromStr == "" || toStr == "" {
		rs.invalid(w, r, errors.New("from and to query parameters are required"))
		return "", "", false
	}
	from, ok = parseDate(fromStr)
	if !ok {
		rs.invalid(w, r, errors.New("invalid from date format, expected YYYY-MM-DD"))
		return "", "", false
	}
	to, ok = parseDate(toStr)
	if !ok {
		rs.invalid(w, r, errors.New("invalid to date format, expected YYYY-MM-DD"))
		return "", "", false
	}
	return from, to, true
}

// actor resolves the acting admin or renders the unauthorized failure.
func (rs *Resource) actor(w http.ResponseWriter, r *http.Request) (Actor, bool) {
	actor, err := rs.runtime.ResolveActor(r.Context())
	if err != nil {
		rs.runtime.Failure(w, r, FailureUnauthorized, err)
		return Actor{}, false
	}
	return actor, true
}

func (rs *Resource) list(w http.ResponseWriter, r *http.Request) {
	from, to, ok := rs.parseDateRange(w, r)
	if !ok {
		return
	}
	// Optional staff_id narrows the week grid to one staff member — the admin
	// staff-detail Plan|Ist view needs exactly that person's planned shifts
	// (#1844) rather than the whole tenant's. A filter param, not a second route.
	query := workforce.ShiftRange{From: from, To: to}
	if staffStr := r.URL.Query().Get("staff_id"); staffStr != "" {
		staffID, err := strconv.ParseInt(staffStr, 10, 64)
		if err != nil || staffID <= 0 {
			rs.invalid(w, r, fmt.Errorf("%w: staff_id must be a positive integer", workforce.ErrInvalidStaffShift))
			return
		}
		query.StaffID = staffID
	}
	shifts, err := rs.planning.ListShifts(r.Context(), query)
	if err != nil {
		rs.renderError(w, r, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, ToShiftResponses(shifts), "Staff shifts retrieved")
}

func (rs *Resource) create(w http.ResponseWriter, r *http.Request) {
	var req ShiftRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		rs.invalid(w, r, err)
		return
	}
	input, err := buildShift(req)
	if err != nil {
		rs.invalid(w, r, err)
		return
	}
	actor, ok := rs.actor(w, r)
	if !ok {
		return
	}
	input.ActorStaffID = actor.StaffID

	saved, err := rs.planning.CreateShift(r.Context(), workforce.CreateStaffShift{StaffShiftInput: input})
	if err != nil {
		rs.renderError(w, r, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusCreated, ToShiftResponse(saved), "Staff shift created")
}

func (rs *Resource) update(w http.ResponseWriter, r *http.Request) {
	id, err := rs.runtime.ParseID(r)
	if err != nil {
		rs.invalid(w, r, err)
		return
	}
	var req ShiftRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		rs.invalid(w, r, err)
		return
	}
	input, err := buildShift(req)
	if err != nil {
		rs.invalid(w, r, err)
		return
	}
	actor, ok := rs.actor(w, r)
	if !ok {
		return
	}
	input.ActorStaffID = actor.StaffID

	saved, err := rs.planning.UpdateShift(r.Context(), workforce.UpdateStaffShift{
		ID:                           id,
		StaffShiftInput:              input,
		PreserveExistingNotes:        req.Notes == nil,
		PreserveExistingShiftType:    !req.ShiftTypeID.Present,
		PreserveExistingChangeReason: !req.ChangeReason.Present,
	})
	if err != nil {
		rs.renderError(w, r, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, ToShiftResponse(saved), "Staff shift updated")
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
	id, err := rs.runtime.ParseID(r)
	if err != nil {
		rs.invalid(w, r, err)
		return
	}
	var req MoveShiftRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		rs.invalid(w, r, err)
		return
	}
	input, err := buildMove(id, req)
	if err != nil {
		rs.invalid(w, r, err)
		return
	}
	actor, ok := rs.actor(w, r)
	if !ok {
		return
	}
	input.ActorStaffID = actor.StaffID
	input.ActorAccountID = actor.AccountID

	saved, err := rs.planning.MoveShift(r.Context(), input)
	if err != nil {
		// A series exception may already have been written before a later
		// validation/persistence failure. Roll back every part of the move even
		// when the client-facing result is a 4xx.
		rs.runtime.MarkRollback(r.Context())
		rs.renderError(w, r, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, ToShiftResponse(saved), "Staff shift moved")
}

func buildMove(id int64, req MoveShiftRequest) (workforce.MoveStaffShift, error) {
	if !req.ShiftTypeID.Present {
		return workforce.MoveStaffShift{}, errors.New("shift_type_id is required")
	}
	date, ok := parseDate(req.Date)
	if !ok {
		return workforce.MoveStaffShift{}, errors.New("date must be YYYY-MM-DD")
	}
	start, end, err := parseShiftTimes(req.StartTime, req.EndTime)
	if err != nil {
		return workforce.MoveStaffShift{}, err
	}
	return workforce.MoveStaffShift{
		ShiftID:       id,
		SourceStaffID: req.SourceStaffID,
		TargetStaffID: req.TargetStaffID,
		Date:          date,
		StartTime:     start,
		EndTime:       end,
		BreakMinutes:  req.BreakMinutes,
		ShiftTypeID:   req.ShiftTypeID.Value,
	}, nil
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
	id, err := rs.runtime.ParseID(r)
	if err != nil {
		rs.invalid(w, r, err)
		return
	}
	var req CancellationRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		rs.invalid(w, r, err)
		return
	}
	// The cancelled flag drives a destructive operation (a reactivation removes
	// every replacement), so it must be explicit — never inferred as false from
	// an omitted key.
	if !req.Cancelled.Present {
		rs.invalid(w, r, errors.New("cancelled is required"))
		return
	}
	actor, ok := rs.actor(w, r)
	if !ok {
		return
	}
	input, err := buildCancellation(id, req)
	if err != nil {
		rs.invalid(w, r, err)
		return
	}
	input.ActorStaffID = actor.StaffID

	result, err := rs.planning.ApplyCancellation(r.Context(), input)
	if err != nil {
		// Roll back the enclosing tenant transaction so an overlap/invalid error
		// mid-operation does not commit the writes that already succeeded.
		rs.runtime.MarkRollback(r.Context())
		rs.renderError(w, r, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, CancellationResponse{
		Shift:        ToShiftResponse(result.Shift),
		Replacements: ToShiftResponses(result.Replacements),
	}, "Staff shift cancellation applied")
}

func buildCancellation(id int64, req CancellationRequest) (workforce.CancelStaffShift, error) {
	if !req.Cancelled.Present {
		return workforce.CancelStaffShift{}, errors.New("cancelled is required")
	}
	input := workforce.CancelStaffShift{
		ShiftID:      id,
		Cancelled:    req.Cancelled.Value,
		ChangeReason: req.ChangeReason,
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
			return workforce.CancelStaffShift{}, errors.New(
				"origin shift edits require start_time, end_time, break_minutes and shift_type_id together")
		}
		start, end, err := parseShiftTimes(req.StartTime, req.EndTime)
		if err != nil {
			return workforce.CancelStaffShift{}, err
		}
		input.ApplyOriginEdits = true
		input.StartTime = start
		input.EndTime = end
		input.BreakMinutes = req.BreakMinutes.Value
		input.ShiftTypeID = req.ShiftTypeID.Value
	}
	for _, rep := range req.Replacements {
		start, end, err := parseShiftTimes(rep.StartTime, rep.EndTime)
		if err != nil {
			return workforce.CancelStaffShift{}, err
		}
		input.Replacements = append(input.Replacements, workforce.ShiftReplacement{
			StaffID:      rep.StaffID,
			StartTime:    start,
			EndTime:      end,
			BreakMinutes: rep.BreakMinutes,
			ShiftTypeID:  rep.ShiftTypeID,
		})
	}
	return input, nil
}

func (rs *Resource) delete(w http.ResponseWriter, r *http.Request) {
	id, err := rs.runtime.ParseID(r)
	if err != nil {
		rs.invalid(w, r, err)
		return
	}
	if err := rs.planning.DeleteShift(r.Context(), id); err != nil {
		rs.renderError(w, r, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, map[string]any{"id": id}, "Staff shift deleted")
}
