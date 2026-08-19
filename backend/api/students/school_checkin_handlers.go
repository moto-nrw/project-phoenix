package students

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
)

// schoolCheckinRequest is the payload for POST /api/students/{id}/school-checkin.
// Action is explicit (not a toggle) so the endpoint is idempotent: repeating
// "in" when the student is already checked in returns 200 without change.
type schoolCheckinRequest struct {
	Action string `json:"action"` // "in" | "out"
}

// schoolCheckinResponse mirrors the shape of AttendanceStatus so the frontend
// can render the new state directly without a follow-up fetch. The Location
// field is the resolved binary-mode label ("Anwesend" | "Schulhof" | "Abwesend")
// so PresenceBadge can update optimistically in the UI.
type schoolCheckinResponse struct {
	StudentID    int64      `json:"student_id"`
	Status       string     `json:"status"`
	CheckInTime  *time.Time `json:"check_in_time,omitempty"`
	CheckOutTime *time.Time `json:"check_out_time,omitempty"`
	YardSince    *time.Time `json:"yard_since,omitempty"`
	Location     string     `json:"location"`
	Changed      bool       `json:"changed"` // false when the call was a no-op (already in target state)
}

// Wire-level action values — aliases of the service constants so handler and
// service can never drift apart.
const (
	schoolCheckinActionIn  = activeService.SchoolCheckinActionIn
	schoolCheckinActionOut = activeService.SchoolCheckinActionOut
)

// schoolCheckinHandler marks a student as checked-in or checked-out of the
// school via the web UI. Mode-agnostic — always writes active.attendance only;
// never creates visits (those come from the IoT kiosk flow). In detailed mode
// a "out" action also ends any open visit for that student so the attendance
// and visit state stay consistent.
//
// Authorization layers:
//  1. JWT + users:checkin permission (route middleware)
//  2. Tenant scoping: student belongs to caller's tenant (middleware + repo)
//
// Any verified staff member may toggle any student (#2329) — the former
// attendance.web_checkin_access per-group gate is gone.
func (rs *Resource) schoolCheckinHandler(w http.ResponseWriter, r *http.Request) {
	logger := rs.getLogger().With(slog.String("handler", "schoolCheckin"))

	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	var req schoolCheckinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if req.Action != schoolCheckinActionIn && req.Action != schoolCheckinActionOut {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(`action must be "in" or "out"`)))
		return
	}

	staffID, err := rs.getStaffIDFromJWT(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorForbidden(err))
		return
	}

	current, err := rs.ActiveService.GetStudentAttendanceStatus(r.Context(), student.ID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	resp, changeErr := rs.applySchoolCheckinAction(r.Context(), student, staffID, req.Action, current)
	if changeErr != nil {
		// A graduated (alumnus) student — reached via CheckInStudent's
		// ensureStudentCheckinAllowed guard on a stale request or graduation
		// race — is treated like an unknown/absent student (404), matching the
		// IoT and timetable mappers rather than surfacing a 500 (#405).
		if errors.Is(changeErr, activeService.ErrStudentGraduated) {
			common.RenderError(w, r, common.ErrorNotFound(changeErr))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(changeErr))
		return
	}

	logger.Info("student_school_checkin",
		slog.Int64("student_id", student.ID),
		slog.Int64("staff_id", staffID),
		slog.String("action", req.Action),
		slog.String("resulting_status", resp.Status),
		slog.Bool("changed", resp.Changed),
	)

	common.Respond(w, r, http.StatusOK, resp, "School checkin toggled successfully")
}

// maxSchoolCheckinBatchSize caps one batch request. The student search view
// loads at most 1000 rows per page walk, so a full-page selection stays
// expressible in a single request while a runaway payload is rejected.
const maxSchoolCheckinBatchSize = 1000

// maxSchoolCheckinBatchBytes bounds the request body BEFORE any JSON decoding.
// 1000 int64 ids serialize to well under 32KB including quotes, commas, and
// the envelope; 64KB leaves generous headroom while a runaway payload never
// gets fully read into memory in the first place.
const maxSchoolCheckinBatchBytes = 64 << 10

// schoolCheckinBatchRequest is the payload for
// POST /api/students/school-checkin/batch. The action applies to every listed
// student; students already in the target state count as no-ops (Changed=false),
// mirroring the single-student endpoint's idempotency.
//
// StudentIDs travel as strings: the frontend holds int64 IDs as strings
// (CLAUDE.md §4), and converting them to JSON numbers client-side would
// silently corrupt values above JavaScript's 2^53-1 safe-integer limit —
// for a write endpoint that means toggling the WRONG student. The backend
// parses them with full int64 range instead.
type schoolCheckinBatchRequest struct {
	Action     string   `json:"action"` // "in" | "out"
	StudentIDs []string `json:"student_ids"`
}

// schoolCheckinBatchResult is the per-student outcome inside a batch response.
// Error carries a machine-readable code ("not_found") — the frontend resolves
// the student's name from its own list, so no PII travels here. StudentID is
// a string for the same reason the request IDs are: the client matches these
// values back against its selection set, which a lossy JSON-number round-trip
// above 2^53-1 would silently break.
type schoolCheckinBatchResult struct {
	StudentID string `json:"student_id"`
	OK        bool   `json:"ok"`
	Changed   bool   `json:"changed"`
	Status    string `json:"status,omitempty"`
	Location  string `json:"location,omitempty"`
	Error     string `json:"error,omitempty"`
}

type schoolCheckinBatchResponse struct {
	Action    string                     `json:"action"`
	Results   []schoolCheckinBatchResult `json:"results"`
	Succeeded int                        `json:"succeeded"`
	Failed    int                        `json:"failed"`
}

// schoolCheckinBatchHandler applies one explicit check-in/out action to a set
// of students (#2359, Sammelauswahl in der Kindersuche).
//
// Failure semantics: the whole request runs inside one tenant transaction
// (TenantTxMiddleware), so an unexpected write error aborts the transaction —
// earlier writes would never commit. Reporting them per-student as "ok" would
// lie, so any unexpected service error fails the entire request with 500 and
// nothing is applied. The per-student "error" entries cover only conditions
// detected BEFORE any write for that student (unknown/foreign/graduated id),
// which never poison the transaction: those students are skipped and the rest
// of the batch still commits.
func (rs *Resource) schoolCheckinBatchHandler(w http.ResponseWriter, r *http.Request) {
	logger := rs.getLogger().With(slog.String("handler", "schoolCheckinBatch"))

	// Bound the body BEFORE decoding: without this an arbitrarily large
	// payload is fully read and unmarshalled before any length check can
	// reject it.
	r.Body = http.MaxBytesReader(w, r.Body, maxSchoolCheckinBatchBytes)
	dec := json.NewDecoder(r.Body)
	var req schoolCheckinBatchRequest
	if err := dec.Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	// The decoder stops at the end of the first JSON value, so trailing bytes
	// would otherwise stay unread — an oversized or malformed tail would slip
	// past the size limit above. Requiring a clean EOF drains the bounded
	// body: trailing data is rejected, and a tail larger than the limit trips
	// MaxBytesReader while being read (review #2372).
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("unexpected trailing data after JSON body")))
		return
	}
	if req.Action != schoolCheckinActionIn && req.Action != schoolCheckinActionOut {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(`action must be "in" or "out"`)))
		return
	}
	// Cap the RAW list before parsing and deduplication: a payload of
	// duplicate ids must not buy unbounded parse work just because it would
	// collapse to a few students afterwards.
	if len(req.StudentIDs) > maxSchoolCheckinBatchSize {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("too many students in one batch")))
		return
	}
	parsedIDs, parseErr := parseStudentIDStrings(req.StudentIDs)
	if parseErr != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(parseErr))
		return
	}
	studentIDs := dedupeStudentIDs(parsedIDs)
	if len(studentIDs) == 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("student_ids must not be empty")))
		return
	}

	staffID, err := rs.getStaffIDFromJWT(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorForbidden(err))
		return
	}

	batch, err := rs.ActiveService.ProcessSchoolCheckinBatch(r.Context(), studentIDs, staffID, req.Action)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	resp := buildSchoolCheckinBatchResponse(req.Action, batch)

	logger.Info("student_school_checkin_batch",
		slog.Int64("staff_id", staffID),
		slog.String("action", req.Action),
		slog.Int("requested", len(studentIDs)),
		slog.Int("succeeded", resp.Succeeded),
		slog.Int("failed", resp.Failed),
	)

	common.Respond(w, r, http.StatusOK, resp, "School checkin batch processed")
}

// buildSchoolCheckinBatchResponse formats the service-level batch result into
// the wire shape: string ids, the machine-readable "not_found" code ("the
// frontend resolves the student's name from its own list, so no PII travels
// here"), and the resolved German presence label per successful entry.
func buildSchoolCheckinBatchResponse(action string, batch *activeService.SchoolCheckinBatchResult) *schoolCheckinBatchResponse {
	resp := &schoolCheckinBatchResponse{
		Action:    action,
		Results:   make([]schoolCheckinBatchResult, 0, len(batch.Results)),
		Succeeded: batch.Succeeded,
		Failed:    batch.Failed,
	}
	for _, item := range batch.Results {
		if !item.OK {
			resp.Results = append(resp.Results, schoolCheckinBatchResult{
				StudentID: strconv.FormatInt(item.StudentID, 10),
				Error:     "not_found",
			})
			continue
		}
		resp.Results = append(resp.Results, schoolCheckinBatchResult{
			StudentID: strconv.FormatInt(item.StudentID, 10),
			OK:        true,
			Changed:   item.Changed,
			Status:    item.Status,
			Location:  labelForAttendanceStatus(item.Status),
		})
	}
	return resp
}

// parseStudentIDStrings converts the wire-format string IDs to int64. Any
// malformed entry rejects the whole request: the frontend only ever sends IDs
// it received from the backend, so a non-numeric value is a client bug, not a
// per-student condition.
func parseStudentIDStrings(ids []string) ([]int64, error) {
	out := make([]int64, 0, len(ids))
	for _, raw := range ids {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("student_ids must be numeric id strings: %q", raw)
		}
		out = append(out, id)
	}
	return out, nil
}

// dedupeStudentIDs drops duplicates while keeping first-seen order, so a
// double-tapped selection never toggles a student twice in one batch.
func dedupeStudentIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// applySchoolCheckinAction is the state-transition core. It short-circuits on
// idempotent no-ops (Changed=false) and otherwise writes active.attendance.
// In detailed mode a successful "out" transition also ends any open visit so
// the room-location view doesn't drift from attendance. In binary mode
// ActiveService.EndVisit is a no-op (L3.1 gate), so the code path is identical.
func (rs *Resource) applySchoolCheckinAction(
	ctx context.Context,
	student *users.Student,
	staffID int64,
	action string,
	current *activeService.AttendanceStatus,
) (*schoolCheckinResponse, error) {
	if activeService.IsSchoolCheckinNoop(action, current.Status) {
		return buildSchoolCheckinResponse(student.ID, current, false), nil
	}

	// Action-explicit writes — never route through ToggleStudentAttendance
	// here. The toggle re-reads state and flips, which under concurrency can
	// turn a desired "in" into an "out" (two parallel "in" requests for an
	// absent student: the second sees the first's commit and flips). The
	// CheckIn/CheckOut methods are race-safe individually (ON CONFLICT for
	// in, state-checked UPDATE for out) so the action contract is preserved.
	var result *activeService.AttendanceResult
	var err error
	switch action {
	case schoolCheckinActionIn:
		result, err = rs.ActiveService.CheckInStudent(ctx, student.ID, staffID, 0, true)
	case schoolCheckinActionOut:
		// CheckOutStudent also ends any open room visit in the same request
		// transaction (issue #895 — see services/active.performCheckOut), so
		// detailed-mode supervisor views never show "still in Room X" after
		// a web checkout. No separate EndVisit call is needed here.
		result, err = rs.ActiveService.CheckOutStudent(ctx, student.ID, staffID, true)
	}
	if err != nil {
		return nil, err
	}

	// Re-fetch so the response reflects the freshly-written row. Changed comes
	// from the write itself, not from the stale pre-read: a concurrent caller
	// may have established the target state after our no-op check, and that
	// absorbed write is a no-op, not a change.
	updated, err := rs.ActiveService.GetStudentAttendanceStatus(ctx, student.ID)
	if err != nil {
		return nil, err
	}
	return buildSchoolCheckinResponse(student.ID, updated, result.Changed), nil
}

// buildSchoolCheckinResponse formats an AttendanceStatus into the HTTP response
// shape, including the resolved presence label so the client can render
// PresenceBadge without a follow-up snapshot fetch.
func buildSchoolCheckinResponse(studentID int64, status *activeService.AttendanceStatus, changed bool) *schoolCheckinResponse {
	resp := &schoolCheckinResponse{
		StudentID:    studentID,
		Status:       status.Status,
		CheckInTime:  status.CheckInTime,
		CheckOutTime: status.CheckOutTime,
		YardSince:    status.YardSince,
		Changed:      changed,
	}
	resp.Location = labelForAttendanceStatus(status.Status)
	return resp
}

// labelForAttendanceStatus maps the tri-state status to the German presence
// label used by the frontend PresenceBadge component.
func labelForAttendanceStatus(status string) string {
	switch status {
	case "checked_in":
		return "Anwesend"
	case "on_yard":
		return "Schulhof"
	default: // "checked_out", "not_checked_in", or unknown
		return "Abwesend"
	}
}

// getLogger returns a nil-safe logger for handler use.
func (rs *Resource) getLogger() *slog.Logger {
	return cmp.Or(rs.Logger, slog.Default())
}
