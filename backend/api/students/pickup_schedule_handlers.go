package students

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// PickupScheduleResponse represents a pickup schedule in API responses
type PickupScheduleResponse struct {
	ID          int64   `json:"id"`
	StudentID   int64   `json:"student_id"`
	Weekday     int     `json:"weekday"`
	WeekdayName string  `json:"weekday_name"`
	PickupTime  string  `json:"pickup_time"` // HH:MM format
	Notes       *string `json:"notes,omitempty"`
	// Source marks the row's provenance: "staff" (manually maintained) or
	// "care_offering" (booking-derived Angebots-Gehzeit).
	Source           string  `json:"source"`
	CareOfferingID   *string `json:"care_offering_id,omitempty"`
	CareOfferingName string  `json:"care_offering_name,omitempty"`
	CreatedBy        int64   `json:"created_by"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

// PickupExceptionResponse represents a pickup exception in API responses
type PickupExceptionResponse struct {
	ID            int64   `json:"id"`
	StudentID     int64   `json:"student_id"`
	ExceptionDate string  `json:"exception_date"` // YYYY-MM-DD format
	PickupTime    *string `json:"pickup_time,omitempty"`
	Reason        *string `json:"reason,omitempty"`
	Source        string  `json:"source"` // "staff" or "guardian" (parent-set)
	// ExcusedFrom (HH:MM) is the partial-absence cutoff carried by this
	// exception; ExcusedAuto marks it as derived from a pulled-forward pickup
	// time (#2360) rather than a manual staff decision.
	ExcusedFrom *string `json:"excused_from,omitempty"`
	ExcusedAuto bool    `json:"excused_auto"`
	CreatedBy   int64   `json:"created_by"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
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
	Schedules          []PickupScheduleResponse      `json:"schedules"`
	EffectiveSchedules []DatedPickupScheduleResponse `json:"effective_schedules,omitempty"`
	Exceptions         []PickupExceptionResponse     `json:"exceptions"`
	Notes              []PickupNoteResponse          `json:"notes"`
}

type DatedPickupScheduleResponse struct {
	Date             string                  `json:"date"`
	Schedule         *PickupScheduleResponse `json:"schedule"`
	OfferingSchedule *PickupScheduleResponse `json:"offering_schedule"`
}

// PickupScheduleRequest represents a request to create/update a pickup schedule
type PickupScheduleRequest struct {
	Weekday    int     `json:"weekday"`
	PickupTime string  `json:"pickup_time"` // HH:MM format
	Notes      *string `json:"notes,omitempty"`
}

// BulkPickupScheduleRequest represents a request to update all weekly schedules
type BulkPickupScheduleRequest struct {
	Schedules     []PickupScheduleRequest `json:"schedules"`
	EffectiveDate *timezone.Date          `json:"effective_date,omitempty"`
}

type BulkPickupSchedulePatchRequest struct {
	StudentIDs         []int64                               `json:"student_ids"`
	Schedules          []scheduleService.PickupScheduleInput `json:"schedules"`
	ConfirmedException bool                                  `json:"confirmed_exception"`
}

// PickupExceptionRequest represents a request to create/update a pickup exception
type PickupExceptionRequest struct {
	ExceptionDate   string  `json:"exception_date"` // YYYY-MM-DD format
	PickupTime      *string `json:"pickup_time,omitempty"`
	ClearPickupTime bool    `json:"clear_pickup_time,omitempty"`
	Reason          *string `json:"reason,omitempty"`
}

// PickupNoteRequest represents a request to create/update a pickup note
type PickupNoteRequest struct {
	NoteDate string `json:"note_date"` // YYYY-MM-DD format
	Content  string `json:"content"`
}

// Bind implements render.Binder
func (r *PickupNoteRequest) Bind(_ *http.Request) error {
	return validateCareNoteRequest(r.NoteDate, r.Content)
}

// Bind implements render.Binder
func (r *PickupScheduleRequest) Bind(_ *http.Request) error {
	return validateCareScheduleItem(careScheduleItem{
		Weekday: r.Weekday,
		Time:    r.PickupTime,
		Notes:   r.Notes,
	}, "pickup_time")
}

// Bind implements render.Binder
func (r *BulkPickupScheduleRequest) Bind(_ *http.Request) error {
	return validatePickupScheduleItems(r.Schedules)
}

func (r *BulkPickupSchedulePatchRequest) Bind(_ *http.Request) error {
	if len(r.StudentIDs) == 0 {
		return errors.New("student_ids array cannot be empty")
	}
	if len(r.StudentIDs) > 500 {
		return errors.New("student_ids array cannot exceed 500 items")
	}
	seenIDs := make(map[int64]struct{}, len(r.StudentIDs))
	for _, id := range r.StudentIDs {
		if id <= 0 {
			return errors.New("student_ids must be positive")
		}
		if _, exists := seenIDs[id]; exists {
			return errors.New("duplicate student_ids are not allowed")
		}
		seenIDs[id] = struct{}{}
	}
	items := make([]PickupScheduleRequest, 0, len(r.Schedules))
	for _, schedule := range r.Schedules {
		items = append(items, PickupScheduleRequest{Weekday: schedule.Weekday, PickupTime: schedule.PickupTime})
	}
	return validatePickupScheduleItems(items)
}

// validatePickupScheduleItems validates weekday range, uniqueness, time format
// and notes length for a set of weekly pickup schedule items. Shared by the
// bulk-update endpoint and the atomic create-student flow.
func validatePickupScheduleItems(items []PickupScheduleRequest) error {
	return validateCareScheduleItems(items, "pickup_time", func(item PickupScheduleRequest) careScheduleItem {
		return careScheduleItem{
			Weekday: item.Weekday,
			Time:    item.PickupTime,
			Notes:   item.Notes,
		}
	})
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
	return validateCareExceptionRequest(r.ExceptionDate, r.PickupTime, r.Reason, "pickup_time")
}

// mapScheduleToResponse converts a schedule model to API response
func mapScheduleToResponse(s *schedule.StudentPickupSchedule) PickupScheduleResponse {
	resp := PickupScheduleResponse{
		ID:               s.ID,
		StudentID:        s.StudentID,
		Weekday:          s.Weekday,
		WeekdayName:      s.GetWeekdayName(),
		PickupTime:       s.PickupTime.Format("15:04"),
		Notes:            s.Notes,
		Source:           s.Source,
		CareOfferingName: s.CareOfferingName,
		CreatedBy:        s.CreatedBy,
		CreatedAt:        s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        s.UpdatedAt.Format(time.RFC3339),
	}
	if resp.Source == "" {
		resp.Source = schedule.PickupScheduleSourceStaff
	}
	if s.CareOfferingID != nil {
		id := strconv.FormatInt(*s.CareOfferingID, 10)
		resp.CareOfferingID = &id
	}
	return resp
}

// mapExceptionToResponse converts an exception model to API response
func mapExceptionToResponse(e *schedule.StudentPickupException) PickupExceptionResponse {
	resp := PickupExceptionResponse{
		ID:            e.ID,
		StudentID:     e.StudentID,
		ExceptionDate: e.ExceptionDate.Format(dateFormatISO),
		Reason:        e.Reason,
		Source:        e.Source,
		ExcusedAuto:   e.ExcusedAuto,
		CreatedBy:     e.CreatedBy,
		CreatedAt:     e.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     e.UpdatedAt.Format(time.RFC3339),
	}
	if e.PickupTime != nil {
		formatted := e.PickupTime.Format("15:04")
		resp.PickupTime = &formatted
	}
	if e.ExcusedFrom != nil {
		formatted := timezone.WallClock(*e.ExcusedFrom).Format("15:04")
		resp.ExcusedFrom = &formatted
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
	return verifyCareOwnership(
		w,
		r,
		exceptionID,
		studentID,
		"pickup exception not found",
		"exception does not belong to this student",
		rs.PickupScheduleService.GetStudentPickupExceptionByID,
		func(exception *schedule.StudentPickupException) int64 { return exception.StudentID },
	)
}

// verifyNoteOwnership checks that a note exists and belongs to the given student.
// Returns the note if valid, or writes an error response and returns nil.
func (rs *Resource) verifyNoteOwnership(w http.ResponseWriter, r *http.Request, noteID, studentID int64) *schedule.StudentPickupNote {
	return verifyCareOwnership(
		w,
		r,
		noteID,
		studentID,
		"pickup note not found",
		"note does not belong to this student",
		rs.PickupScheduleService.GetStudentPickupNoteByID,
		func(note *schedule.StudentPickupNote) int64 { return note.StudentID },
	)
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

// requirePickupReadAccess parses the student from URL params and checks read
// access: admins and verified staff pass, guest/guardian accounts do not
// (#2329).
// Returns the student on success or writes an error response and returns nil.
func (rs *Resource) requirePickupReadAccess(w http.ResponseWriter, r *http.Request) *users.Student {
	return rs.requireCareReadAccess(w, r, "pickup")
}

// requirePickupWriteAccess parses the student from URL params and verifies full access.
// Used for write operations (create, update, delete) that require supervisor/admin access.
// Returns the student on success or writes an error response and returns nil.
func (rs *Resource) requirePickupWriteAccess(w http.ResponseWriter, r *http.Request, action string) *users.Student {
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
	var data *scheduleService.StudentPickupData
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
		return timezone.Date{}, timezone.Date{}, false, nil
	}
	if fromRaw == "" || toRaw == "" {
		return timezone.Date{}, timezone.Date{}, false, errors.New("from and to must be provided together")
	}
	from, err := timezone.ParseDate(fromRaw)
	if err != nil {
		return timezone.Date{}, timezone.Date{}, false, fmt.Errorf("invalid from date: %w", err)
	}
	to, err := timezone.ParseDate(toRaw)
	if err != nil {
		return timezone.Date{}, timezone.Date{}, false, fmt.Errorf("invalid to date: %w", err)
	}
	if to.Before(from) || to.After(from.AddDays(31)) {
		return timezone.Date{}, timezone.Date{}, false, errors.New("pickup schedule date range must span at most 32 days")
	}
	return from, to, true, nil
}

// buildPickupDataResponse converts service pickup data to API response
func buildPickupDataResponse(data *scheduleService.StudentPickupData) PickupDataResponse {
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
	if err := tenant.WithTenantTx(r.Context(), rs.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		date := timezone.TodayDate()
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
		r.Context(), enrollmentService.PickupAdjustmentBulkInput{
			StudentIDs: req.StudentIDs, Schedules: req.Schedules,
			ConfirmedException: req.ConfirmedException, CreatedByStaffID: staffID,
			ActorAccountID: int64(claims.ID),
			Authorize: func(ctx context.Context, student *users.Student) (bool, error) {
				return canUpdateStudent(ctx, permissions, student, rs.UserContextService)
			},
		},
	)
	if err != nil {
		if errors.Is(err, enrollmentService.ErrPickupAdjustmentBulkConfirmation) {
			renderError(w, r, common.ErrorInvalidRequestWithCode(
				err, "pickup.bulk_exception_confirmation_required",
			))
			return
		}
		if errors.Is(err, scheduleService.ErrBulkStudentUnauthorized) {
			renderError(w, r, common.ErrorForbidden(err))
			return
		}
		if errors.Is(err, scheduleService.ErrBulkStudentNotFound) {
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
	effectiveTime *scheduleService.EffectivePickupTime,
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
	if r.Weekday < schedule.WeekdayMonday || r.Weekday > schedule.WeekdayFriday {
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
	svc, ok := rs.EnrollmentDecision.(enrollmentService.OfferingPickupTimeService)
	if rs.EnrollmentDecision == nil || !ok {
		renderError(w, r, common.ErrorInternalServer(errors.New("offering pickup service not configured")))
		return
	}
	req := &ResetOfferingPickupRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
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
