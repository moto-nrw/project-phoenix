package students

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// dateFormatISO is the standard date format (YYYY-MM-DD) used for pickup schedules.
const dateFormatISO = "2006-01-02"

// parseTimeOnly parses a time string (HH:MM) and returns a time.Time with a valid reference date.
// PostgreSQL TIME columns require a valid date, so we use 2000-01-01 as reference.
func parseTimeOnly(timeStr string) (time.Time, error) {
	return time.Parse("2006-01-02 15:04", "2000-01-01 "+timeStr)
}

// PickupScheduleResponse represents a pickup schedule in API responses
type PickupScheduleResponse struct {
	ID          int64   `json:"id"`
	StudentID   int64   `json:"student_id"`
	Weekday     int     `json:"weekday"`
	WeekdayName string  `json:"weekday_name"`
	PickupTime  string  `json:"pickup_time"` // HH:MM format
	Notes       *string `json:"notes,omitempty"`
	CreatedBy   int64   `json:"created_by"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// PickupExceptionResponse represents a pickup exception in API responses
type PickupExceptionResponse struct {
	ID            int64   `json:"id"`
	StudentID     int64   `json:"student_id"`
	ExceptionDate string  `json:"exception_date"` // YYYY-MM-DD format
	PickupTime    *string `json:"pickup_time,omitempty"`
	Reason        *string `json:"reason,omitempty"`
	Source        string  `json:"source"` // "staff" or "guardian" (parent-set)
	CreatedBy     int64   `json:"created_by"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

// PickupNoteResponse represents a pickup note in API responses
type PickupNoteResponse struct {
	ID        int64  `json:"id"`
	StudentID int64  `json:"student_id"`
	NoteDate  string `json:"note_date"` // YYYY-MM-DD format
	Content   string `json:"content"`
	CreatedBy int64  `json:"created_by"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// PickupDataResponse represents combined pickup data
type PickupDataResponse struct {
	Schedules  []PickupScheduleResponse  `json:"schedules"`
	Exceptions []PickupExceptionResponse `json:"exceptions"`
	Notes      []PickupNoteResponse      `json:"notes"`
}

// PickupScheduleRequest represents a request to create/update a pickup schedule
type PickupScheduleRequest struct {
	Weekday    int     `json:"weekday"`
	PickupTime string  `json:"pickup_time"` // HH:MM format
	Notes      *string `json:"notes,omitempty"`
}

// BulkPickupScheduleRequest represents a request to update all weekly schedules
type BulkPickupScheduleRequest struct {
	Schedules []PickupScheduleRequest `json:"schedules"`
}

// PickupExceptionRequest represents a request to create/update a pickup exception
type PickupExceptionRequest struct {
	ExceptionDate string  `json:"exception_date"` // YYYY-MM-DD format
	PickupTime    *string `json:"pickup_time,omitempty"`
	Reason        *string `json:"reason,omitempty"`
}

// PickupNoteRequest represents a request to create/update a pickup note
type PickupNoteRequest struct {
	NoteDate string `json:"note_date"` // YYYY-MM-DD format
	Content  string `json:"content"`
}

// Bind implements render.Binder
func (r *PickupNoteRequest) Bind(_ *http.Request) error {
	if r.NoteDate == "" {
		return errors.New("note_date is required")
	}
	if _, err := time.Parse(dateFormatISO, r.NoteDate); err != nil {
		return errors.New("invalid note_date format, expected YYYY-MM-DD")
	}
	if r.Content == "" {
		return errors.New("content is required")
	}
	if len(r.Content) > 500 {
		return errors.New("content cannot exceed 500 characters")
	}
	return nil
}

// Bind implements render.Binder
func (r *PickupScheduleRequest) Bind(_ *http.Request) error {
	if r.Weekday < schedule.WeekdayMonday || r.Weekday > schedule.WeekdayFriday {
		return errors.New("weekday must be between 1 (Monday) and 5 (Friday)")
	}
	if r.PickupTime == "" {
		return errors.New("pickup_time is required")
	}
	if _, err := time.Parse("15:04", r.PickupTime); err != nil {
		return errors.New("invalid pickup_time format, expected HH:MM")
	}
	if r.Notes != nil && len(*r.Notes) > 500 {
		return errors.New("notes cannot exceed 500 characters")
	}
	return nil
}

// Bind implements render.Binder
func (r *BulkPickupScheduleRequest) Bind(_ *http.Request) error {
	return validatePickupScheduleItems(r.Schedules)
}

// validatePickupScheduleItems validates weekday range, uniqueness, time format
// and notes length for a set of weekly pickup schedule items. Shared by the
// bulk-update endpoint and the atomic create-student flow.
func validatePickupScheduleItems(items []PickupScheduleRequest) error {
	seenWeekdays := make(map[int]bool)
	for i, s := range items {
		if s.Weekday < schedule.WeekdayMonday || s.Weekday > schedule.WeekdayFriday {
			return fmt.Errorf("schedule %d: weekday must be between 1 (Monday) and 5 (Friday)", i)
		}
		if seenWeekdays[s.Weekday] {
			return fmt.Errorf("schedule %d: duplicate weekday %d", i, s.Weekday)
		}
		seenWeekdays[s.Weekday] = true
		if s.PickupTime == "" {
			return fmt.Errorf("schedule %d: pickup_time is required", i)
		}
		if _, err := time.Parse("15:04", s.PickupTime); err != nil {
			return fmt.Errorf("schedule %d: invalid pickup_time format, expected HH:MM", i)
		}
		if s.Notes != nil && len(*s.Notes) > 500 {
			return fmt.Errorf("schedule %d: notes cannot exceed 500 characters", i)
		}
	}
	return nil
}

// toPickupScheduleModels maps request items onto schedule models stamped with
// the student and acting staff. Shared by the bulk-update endpoint and the
// create-student flow.
//
// Callers MUST run validatePickupScheduleItems first (both current callers do,
// via their Bind): the parse error below is intentionally discarded because the
// time format is already guaranteed valid at that point. Mapping unvalidated
// items here would silently persist a zero time.
func toPickupScheduleModels(items []PickupScheduleRequest, studentID, staffID int64) []*schedule.StudentPickupSchedule {
	schedules := make([]*schedule.StudentPickupSchedule, 0, len(items))
	for _, s := range items {
		pickupTime, _ := parseTimeOnly(s.PickupTime)
		schedules = append(schedules, &schedule.StudentPickupSchedule{
			StudentID:  studentID,
			Weekday:    s.Weekday,
			PickupTime: pickupTime,
			Notes:      s.Notes,
			CreatedBy:  staffID,
		})
	}
	return schedules
}

// Bind implements render.Binder
func (r *PickupExceptionRequest) Bind(_ *http.Request) error {
	if r.ExceptionDate == "" {
		return errors.New("exception_date is required")
	}
	if _, err := time.Parse(dateFormatISO, r.ExceptionDate); err != nil {
		return errors.New("invalid exception_date format, expected YYYY-MM-DD")
	}
	// pickup_time is optional (nil = absent/no pickup)
	// but if provided, it must be valid HH:MM format
	if r.PickupTime != nil && *r.PickupTime != "" {
		if _, err := time.Parse("15:04", *r.PickupTime); err != nil {
			return errors.New("invalid pickup_time format, expected HH:MM")
		}
	}
	if r.Reason != nil && len(*r.Reason) > 255 {
		return errors.New("reason cannot exceed 255 characters")
	}
	return nil
}

// mapScheduleToResponse converts a schedule model to API response
func mapScheduleToResponse(s *schedule.StudentPickupSchedule) PickupScheduleResponse {
	return PickupScheduleResponse{
		ID:          s.ID,
		StudentID:   s.StudentID,
		Weekday:     s.Weekday,
		WeekdayName: s.GetWeekdayName(),
		PickupTime:  s.PickupTime.Format("15:04"),
		Notes:       s.Notes,
		CreatedBy:   s.CreatedBy,
		CreatedAt:   s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   s.UpdatedAt.Format(time.RFC3339),
	}
}

// mapExceptionToResponse converts an exception model to API response
func mapExceptionToResponse(e *schedule.StudentPickupException) PickupExceptionResponse {
	resp := PickupExceptionResponse{
		ID:            e.ID,
		StudentID:     e.StudentID,
		ExceptionDate: e.ExceptionDate.Format(dateFormatISO),
		Reason:        e.Reason,
		Source:        e.Source,
		CreatedBy:     e.CreatedBy,
		CreatedAt:     e.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     e.UpdatedAt.Format(time.RFC3339),
	}
	if e.PickupTime != nil {
		formatted := e.PickupTime.Format("15:04")
		resp.PickupTime = &formatted
	}
	return resp
}

// mapNoteToResponse converts a note model to API response
func mapNoteToResponse(n *schedule.StudentPickupNote) PickupNoteResponse {
	return PickupNoteResponse{
		ID:        n.ID,
		StudentID: n.StudentID,
		NoteDate:  n.NoteDate.Format(dateFormatISO),
		Content:   n.Content,
		CreatedBy: n.CreatedBy,
		CreatedAt: n.CreatedAt.Format(time.RFC3339),
		UpdatedAt: n.UpdatedAt.Format(time.RFC3339),
	}
}

// verifyExceptionOwnership checks that an exception exists and belongs to the given student.
// Returns the exception if valid, or writes an error response and returns nil.
func (rs *Resource) verifyExceptionOwnership(w http.ResponseWriter, r *http.Request, exceptionID, studentID int64) *schedule.StudentPickupException {
	exception, err := rs.PickupScheduleService.GetStudentPickupExceptionByID(r.Context(), exceptionID)
	if err != nil || exception == nil {
		renderError(w, r, common.ErrorNotFound(errors.New("pickup exception not found")))
		return nil
	}
	if exception.StudentID != studentID {
		renderError(w, r, common.ErrorForbidden(errors.New("exception does not belong to this student")))
		return nil
	}
	return exception
}

// verifyNoteOwnership checks that a note exists and belongs to the given student.
// Returns the note if valid, or writes an error response and returns nil.
func (rs *Resource) verifyNoteOwnership(w http.ResponseWriter, r *http.Request, noteID, studentID int64) *schedule.StudentPickupNote {
	note, err := rs.PickupScheduleService.GetStudentPickupNoteByID(r.Context(), noteID)
	if err != nil || note == nil {
		renderError(w, r, common.ErrorNotFound(errors.New("pickup note not found")))
		return nil
	}
	if note.StudentID != studentID {
		renderError(w, r, common.ErrorForbidden(errors.New("note does not belong to this student")))
		return nil
	}
	return note
}

// getStaffIDFromJWT extracts the staff ID from JWT claims by looking up the person and staff
func (rs *Resource) getStaffIDFromJWT(r *http.Request) (int64, error) {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		return 0, errors.New("no valid JWT claims found")
	}

	// Get person from account ID
	person, err := rs.PersonService.FindByAccountID(r.Context(), int64(claims.ID))
	if err != nil || person == nil {
		return 0, errors.New("person not found for account")
	}

	// Get staff from person ID
	staff, err := rs.PersonService.GetStaffByPersonID(r.Context(), person.ID)
	if err != nil || staff == nil {
		return 0, errors.New("user is not a staff member")
	}

	return staff.ID, nil
}

// requirePickupReadAccess parses the student from URL params and checks read access.
// Respects the gdpr.student_data_scope setting: with group_supervisors_only only
// supervisors and admins can view pickup data; with all_staff any verified staff can.
// Returns the student on success or writes an error response and returns nil.
func (rs *Resource) requirePickupReadAccess(w http.ResponseWriter, r *http.Request) *users.Student {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return nil
	}
	if !rs.checkStudentReadAccess(r, student) {
		renderError(w, r, common.ErrorForbidden(fmt.Errorf("read access required to view pickup schedules")))
		return nil
	}
	return student
}

// requirePickupWriteAccess parses the student from URL params and verifies full access.
// Used for write operations (create, update, delete) that require supervisor/admin access.
// Returns the student on success or writes an error response and returns nil.
func (rs *Resource) requirePickupWriteAccess(w http.ResponseWriter, r *http.Request, action string) *users.Student {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return nil
	}
	if !rs.checkStudentFullAccess(r, student) {
		renderError(w, r, common.ErrorForbidden(fmt.Errorf("full access required to %s", action)))
		return nil
	}
	return student
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

// getStudentPickupSchedules handles GET /students/{id}/pickup-schedules
// Access is controlled by the gdpr.student_data_scope tenant setting.
func (rs *Resource) getStudentPickupSchedules(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupReadAccess(w, r)
	if student == nil {
		return
	}

	data, err := rs.PickupScheduleService.GetStudentPickupData(r.Context(), student.ID)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	response := buildPickupDataResponse(data)

	common.Respond(w, r, http.StatusOK, response, "Pickup schedules retrieved successfully")
}

// buildPickupDataResponse converts service pickup data to API response
func buildPickupDataResponse(data *scheduleService.StudentPickupData) PickupDataResponse {
	response := PickupDataResponse{
		Schedules:  make([]PickupScheduleResponse, 0, len(data.Schedules)),
		Exceptions: make([]PickupExceptionResponse, 0, len(data.Exceptions)),
		Notes:      make([]PickupNoteResponse, 0, len(data.Notes)),
	}

	for _, s := range data.Schedules {
		response.Schedules = append(response.Schedules, mapScheduleToResponse(s))
	}
	for _, e := range data.Exceptions {
		response.Exceptions = append(response.Exceptions, mapExceptionToResponse(e))
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
	// TENANT-WIDE broadcast: every staff client in the school receives it,
	// including those who under gdpr.student_data_scope=group_supervisors_only
	// may NOT read this child. A student_id in that raw SSE stream would leak
	// which children have pickup-plan activity to staff outside their scope.
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
	if err := tenant.WithTenantTx(r.Context(), rs.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.PickupScheduleService.UpsertBulkStudentPickupSchedules(ctx, student.ID, schedules)
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
	// later 5xx rolls back (#1725 review).
	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.wakeChildGuardians(tenantID, student.ID)
		rs.broadcastPickupScheduleChanged(tenantID, student.ID)
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
		r.Context(), exceptionID, student.ID, exceptionDate, req.Reason, pickupTime,
		func() (int64, error) { return rs.getStaffIDFromJWT(r) },
	)
	if err != nil {
		renderExceptionWriteError(w, r, err)
		return
	}

	// Wake the child's guardians so the "Heute" pickup tile reflects the edited
	// override live; defer to the outer request tx's commit (see the create path
	// above — the service write runs in a nested tx, not committed on return)
	// (#1725 review).
	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.wakeChildGuardians(tenantID, student.ID)
		rs.broadcastPickupScheduleChanged(tenantID, student.ID)
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
	if err := tenant.WithTenantTx(r.Context(), rs.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := scheduleService.LockCareExceptionDay(ctx, rs.DB, student.ID, existingException.ExceptionDate); err != nil {
			return err
		}
		freshException, err := rs.PickupScheduleService.GetStudentPickupExceptionByID(ctx, exceptionID)
		if err != nil {
			return err
		}
		if freshException == nil {
			return nil
		}
		if freshException.StudentID != student.ID {
			return ErrExceptionWrongStudent
		}
		return rs.PickupScheduleService.DeleteStudentPickupException(ctx, freshException.ID)
	}); err != nil {
		renderExceptionWriteError(w, r, err)
		return
	}

	// Wake the child's guardians so the "Heute" pickup tile drops the removed
	// override live; defer to the outer request tx's commit (nested handler
	// WithTenantTx is not committed on return) (#1725 review).
	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.wakeChildGuardians(tenantID, student.ID)
		rs.broadcastPickupScheduleChanged(tenantID, student.ID)
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

	noteDate, _ := timezone.ParseDate(req.NoteDate)
	note := &schedule.StudentPickupNote{
		StudentID: student.ID,
		NoteDate:  noteDate,
		Content:   req.Content,
		CreatedBy: staffID,
	}

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
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

	noteDate, _ := timezone.ParseDate(req.NoteDate)
	note := &schedule.StudentPickupNote{
		StudentID: student.ID,
		NoteDate:  noteDate,
		Content:   req.Content,
		CreatedBy: existingNote.CreatedBy, // Preserve original creator
	}
	note.ID = noteID
	note.CreatedAt = existingNote.CreatedAt // Preserve original creation timestamp
	note.SetTenantID(existingNote.TenantID)

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
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
	if err := tenant.WithTenantTx(r.Context(), rs.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
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

// BulkPickupTimeRequest represents a request to get pickup times for multiple students
type BulkPickupTimeRequest struct {
	StudentIDs []int64 `json:"student_ids"`
	Date       *string `json:"date,omitempty"` // Optional date in YYYY-MM-DD format, defaults to today
}

// Bind implements render.Binder
func (r *BulkPickupTimeRequest) Bind(_ *http.Request) error {
	if len(r.StudentIDs) == 0 {
		return errors.New("student_ids array cannot be empty")
	}
	if len(r.StudentIDs) > 500 {
		return errors.New("student_ids array cannot exceed 500 items")
	}
	if r.Date != nil && *r.Date != "" {
		if _, err := time.Parse(dateFormatISO, *r.Date); err != nil {
			return errors.New("invalid date format, expected YYYY-MM-DD")
		}
	}
	return nil
}

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
	req := &BulkPickupTimeRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Filter student IDs to only those the user has access to
	authorizedIDs, err := rs.filterAuthorizedStudentIDs(r, req.StudentIDs)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	if len(authorizedIDs) == 0 {
		// No authorized students - return empty result
		common.Respond(w, r, http.StatusOK, []BulkPickupTimeResponse{}, "Bulk pickup times retrieved successfully")
		return
	}

	// Determine the date to query
	date := timezone.TodayDate()
	if req.Date != nil && *req.Date != "" {
		parsedDate, _ := timezone.ParseDate(*req.Date) // Already validated in Bind
		date = parsedDate
	}

	// Use bulk service method (O(2) queries instead of O(N))
	pickupTimes, err := rs.PickupScheduleService.GetBulkEffectivePickupTimesForDate(r.Context(), authorizedIDs, date)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Convert to response format
	responses := make([]BulkPickupTimeResponse, 0, len(pickupTimes))
	for studentID, ept := range pickupTimes {
		resp := BulkPickupTimeResponse{
			StudentID:   studentID,
			Date:        ept.Date.Format(dateFormatISO),
			WeekdayName: ept.WeekdayName,
			IsException: ept.IsException,
			Notes:       ept.Notes,
		}
		if ept.PickupTime != nil {
			formatted := ept.PickupTime.Format("15:04")
			resp.PickupTime = &formatted
		}
		if len(ept.DayNotes) > 0 {
			resp.DayNotes = make([]BulkDayNoteResponse, 0, len(ept.DayNotes))
			for _, note := range ept.DayNotes {
				resp.DayNotes = append(resp.DayNotes, BulkDayNoteResponse{
					ID:      note.ID,
					Content: note.Content,
				})
			}
		}
		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Bulk pickup times retrieved successfully")
}

// filterAuthorizedStudentIDs filters the requested student IDs to only those
// the current user has read access to. Respects the gdpr.student_data_scope
// setting: admins see all, all_staff scope lets any verified staff see all,
// otherwise only students in the user's supervised groups are returned.
func (rs *Resource) filterAuthorizedStudentIDs(r *http.Request, requestedIDs []int64) ([]int64, error) {
	accessCtx := rs.determineStudentAccess(r)

	// Admins and all_staff scope see all requested students
	if accessCtx.IsAdmin || accessCtx.AllStaffScope {
		return requestedIDs, nil
	}

	// No supervised groups → no access
	if len(accessCtx.MyGroupIDs) == 0 {
		return []int64{}, nil
	}

	// Extract group IDs from access context
	groupIDs := make([]int64, 0, len(accessCtx.MyGroupIDs))
	for gid := range accessCtx.MyGroupIDs {
		groupIDs = append(groupIDs, gid)
	}

	// Get all students in these groups
	students, err := rs.PersonService.GetStudentsByGroupIDs(r.Context(), groupIDs)
	if err != nil {
		return nil, err
	}

	// Build set of authorized student IDs for O(1) lookup
	authorizedSet := make(map[int64]struct{}, len(students))
	for _, student := range students {
		authorizedSet[student.ID] = struct{}{}
	}

	// Filter requested IDs to only authorized ones
	filtered := make([]int64, 0, len(requestedIDs))
	for _, id := range requestedIDs {
		if _, ok := authorizedSet[id]; ok {
			filtered = append(filtered, id)
		}
	}

	return filtered, nil
}
