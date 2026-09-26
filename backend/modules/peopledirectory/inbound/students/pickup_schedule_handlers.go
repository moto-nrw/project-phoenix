package students

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// getStaffIDFromJWT extracts the staff ID from JWT claims by looking up the person and staff
func (rs *Resource) getStaffIDFromJWT(r *http.Request) (int64, error) {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		return 0, errors.New("no valid JWT claims found")
	}

	// Get person from account ID
	person, err := rs.PeopleDirectory.FindPersonByAccount(r.Context(), int64(claims.ID))
	if err != nil {
		if errors.Is(err, peopleModule.ErrPersonNotFound) {
			return 0, errPersonNotFoundForAccount
		}
		return 0, fmt.Errorf("find person by account: %w", err)
	}

	// Get staff from person ID
	staffID, found, err := rs.Persons.StaffIDForPerson(r.Context(), person.ID)
	if err != nil {
		return 0, fmt.Errorf("find staff by person: %w", err)
	}
	if !found {
		return 0, errUserNotStaff
	}

	return staffID, nil
}

// requirePickupReadAccess parses the student from URL params and checks read
// access: admins and verified staff pass, guest/guardian accounts do not
// (#2329).
// Returns the student on success or writes an error response and returns nil.
func (rs *Resource) requirePickupReadAccess(w http.ResponseWriter, r *http.Request) *Student {
	return rs.requireCareReadAccess(w, r, "pickup")
}

// requirePickupWriteAccess parses the student from URL params and verifies full access.
// Used for write operations (create, update, delete) that require supervisor/admin access.
// Returns the student on success or writes an error response and returns nil.
func (rs *Resource) requirePickupWriteAccess(w http.ResponseWriter, r *http.Request, action string) *Student {
	return rs.requireCareWriteAccess(w, r, action)
}

// parseEntityID extracts a numeric ID from a URL parameter.
// Returns 0 and writes an error response on failure.
func parseEntityID(w http.ResponseWriter, r *http.Request, param string, label string) (int64, bool) {
	id, err := common.ParseIDParam(r, param)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("invalid %s ID", label)))
		return 0, false
	}
	return id, true
}

// getStudentPickupSchedules handles GET /students/{id}/pickup-schedules.
// Readable by admins and verified staff (#2329).
func (rs *Resource) getStudentPickupSchedules(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupReadAccess(w, r)
	if student == nil {
		return
	}

	from, to, hasRange, err := pickupScheduleDateRange(r)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	var data *careplan.StudentPickupData
	if hasRange {
		data, err = rs.PickupScheduleService.GetStudentPickupDataForRange(r.Context(), student.ID, from, to)
	} else {
		data, err = rs.PickupScheduleService.GetStudentPickupData(r.Context(), student.ID)
	}
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	response := buildPickupDataResponse(data)

	common.Respond(w, r, http.StatusOK, response, "Pickup schedules retrieved successfully")
}

func pickupScheduleDateRange(r *http.Request) (timezone.Date, timezone.Date, bool, error) {
	fromRaw := strings.TrimSpace(r.URL.Query().Get("from"))
	toRaw := strings.TrimSpace(r.URL.Query().Get("to"))
	if fromRaw == "" && toRaw == "" {
		return timezone.Date(""), timezone.Date(""), false, nil
	}
	if fromRaw == "" || toRaw == "" {
		return timezone.Date(""), timezone.Date(""), false, errors.New("from and to must be provided together")
	}
	from, err := timezone.ParseDate(fromRaw)
	if err != nil {
		return timezone.Date(""), timezone.Date(""), false, fmt.Errorf("invalid from date: %w", err)
	}
	to, err := timezone.ParseDate(toRaw)
	if err != nil {
		return timezone.Date(""), timezone.Date(""), false, fmt.Errorf("invalid to date: %w", err)
	}
	if to.Before(from) || to.After(from.AddDays(31)) {
		return timezone.Date(""), timezone.Date(""), false, errors.New("pickup schedule date range must span at most 32 days")
	}
	return from, to, true, nil
}

// buildPickupDataResponse converts service pickup data to API response
func buildPickupDataResponse(data *careplan.StudentPickupData) PickupDataResponse {
	response := PickupDataResponse{
		Schedules:          make([]PickupScheduleResponse, 0, len(data.Schedules)),
		EffectiveSchedules: make([]DatedPickupScheduleResponse, 0, len(data.EffectiveSchedules)),
		Exceptions:         make([]PickupExceptionResponse, 0, len(data.Exceptions)),
		Notes:              make([]PickupNoteResponse, 0, len(data.Notes)),
	}

	for _, s := range data.Schedules {
		response.Schedules = append(response.Schedules, mapScheduleToResponse(s))
	}
	for _, e := range data.Exceptions {
		response.Exceptions = append(response.Exceptions, mapExceptionToResponse(e))
	}
	for _, dated := range data.EffectiveSchedules {
		var scheduleResponse *PickupScheduleResponse
		if dated.Schedule != nil {
			mapped := mapScheduleToResponse(dated.Schedule)
			scheduleResponse = &mapped
		}
		var offeringResponse *PickupScheduleResponse
		if dated.OfferingSchedule != nil {
			mapped := mapScheduleToResponse(dated.OfferingSchedule)
			offeringResponse = &mapped
		}
		response.EffectiveSchedules = append(response.EffectiveSchedules, DatedPickupScheduleResponse{
			Date: dated.Date.String(), Schedule: scheduleResponse, OfferingSchedule: offeringResponse,
		})
	}
	for _, n := range data.Notes {
		response.Notes = append(response.Notes, mapNoteToResponse(n))
	}

	return response
}

// broadcastPickupScheduleChanged tells the affected school's open STAFF tabs
// that this child's Gehzeit plan changed. Without it the guardian wake below is
// the only signal a pickup write emits, so the parents app refreshes live while
// staff views that disable focus revalidation (per-child Betreuungsplan,
// student lists) keep showing the previous time until a manual reload.
//
// Scoped to the writing tenant (BroadcastToTenant, like broadcastStudentUpdated
// — deliberately NOT the BroadcastToAll its arrival sibling still uses): only
// that school can read the affected child, so a global fan-out would make every
// other school refetch student and care-plan data for a write they can never
// see. Tenant scope loses no recipient — the hub indexes every staff client
// under its tenant at connect time, including zero-topic admins.
//
// Call it from an after-commit hook: the handler's WithTenantTx only REUSES the
// tx opened by TenantTxMiddleware, so at handler return the write is not yet
// visible to the refetch this wakes (#1725 review). Delivery is
// fire-and-forget; a failure only costs other tabs an auto-refresh, so it is
// logged, never surfaced.
func (rs *Resource) broadcastPickupScheduleChanged(tenantID, studentID int64) {
	if rs.Broadcaster == nil {
		return
	}
	if tenantID <= 0 {
		// Defensive, mirroring broadcastStudentUpdated: without a tenant we do
		// not know whose clients should invalidate, and guessing by going
		// global is exactly the cross-tenant traffic this scoping removes.
		if rs.Logger != nil {
			rs.Logger.Warn(
				"skipping pickup_schedule_changed broadcast, no tenant context",
				"student_id", studentID,
			)
		}
		return
	}

	// No student_id on the payload — deliberately, for GDPR. This is a
	// TENANT-WIDE broadcast: every staff client in the school receives it. A
	// student_id in that raw SSE stream would leak which children have
	// pickup-plan activity to clients without student read access
	// (guest/guardian, #2329).
	// The rule this follows: only GROUP-topic events (student_checkin/checkout
	// via BroadcastToGroup, whose audience is already the child's room) may
	// carry the id; tenant-wide staff events must not — the arrival sibling is
	// id-less for the same reason, and parent_message scrubs the id for its
	// staff fan-out (staffSafeParentMessage). The cost is that each client
	// re-checks its pickup-derived caches rather than only the affected one, but
	// every refetch is itself server-access-filtered per student, so nothing
	// leaks and no unauthorized data is returned.
	source := "manual"
	event := realtime.NewEvent(
		realtime.EventPickupScheduleChanged,
		"",
		realtime.EventData{Source: &source},
	)
	if err := rs.Broadcaster.BroadcastToTenant(tenantID, event); err != nil && rs.Logger != nil {
		rs.Logger.Warn(
			"failed to broadcast pickup schedule change",
			"tenant_id", tenantID,
			"student_id", studentID,
			"error", err.Error(),
		)
	}
}

// updateStudentPickupSchedules handles PUT /students/{id}/pickup-schedules
func (rs *Resource) updateStudentPickupSchedules(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupWriteAccess(w, r, "update pickup schedules")
	if student == nil {
		return
	}

	req := &BulkPickupScheduleRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	// Get staff ID from JWT
	staffID, err := rs.getStaffIDFromJWT(r)
	if err != nil {
		renderError(w, r, common.ErrorForbidden(err))
		return
	}

	// Convert requests to schedule models
	schedules := toPickupScheduleModels(req.Schedules, student.ID, staffID)

	tenantID := tenant.FromContext(r.Context())
	if err := withinTenant(r.Context(), tenantID, func(ctx context.Context) error {
		date := rs.todayDate()
		if req.EffectiveDate != nil {
			date = *req.EffectiveDate
		}
		return rs.PickupScheduleService.UpsertBulkStudentPickupSchedulesForDate(ctx, student.ID, date, schedules)
	}); err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Wake the child's guardians: the weekly base plan feeds the "Heute" pickup
	// tile's base time, so a changed schedule must refetch on an open parents-app
	// tab live (#1725). Defer to the OUTER request transaction's commit — the
	// handler WithTenantTx above only REUSES the tx opened by TenantTxMiddleware
	// (nested), so it has NOT committed on return; waking now would let a client
	// refetch before the write is visible or wake for a write a later 5xx rolls
	// back (#1725 review).
	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.wakeChildGuardians(tenantID, student.ID)
		rs.broadcastPickupScheduleChanged(tenantID, student.ID)
	})

	// Fetch updated data
	data, err := rs.PickupScheduleService.GetStudentPickupData(r.Context(), student.ID)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	response := buildPickupDataResponse(data)

	common.Respond(w, r, http.StatusOK, response, "Pickup schedules updated successfully")
}

func (rs *Resource) bulkUpsertPickupSchedules(w http.ResponseWriter, r *http.Request) {
	req := &BulkPickupSchedulePatchRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	staffID, err := rs.getStaffIDFromJWT(r)
	if err != nil {
		renderError(w, r, common.ErrorForbidden(err))
		return
	}
	if rs.PickupAdjustmentService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("pickup adjustment service not configured")))
		return
	}
	permissions := jwt.PermissionsFromCtx(r.Context())
	claims := jwt.ClaimsFromCtx(r.Context())
	result, err := rs.PickupAdjustmentService.ApplyBulkExceptions(
		r.Context(), careplan.PickupAdjustmentBulkInput{
			StudentIDs: req.StudentIDs, Schedules: req.Schedules,
			ConfirmedException: req.ConfirmedException, CreatedByStaffID: staffID,
			ActorAccountID: int64(claims.ID),
			Authorize: func(ctx context.Context, student careplan.ScheduleStudent) (bool, error) {
				return securityruntime.CanUpdateStudent(ctx, permissions, student, rs.UserContextService)
			},
		},
	)
	if err != nil {
		if errors.Is(err, careplan.ErrPickupAdjustmentBulkConfirmation) {
			renderError(w, r, common.ErrorInvalidRequestWithCode(
				err, "pickup.bulk_exception_confirmation_required",
			))
			return
		}
		if errors.Is(err, careplan.ErrBulkStudentUnauthorized) {
			renderError(w, r, common.ErrorForbidden(err))
			return
		}
		if errors.Is(err, careplan.ErrBulkStudentNotFound) {
			renderError(w, r, common.ErrorNotFound(err))
			return
		}
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	affected := result.AffectedStudentIDs
	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.broadcastPickupScheduleChanged(tenantID, 0)
		for _, studentID := range affected {
			rs.wakeChildGuardians(tenantID, studentID)
		}
	})
	common.Respond(w, r, http.StatusOK, result, "Bulk pickup schedules upserted successfully")
}

// createStudentPickupException handles POST /students/{id}/pickup-exceptions
func (rs *Resource) createStudentPickupException(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupWriteAccess(w, r, "create pickup exceptions")
	if student == nil {
		return
	}

	req := &PickupExceptionRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	staffID, err := rs.getStaffIDFromJWT(r)
	if err != nil {
		renderError(w, r, common.ErrorForbidden(err))
		return
	}

	exceptionDate, _ := timezone.ParseDate(req.ExceptionDate)
	var pickupTime *time.Time
	if req.PickupTime != nil && *req.PickupTime != "" {
		parsed, _ := parseTimeOnly(*req.PickupTime)
		pickupTime = &parsed
	}

	tenantID := tenant.FromContext(r.Context())
	exception, err := rs.PickupScheduleService.CreateOrReclaimException(
		r.Context(), student.ID, exceptionDate, pickupTime, req.Reason, staffID,
		func() (int64, error) { return rs.getStaffIDFromJWT(r) },
	)
	if err != nil {
		renderExceptionWriteError(w, r, err)
		return
	}

	// Wake the child's guardians so an open parents-app tab reflects the new
	// pickup override on the "Heute" tile live. Defer to the OUTER request tx's
	// commit: the service write runs in a nested tx that only REUSES the tx opened
	// by TenantTxMiddleware and has NOT committed on return, so a woken client
	// would otherwise refetch the pre-commit snapshot — or be woken for a write a
	// later 5xx rolls back (#1725 review). broadcastStudentUpdated mirrors the
	// manual partial-absence handlers: a pulled-forward pickup time may have
	// auto-excused the child's later blocks (#2360), so student views refetch.
	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.wakeChildGuardians(tenantID, student.ID)
		rs.broadcastPickupScheduleChanged(tenantID, student.ID)
		rs.broadcastStudentUpdated(tenantID, student.ID)
	})

	common.Respond(w, r, http.StatusCreated, mapExceptionToResponse(exception), "Pickup exception created successfully")
}

// updateStudentPickupException handles PUT /students/{id}/pickup-exceptions/{exceptionId}
func (rs *Resource) updateStudentPickupException(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupWriteAccess(w, r, "update pickup exceptions")
	if student == nil {
		return
	}

	exceptionID, ok := parseEntityID(w, r, "exceptionId", "exception")
	if !ok {
		return
	}

	existingException := rs.verifyExceptionOwnership(w, r, exceptionID, student.ID)
	if existingException == nil {
		return
	}

	req := &PickupExceptionRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// existingException was the ownership pre-check; the locked re-read inside the
	// service is authoritative for the source/author decision.
	exceptionDate, _ := timezone.ParseDate(req.ExceptionDate)
	var pickupTime *time.Time
	if req.PickupTime != nil && *req.PickupTime != "" {
		parsed, _ := parseTimeOnly(*req.PickupTime)
		pickupTime = &parsed
	}

	tenantID := tenant.FromContext(r.Context())
	exception, err := rs.PickupScheduleService.UpdateException(
		r.Context(), exceptionID, student.ID, exceptionDate, req.Reason, pickupTime, req.ClearPickupTime,
		func() (int64, error) { return rs.getStaffIDFromJWT(r) },
	)
	if err != nil {
		renderExceptionWriteError(w, r, err)
		return
	}

	// Wake the child's guardians so the "Heute" pickup tile reflects the edited
	// override live; defer to the outer request tx's commit (see the create path
	// above — the service write runs in a nested tx, not committed on return)
	// (#1725 review). broadcastStudentUpdated: the edit may have applied or
	// released an auto excusal on the child's blocks (#2360).
	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.wakeChildGuardians(tenantID, student.ID)
		rs.broadcastPickupScheduleChanged(tenantID, student.ID)
		rs.broadcastStudentUpdated(tenantID, student.ID)
	})

	common.Respond(w, r, http.StatusOK, mapExceptionToResponse(exception), "Pickup exception updated successfully")
}

// deleteStudentPickupException handles DELETE /students/{id}/pickup-exceptions/{exceptionId}
func (rs *Resource) deleteStudentPickupException(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupWriteAccess(w, r, "delete pickup exceptions")
	if student == nil {
		return
	}

	exceptionID, ok := parseEntityID(w, r, "exceptionId", "exception")
	if !ok {
		return
	}

	existingException := rs.verifyExceptionOwnership(w, r, exceptionID, student.ID)
	if existingException == nil {
		return
	}

	tenantID := tenant.FromContext(r.Context())
	if err := rs.PickupScheduleService.DeleteStudentPickupException(r.Context(), exceptionID, student.ID); err != nil {
		renderExceptionWriteError(w, r, err)
		return
	}

	// Wake the child's guardians so the "Heute" pickup tile drops the removed
	// override live; defer to the outer request transaction's commit. The
	// native delete joins that transaction (#1725 review).
	// broadcastStudentUpdated: the delete may have released auto-excused
	// blocks back to expected (#2360).
	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.wakeChildGuardians(tenantID, student.ID)
		rs.broadcastPickupScheduleChanged(tenantID, student.ID)
		rs.broadcastStudentUpdated(tenantID, student.ID)
	})

	common.Respond(w, r, http.StatusOK, nil, "Pickup exception deleted successfully")
}

// createStudentPickupNote handles POST /students/{id}/pickup-notes
func (rs *Resource) createStudentPickupNote(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupWriteAccess(w, r, "create pickup notes")
	if student == nil {
		return
	}

	req := &PickupNoteRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	staffID, err := rs.getStaffIDFromJWT(r)
	if err != nil {
		renderError(w, r, common.ErrorForbidden(err))
		return
	}

	note := req.toModel(student.ID, staffID)

	tenantID := tenant.FromContext(r.Context())
	if err := withinTenant(r.Context(), tenantID, func(ctx context.Context) error {
		return rs.PickupScheduleService.CreateStudentPickupNote(ctx, note)
	}); err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// A day note rides along in the same pickup payload the detail header and
	// the Betreuungszeiten editor render, so it goes stale the same way a time
	// does — after commit, for the same nested-tx reason as above.
	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.broadcastPickupScheduleChanged(tenantID, student.ID)
	})

	common.Respond(w, r, http.StatusCreated, mapNoteToResponse(note), "Pickup note created successfully")
}

// replaceStudentWeekdayPickupNotes handles PUT /students/{id}/pickup-notes.
// All writes share the request transaction: a failed create, update, or delete
// leaves the recurring notes exactly as they were before the request.
func (rs *Resource) replaceStudentWeekdayPickupNotes(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupWriteAccess(w, r, "replace pickup notes")
	if student == nil {
		return
	}

	req := &WeekdayPickupNotesRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	staffID, err := rs.getStaffIDFromJWT(r)
	if err != nil {
		renderError(w, r, common.ErrorForbidden(err))
		return
	}

	if rs.WeekdayPickupNotes == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("weekday pickup notes are not configured")))
		return
	}
	notes := make(map[int]string, len(req.Notes))
	for _, note := range req.Notes {
		notes[note.Weekday] = note.Content
	}
	if err := rs.WeekdayPickupNotes.ReplaceWeekdayPickupNotes(r.Context(), student.ID, staffID, notes); err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.broadcastPickupScheduleChanged(tenantID, student.ID)
	})
	common.Respond(w, r, http.StatusOK, nil, "Pickup notes updated successfully")
}

// updateStudentPickupNote handles PUT /students/{id}/pickup-notes/{noteId}
func (rs *Resource) updateStudentPickupNote(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupWriteAccess(w, r, "update pickup notes")
	if student == nil {
		return
	}

	noteID, ok := parseEntityID(w, r, "noteId", "note")
	if !ok {
		return
	}

	existingNote := rs.verifyNoteOwnership(w, r, noteID, student.ID)
	if existingNote == nil {
		return
	}

	req := &PickupNoteRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	note := req.toModel(student.ID, existingNote.CreatedBy) // Preserve original creator
	note.ID = noteID
	note.CreatedAt = existingNote.CreatedAt // Preserve original creation timestamp
	note.SetTenantID(existingNote.TenantID)

	tenantID := tenant.FromContext(r.Context())
	if err := withinTenant(r.Context(), tenantID, func(ctx context.Context) error {
		return rs.PickupScheduleService.UpdateStudentPickupNote(ctx, note)
	}); err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.broadcastPickupScheduleChanged(tenantID, student.ID)
	})

	common.Respond(w, r, http.StatusOK, mapNoteToResponse(note), "Pickup note updated successfully")
}

// deleteStudentPickupNote handles DELETE /students/{id}/pickup-notes/{noteId}
func (rs *Resource) deleteStudentPickupNote(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupWriteAccess(w, r, "delete pickup notes")
	if student == nil {
		return
	}

	noteID, ok := parseEntityID(w, r, "noteId", "note")
	if !ok {
		return
	}

	existingNote := rs.verifyNoteOwnership(w, r, noteID, student.ID)
	if existingNote == nil {
		return
	}

	tenantID := tenant.FromContext(r.Context())
	if err := withinTenant(r.Context(), tenantID, func(ctx context.Context) error {
		return rs.PickupScheduleService.DeleteStudentPickupNote(ctx, noteID)
	}); err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.broadcastPickupScheduleChanged(tenantID, student.ID)
	})

	common.Respond(w, r, http.StatusOK, nil, "Pickup note deleted successfully")
}

// BulkPickupTimeRequest keeps the domain-specific public name while sharing
// validation and binding with arrival bulk effective-time requests.
type BulkPickupTimeRequest = BulkEffectiveTimeRequest

// BulkDayNoteResponse represents a single day note in bulk pickup time responses
type BulkDayNoteResponse struct {
	ID      int64  `json:"id"`
	Content string `json:"content"`
}

// BulkPickupTimeResponse represents pickup time data for a single student
type BulkPickupTimeResponse struct {
	StudentID   int64                 `json:"student_id"`
	Date        string                `json:"date"`
	WeekdayName string                `json:"weekday_name"`
	PickupTime  *string               `json:"pickup_time,omitempty"` // HH:MM format or null
	IsException bool                  `json:"is_exception"`
	Notes       string                `json:"notes,omitempty"`
	DayNotes    []BulkDayNoteResponse `json:"day_notes,omitempty"`
}

// getBulkPickupTimes handles POST /students/pickup-times/bulk
// Returns effective pickup times for multiple students on a given date
func (rs *Resource) getBulkPickupTimes(w http.ResponseWriter, r *http.Request) {
	handleBulkEffectiveTimes(
		rs,
		w,
		r,
		"Bulk pickup times retrieved successfully",
		rs.PickupScheduleService.GetBulkEffectivePickupTimesForDate,
		mapBulkPickupTimeResponse,
	)
}

func mapBulkPickupTimeResponse(
	studentID int64,
	effectiveTime *careplan.EffectivePickupTime,
) BulkPickupTimeResponse {
	response := BulkPickupTimeResponse{
		StudentID:   studentID,
		Date:        effectiveTime.Date.Format(dateFormatISO),
		WeekdayName: effectiveTime.WeekdayName,
		IsException: effectiveTime.IsException,
		Notes:       effectiveTime.Notes,
	}
	if effectiveTime.PickupTime != nil {
		formatted := effectiveTime.PickupTime.Format("15:04")
		response.PickupTime = &formatted
	}
	if len(effectiveTime.DayNotes) > 0 {
		response.DayNotes = make([]BulkDayNoteResponse, 0, len(effectiveTime.DayNotes))
		for _, note := range effectiveTime.DayNotes {
			response.DayNotes = append(response.DayNotes, BulkDayNoteResponse{
				ID:      note.ID,
				Content: note.Content,
			})
		}
	}
	return response
}

// filterAuthorizedStudentIDs filters the requested student IDs to only those
// the current user has read access to: admins and verified staff see all
// requested students, every other caller none (#2329).
func (rs *Resource) filterAuthorizedStudentIDs(r *http.Request, requestedIDs []int64) ([]int64, error) {
	if rs.determineStudentAccess(r).HasFullAccess() {
		return requestedIDs, nil
	}
	return []int64{}, nil
}

// ResetOfferingPickupRequest selects the weekday to reset onto the
// Angebots-Gehzeit (#2290).
type ResetOfferingPickupRequest struct {
	Weekday int    `json:"weekday"`
	Date    string `json:"date"`
	date    timezone.Date
}

func (r *ResetOfferingPickupRequest) Bind(_ *http.Request) error {
	if r.Weekday < weekdayMonday || r.Weekday > weekdayFriday {
		return errors.New("weekday must be between 1 (Monday) and 5 (Friday)")
	}
	date, err := timezone.ParseDate(strings.TrimSpace(r.Date))
	if err != nil {
		return fmt.Errorf("date must be YYYY-MM-DD: %w", err)
	}
	if int(date.Weekday()) != r.Weekday {
		return errors.New("date does not match weekday")
	}
	r.date = date
	return nil
}

// resetStudentPickupToOffering handles
// POST /students/{id}/pickup-schedules/reset-offering: the weekday's Gehzeit
// goes back to the Angebots-Gehzeit that applies on the requested date.
func (rs *Resource) resetStudentPickupToOffering(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupWriteAccess(w, r, "reset pickup schedule to offering")
	if student == nil {
		return
	}
	svc := rs.OfferingPickupTimes
	if svc == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("offering pickup service not configured")))
		return
	}
	req := &ResetOfferingPickupRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	tenantID := tenant.FromContext(r.Context())
	if err := withinTenant(r.Context(), tenantID, func(ctx context.Context) error {
		return svc.ResetStudentPickupDayToOffering(ctx, student.ID, req.date)
	}); err != nil {
		renderError(w, r, pickupResetErrorRenderer(err))
		return
	}
	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.wakeChildGuardians(tenantID, student.ID)
		rs.broadcastPickupScheduleChanged(tenantID, student.ID)
	})
	data, err := rs.PickupScheduleService.GetStudentPickupData(r.Context(), student.ID)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, buildPickupDataResponse(data), "Pickup schedule reset to offering")
}
