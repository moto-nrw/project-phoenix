package students

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
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

// ArrivalScheduleResponse represents an arrival schedule in API responses
type ArrivalScheduleResponse struct {
	ID              int64   `json:"id"`
	StudentID       int64   `json:"student_id"`
	Weekday         int     `json:"weekday"`
	WeekdayName     string  `json:"weekday_name"`
	ExpectedArrival string  `json:"expected_arrival"` // HH:MM, empty when no time is known yet
	Notes           *string `json:"notes,omitempty"`
	// Source says where ExpectedArrival comes from: "class_schedule" = the
	// child's class timetable, "staff" = a deliberate per-child deviation.
	// SourceClass names the class in the first case (#2414).
	Source      string `json:"source,omitempty"`
	SourceClass string `json:"source_class,omitempty"`
	CreatedBy   int64  `json:"created_by"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// ArrivalExceptionResponse represents an arrival exception in API responses
type ArrivalExceptionResponse struct {
	ID              int64   `json:"id"`
	StudentID       int64   `json:"student_id"`
	ExceptionDate   string  `json:"exception_date"`             // YYYY-MM-DD format
	ExpectedArrival *string `json:"expected_arrival,omitempty"` // HH:MM or null
	Reason          *string `json:"reason,omitempty"`
	Source          string  `json:"source"` // "staff" or "guardian" (parent-set)
	CreatedBy       int64   `json:"created_by"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

// ArrivalNoteResponse represents an arrival note in API responses
type ArrivalNoteResponse struct {
	ID        int64  `json:"id"`
	StudentID int64  `json:"student_id"`
	NoteDate  string `json:"note_date"` // YYYY-MM-DD format
	Content   string `json:"content"`
	CreatedBy int64  `json:"created_by"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ArrivalDataResponse represents combined arrival data
type ArrivalDataResponse struct {
	Schedules  []ArrivalScheduleResponse  `json:"schedules"`
	Exceptions []ArrivalExceptionResponse `json:"exceptions"`
	Notes      []ArrivalNoteResponse      `json:"notes"`
}

// BulkArrivalScheduleRequest represents a request to update all weekly arrival schedules for a student
type BulkArrivalScheduleRequest struct {
	Schedules []ArrivalScheduleRequestItem `json:"schedules"`
}

// ArrivalScheduleRequestItem represents a single arrival schedule entry in a request
type ArrivalScheduleRequestItem struct {
	Weekday         int     `json:"weekday"`
	ExpectedArrival string  `json:"expected_arrival"` // HH:MM format
	Notes           *string `json:"notes,omitempty"`
}

// ArrivalExceptionRequest represents a request to create/update an arrival exception
type ArrivalExceptionRequest struct {
	ExceptionDate        string  `json:"exception_date"`             // YYYY-MM-DD format
	ExpectedArrival      *string `json:"expected_arrival,omitempty"` // HH:MM or null (null = absent)
	ClearExpectedArrival bool    `json:"clear_expected_arrival,omitempty"`
	Reason               *string `json:"reason,omitempty"`
}

// ArrivalNoteRequest represents a request to create/update an arrival note
type ArrivalNoteRequest struct {
	NoteDate string `json:"note_date"` // YYYY-MM-DD format
	Content  string `json:"content"`
}

// BulkUpsertArrivalScheduleRequest represents a request to bulk upsert arrival
// schedules for exactly one filtered student cohort.
type BulkUpsertArrivalScheduleRequest struct {
	SchoolClass string                                 `json:"school_class"`
	GroupID     int64                                  `json:"group_id"`
	StudentIDs  []int64                                `json:"student_ids"`
	Schedules   []scheduleService.ArrivalScheduleInput `json:"schedules"`
}

// Bind implements render.Binder for ArrivalNoteRequest
func (r *ArrivalNoteRequest) Bind(_ *http.Request) error {
	return validateCareNoteRequest(r.NoteDate, r.Content)
}

// Bind implements render.Binder for BulkArrivalScheduleRequest
func (r *BulkArrivalScheduleRequest) Bind(_ *http.Request) error {
	return validateArrivalScheduleItems(r.Schedules)
}

// validateArrivalScheduleItems validates weekday range, uniqueness, time format
// and notes length for a set of weekly arrival schedule items. Shared by the
// bulk-update endpoint and the atomic create-student flow so both paths enforce
// the same rules.
func validateArrivalScheduleItems(items []ArrivalScheduleRequestItem) error {
	return validateCareScheduleItems(items, "expected_arrival", func(item ArrivalScheduleRequestItem) careScheduleItem {
		return careScheduleItem{
			Weekday:      item.Weekday,
			Time:         item.ExpectedArrival,
			Notes:        item.Notes,
			TimeOptional: true,
		}
	})
}

// toArrivalScheduleModels maps request items onto schedule models stamped with
// the student and acting staff. Shared by the bulk-update endpoint and the
// create-student flow.
//
// Callers MUST run validateArrivalScheduleItems first (both current callers do,
// via their Bind): the parse error below is intentionally discarded because the
// time format is already guaranteed valid at that point. An empty time parses
// to the zero value on purpose — that is the care day whose time comes from the
// class timetable (#2414).
func toArrivalScheduleModels(items []ArrivalScheduleRequestItem, studentID, staffID int64) []*schedule.StudentArrivalSchedule {
	schedules := make([]*schedule.StudentArrivalSchedule, 0, len(items))
	for _, s := range items {
		arrivalTime, _ := parseTimeOnly(s.ExpectedArrival)
		schedules = append(schedules, &schedule.StudentArrivalSchedule{
			StudentID:       studentID,
			Weekday:         s.Weekday,
			ExpectedArrival: arrivalTime,
			Notes:           s.Notes,
			CreatedBy:       staffID,
		})
	}
	return schedules
}

// Bind implements render.Binder for ArrivalExceptionRequest
func (r *ArrivalExceptionRequest) Bind(_ *http.Request) error {
	return validateCareExceptionRequest(
		r.ExceptionDate,
		r.ExpectedArrival,
		r.Reason,
		"expected_arrival",
	)
}

// Bind implements render.Binder for BulkUpsertArrivalScheduleRequest
func (r *BulkUpsertArrivalScheduleRequest) Bind(_ *http.Request) error {
	if err := validateBulkArrivalSelector(r); err != nil {
		return err
	}
	return validateBulkArrivalSchedules(r)
}

func validateBulkArrivalSelector(r *BulkUpsertArrivalScheduleRequest) error {
	hasSchoolClass := strings.TrimSpace(r.SchoolClass) != ""
	hasGroup := r.GroupID != 0
	hasStudents := len(r.StudentIDs) > 0
	selectorCount := 0
	for _, selected := range []bool{hasSchoolClass, hasGroup, hasStudents} {
		if selected {
			selectorCount++
		}
	}
	if selectorCount != 1 {
		return errors.New("exactly one filter is required: school_class, group_id, or student_ids")
	}
	if r.GroupID < 0 {
		return errors.New("group_id must be positive")
	}
	if len(r.StudentIDs) > 500 {
		return errors.New("student_ids array cannot exceed 500 items")
	}
	seenStudentIDs := make(map[int64]struct{}, len(r.StudentIDs))
	for _, id := range r.StudentIDs {
		if id <= 0 {
			return errors.New("student_ids must be positive")
		}
		if _, exists := seenStudentIDs[id]; exists {
			return errors.New("duplicate student_ids are not allowed")
		}
		seenStudentIDs[id] = struct{}{}
	}
	return nil
}

func validateBulkArrivalSchedules(r *BulkUpsertArrivalScheduleRequest) error {
	if len(r.Schedules) == 0 {
		return errors.New("schedules array cannot be empty")
	}
	seenWeekdays := make(map[int]bool)
	for i, s := range r.Schedules {
		if s.Weekday < schedule.WeekdayMonday || s.Weekday > schedule.WeekdayFriday {
			return fmt.Errorf("schedule %d: weekday must be between 1 (Monday) and 5 (Friday)", i)
		}
		if seenWeekdays[s.Weekday] {
			return fmt.Errorf("schedule %d: duplicate weekday %d", i, s.Weekday)
		}
		seenWeekdays[s.Weekday] = true
		// Only a class timetable can clear a time. Group and explicit-student
		// updates always write an own time.
		if s.ArrivalTime == "" {
			if strings.TrimSpace(r.SchoolClass) == "" {
				return fmt.Errorf("schedule %d: expected_arrival is required unless school_class is selected", i)
			}
			continue
		}
		if _, err := time.Parse("15:04", s.ArrivalTime); err != nil {
			return fmt.Errorf("schedule %d: invalid expected_arrival format, expected HH:MM", i)
		}
	}
	return nil
}

// mapArrivalScheduleToResponse converts an arrival schedule model to API response
func mapArrivalScheduleToResponse(s *schedule.StudentArrivalSchedule) ArrivalScheduleResponse {
	resp := ArrivalScheduleResponse{
		ID:          s.ID,
		StudentID:   s.StudentID,
		Weekday:     s.Weekday,
		WeekdayName: s.GetWeekdayName(),
		Notes:       s.Notes,
		Source:      s.Source,
		SourceClass: s.SourceClass,
		CreatedBy:   s.CreatedBy,
		CreatedAt:   s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   s.UpdatedAt.Format(time.RFC3339),
	}
	// A care day whose class has no time yet reports an empty string, never
	// midnight (#2414).
	if !s.ExpectedArrival.IsZero() {
		resp.ExpectedArrival = s.ExpectedArrival.Format("15:04")
	}
	return resp
}

// mapArrivalExceptionToResponse converts an arrival exception model to API response
func mapArrivalExceptionToResponse(e *schedule.StudentArrivalException) ArrivalExceptionResponse {
	resp := ArrivalExceptionResponse{
		ID:            e.ID,
		StudentID:     e.StudentID,
		ExceptionDate: e.ExceptionDate.Format(dateFormatISO),
		Reason:        e.Reason,
		Source:        e.Source,
		CreatedBy:     e.CreatedBy,
		CreatedAt:     e.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     e.UpdatedAt.Format(time.RFC3339),
	}
	if e.ExpectedArrival != nil {
		formatted := e.ExpectedArrival.Format("15:04")
		resp.ExpectedArrival = &formatted
	}
	return resp
}

// mapArrivalNoteToResponse converts an arrival note model to API response
func mapArrivalNoteToResponse(n *schedule.StudentArrivalNote) ArrivalNoteResponse {
	return ArrivalNoteResponse{
		ID:        n.ID,
		StudentID: n.StudentID,
		NoteDate:  n.NoteDate.Format(dateFormatISO),
		Content:   n.Content,
		CreatedBy: n.CreatedBy,
		CreatedAt: n.CreatedAt.Format(time.RFC3339),
		UpdatedAt: n.UpdatedAt.Format(time.RFC3339),
	}
}

// verifyArrivalExceptionOwnership checks that an arrival exception exists and belongs to the given student.
func (rs *Resource) verifyArrivalExceptionOwnership(w http.ResponseWriter, r *http.Request, exceptionID, studentID int64) *schedule.StudentArrivalException {
	return verifyCareOwnership(
		w,
		r,
		exceptionID,
		studentID,
		"arrival exception not found",
		"exception does not belong to this student",
		rs.ArrivalScheduleService.GetStudentArrivalExceptionByID,
		func(exception *schedule.StudentArrivalException) int64 { return exception.StudentID },
	)
}

// verifyArrivalNoteOwnership checks that an arrival note exists and belongs to the given student.
func (rs *Resource) verifyArrivalNoteOwnership(w http.ResponseWriter, r *http.Request, noteID, studentID int64) *schedule.StudentArrivalNote {
	return verifyCareOwnership(
		w,
		r,
		noteID,
		studentID,
		"arrival note not found",
		"note does not belong to this student",
		rs.ArrivalScheduleService.GetStudentArrivalNoteByID,
		func(note *schedule.StudentArrivalNote) int64 { return note.StudentID },
	)
}

// requireArrivalReadAccess parses the student from URL params and checks read access.
func (rs *Resource) requireArrivalReadAccess(w http.ResponseWriter, r *http.Request) *users.Student {
	return rs.requireCareReadAccess(w, r, "arrival")
}

// requireArrivalWriteAccess parses the student from URL params and verifies full access.
func (rs *Resource) requireArrivalWriteAccess(w http.ResponseWriter, r *http.Request, action string) *users.Student {
	return rs.requireCareWriteAccess(w, r, action)
}

// getStudentArrivalSchedules handles GET /students/{id}/arrival-schedules
func (rs *Resource) getStudentArrivalSchedules(w http.ResponseWriter, r *http.Request) {
	student := rs.requireArrivalReadAccess(w, r)
	if student == nil {
		return
	}

	data, err := rs.ArrivalScheduleService.GetStudentArrivalData(r.Context(), student.ID)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	response := buildArrivalDataResponse(data)
	common.Respond(w, r, http.StatusOK, response, "Arrival schedules retrieved successfully")
}

// buildArrivalDataResponse converts service arrival data to API response
func buildArrivalDataResponse(data *scheduleService.StudentArrivalData) ArrivalDataResponse {
	response := ArrivalDataResponse{
		Schedules:  make([]ArrivalScheduleResponse, 0, len(data.Schedules)),
		Exceptions: make([]ArrivalExceptionResponse, 0, len(data.Exceptions)),
		Notes:      make([]ArrivalNoteResponse, 0, len(data.Notes)),
	}

	for _, s := range data.Schedules {
		response.Schedules = append(response.Schedules, mapArrivalScheduleToResponse(s))
	}
	for _, e := range data.Exceptions {
		response.Exceptions = append(response.Exceptions, mapArrivalExceptionToResponse(e))
	}
	for _, n := range data.Notes {
		response.Notes = append(response.Notes, mapArrivalNoteToResponse(n))
	}

	return response
}

// broadcastArrivalScheduleChanged tells every open staff tab that a child's
// arrival plan changed.
//
// MUST be called from a tenant.RegisterAfterCommit hook, never inline. Handler
// writes run in a WithTenantTx that merely reuses the still-open
// TenantTxMiddleware transaction, so at handler return nothing is committed
// yet: a client woken inline refetches the PREVIOUS plan, and because this is
// the only invalidation the arrival caches get, nothing corrects it afterwards.
// A write that a later 5xx rolls back would leave every tab showing data that
// never existed. RegisterAfterCommit runs the callback immediately when no
// transaction is registered, so the hook is safe on every path.
//
// studentID is for the log line only — the event carries no student id (it is
// a tenant-wide staff broadcast; see the pickup sibling for the GDPR reasoning).
func (rs *Resource) broadcastArrivalScheduleChanged(studentID int64) {
	if rs.Broadcaster == nil {
		return
	}

	source := "manual"
	data := realtime.EventData{Source: &source}
	event := realtime.NewEvent(
		realtime.EventArrivalScheduleChanged,
		"",
		data,
	)
	if err := rs.Broadcaster.BroadcastToAll(event); err != nil && rs.Logger != nil {
		rs.Logger.Warn(
			"failed to broadcast arrival schedule change",
			"student_id", studentID,
			"error", err.Error(),
		)
	}
}

// updateStudentArrivalSchedules handles PUT /students/{id}/arrival-schedules
func (rs *Resource) updateStudentArrivalSchedules(w http.ResponseWriter, r *http.Request) {
	student := rs.requireArrivalWriteAccess(w, r, "update arrival schedules")
	if student == nil {
		return
	}

	req := &BulkArrivalScheduleRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	staffID, err := rs.getStaffIDFromJWT(r)
	if err != nil {
		renderError(w, r, common.ErrorForbidden(err))
		return
	}

	schedules := toArrivalScheduleModels(req.Schedules, student.ID, staffID)

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.ArrivalScheduleService.UpsertBulkStudentArrivalSchedules(ctx, student.ID, schedules)
	}); err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	data, err := rs.ArrivalScheduleService.GetStudentArrivalData(r.Context(), student.ID)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Both wakes defer to the OUTER request tx's commit — the handler
	// WithTenantTx is a nested reuse of the TenantTxMiddleware tx and has NOT
	// committed on return, so a woken client must not refetch yet (#1725 review).
	// This binds the STAFF broadcast as much as the guardian wake: a client that
	// refetches pre-commit reads the previous plan and nothing invalidates it a
	// second time, and a later 5xx rolls the write back after every open tab has
	// already refreshed to it. The guardian fan-out is separate because the
	// tenant-wide arrival_schedule_changed never reaches the parent SSE stream.
	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.broadcastArrivalScheduleChanged(student.ID)
		rs.wakeChildGuardians(tenantID, student.ID)
	})
	response := buildArrivalDataResponse(data)
	common.Respond(w, r, http.StatusOK, response, "Arrival schedules updated successfully")
}

// createStudentArrivalException handles POST /students/{id}/arrival-exceptions
func (rs *Resource) createStudentArrivalException(w http.ResponseWriter, r *http.Request) {
	student := rs.requireArrivalWriteAccess(w, r, "create arrival exceptions")
	if student == nil {
		return
	}

	req := &ArrivalExceptionRequest{}
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
	var arrivalTime *time.Time
	if req.ExpectedArrival != nil && *req.ExpectedArrival != "" {
		parsed, _ := parseTimeOnly(*req.ExpectedArrival)
		arrivalTime = &parsed
	}

	tenantID := tenant.FromContext(r.Context())
	exception, err := rs.ArrivalScheduleService.CreateOrReclaimException(
		r.Context(), student.ID, exceptionDate, arrivalTime, req.Reason, staffID,
		func() (int64, error) { return rs.getStaffIDFromJWT(r) },
	)
	if err != nil {
		renderExceptionWriteError(w, r, err)
		return
	}

	// Deferred to the outer request tx's commit — the service write runs in a
	// nested tx that has NOT committed on return, so neither the staff broadcast
	// nor the guardian wake may fire yet (#1725 review; see the schedules
	// handler above for why pre-commit staff invalidation strands stale data).
	// The guardian wake is what makes the "Heute" tile reflect a staff arrival
	// override (a no-show arrival_absent resolves the tile as absent).
	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.broadcastArrivalScheduleChanged(student.ID)
		rs.wakeChildGuardians(tenantID, student.ID)
	})
	common.Respond(w, r, http.StatusCreated, mapArrivalExceptionToResponse(exception), "Arrival exception created successfully")
}

// updateStudentArrivalException handles PUT /students/{id}/arrival-exceptions/{exceptionId}
func (rs *Resource) updateStudentArrivalException(w http.ResponseWriter, r *http.Request) {
	student := rs.requireArrivalWriteAccess(w, r, "update arrival exceptions")
	if student == nil {
		return
	}

	exceptionID, ok := parseEntityID(w, r, "exceptionId", "exception")
	if !ok {
		return
	}

	existingException := rs.verifyArrivalExceptionOwnership(w, r, exceptionID, student.ID)
	if existingException == nil {
		return
	}

	req := &ArrivalExceptionRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// existingException was the ownership pre-check; the locked re-read inside the
	// service is authoritative for the source/author decision.
	exceptionDate, _ := timezone.ParseDate(req.ExceptionDate)
	var arrivalTime *time.Time
	if req.ExpectedArrival != nil && *req.ExpectedArrival != "" {
		parsed, _ := parseTimeOnly(*req.ExpectedArrival)
		arrivalTime = &parsed
	}

	tenantID := tenant.FromContext(r.Context())
	exception, err := rs.ArrivalScheduleService.UpdateException(
		r.Context(), exceptionID, student.ID, exceptionDate, req.Reason, arrivalTime, req.ClearExpectedArrival,
		func() (int64, error) { return rs.getStaffIDFromJWT(r) },
	)
	if err != nil {
		renderExceptionWriteError(w, r, err)
		return
	}

	// Deferred to the outer request tx's commit so neither the staff broadcast
	// nor the guardian wake can make a client refetch the pre-edit override (the
	// service write runs in a nested tx, not committed on return) (#1725 review).
	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.broadcastArrivalScheduleChanged(student.ID)
		rs.wakeChildGuardians(tenantID, student.ID)
	})
	common.Respond(w, r, http.StatusOK, mapArrivalExceptionToResponse(exception), "Arrival exception updated successfully")
}

// deleteStudentArrivalException handles DELETE /students/{id}/arrival-exceptions/{exceptionId}
func (rs *Resource) deleteStudentArrivalException(w http.ResponseWriter, r *http.Request) {
	student := rs.requireArrivalWriteAccess(w, r, "delete arrival exceptions")
	if student == nil {
		return
	}

	exceptionID, ok := parseEntityID(w, r, "exceptionId", "exception")
	if !ok {
		return
	}

	existingException := rs.verifyArrivalExceptionOwnership(w, r, exceptionID, student.ID)
	if existingException == nil {
		return
	}

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := scheduleService.LockCareExceptionDay(ctx, rs.DB, student.ID, existingException.ExceptionDate); err != nil {
			return err
		}
		freshException, err := rs.ArrivalScheduleService.GetStudentArrivalExceptionByID(ctx, exceptionID)
		if err != nil {
			return err
		}
		if freshException == nil {
			return nil
		}
		if freshException.StudentID != student.ID {
			return ErrExceptionWrongStudent
		}
		return rs.ArrivalScheduleService.DeleteStudentArrivalException(ctx, freshException.ID)
	}); err != nil {
		renderExceptionWriteError(w, r, err)
		return
	}

	// Deferred to the outer request tx's commit so neither the staff broadcast
	// nor the guardian wake can make a client refetch an override the delete has
	// not committed yet (nested handler WithTenantTx) (#1725 review).
	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.broadcastArrivalScheduleChanged(student.ID)
		rs.wakeChildGuardians(tenantID, student.ID)
	})
	common.Respond(w, r, http.StatusOK, nil, "Arrival exception deleted successfully")
}

// createStudentArrivalNote handles POST /students/{id}/arrival-notes
func (rs *Resource) createStudentArrivalNote(w http.ResponseWriter, r *http.Request) {
	student := rs.requireArrivalWriteAccess(w, r, "create arrival notes")
	if student == nil {
		return
	}

	req := &ArrivalNoteRequest{}
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
	note := &schedule.StudentArrivalNote{
		StudentID: student.ID,
		NoteDate:  noteDate,
		Content:   req.Content,
		CreatedBy: staffID,
	}

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.ArrivalScheduleService.CreateStudentArrivalNote(ctx, note)
	}); err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// After commit, not inline: the handler WithTenantTx above only reuses the
	// still-open TenantTxMiddleware tx, so a client woken here would read the
	// note-less day and never be invalidated again (#1725 review).
	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.broadcastArrivalScheduleChanged(student.ID)
	})
	common.Respond(w, r, http.StatusCreated, mapArrivalNoteToResponse(note), "Arrival note created successfully")
}

// updateStudentArrivalNote handles PUT /students/{id}/arrival-notes/{noteId}
func (rs *Resource) updateStudentArrivalNote(w http.ResponseWriter, r *http.Request) {
	student := rs.requireArrivalWriteAccess(w, r, "update arrival notes")
	if student == nil {
		return
	}

	noteID, ok := parseEntityID(w, r, "noteId", "note")
	if !ok {
		return
	}

	existingNote := rs.verifyArrivalNoteOwnership(w, r, noteID, student.ID)
	if existingNote == nil {
		return
	}

	req := &ArrivalNoteRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	noteDate, _ := timezone.ParseDate(req.NoteDate)
	note := &schedule.StudentArrivalNote{
		StudentID: student.ID,
		NoteDate:  noteDate,
		Content:   req.Content,
		CreatedBy: existingNote.CreatedBy,
	}
	note.ID = noteID
	note.CreatedAt = existingNote.CreatedAt
	note.SetTenantID(existingNote.TenantID)

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.ArrivalScheduleService.UpdateStudentArrivalNote(ctx, note)
	}); err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// After commit — same nested-tx reason as the create path above.
	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.broadcastArrivalScheduleChanged(student.ID)
	})
	common.Respond(w, r, http.StatusOK, mapArrivalNoteToResponse(note), "Arrival note updated successfully")
}

// deleteStudentArrivalNote handles DELETE /students/{id}/arrival-notes/{noteId}
func (rs *Resource) deleteStudentArrivalNote(w http.ResponseWriter, r *http.Request) {
	student := rs.requireArrivalWriteAccess(w, r, "delete arrival notes")
	if student == nil {
		return
	}

	noteID, ok := parseEntityID(w, r, "noteId", "note")
	if !ok {
		return
	}

	existingNote := rs.verifyArrivalNoteOwnership(w, r, noteID, student.ID)
	if existingNote == nil {
		return
	}

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.ArrivalScheduleService.DeleteStudentArrivalNote(ctx, noteID)
	}); err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// After commit — same nested-tx reason as the create path above.
	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.broadcastArrivalScheduleChanged(student.ID)
	})
	common.Respond(w, r, http.StatusOK, nil, "Arrival note deleted successfully")
}

// bulkUpsertArrivalSchedules handles POST /students/arrival-schedules/bulk
func (rs *Resource) bulkUpsertArrivalSchedules(w http.ResponseWriter, r *http.Request) {
	req := &BulkUpsertArrivalScheduleRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	staffID, err := rs.getStaffIDFromJWT(r)
	if err != nil {
		renderError(w, r, common.ErrorForbidden(err))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	result, err := rs.ArrivalScheduleService.BulkUpsertArrivalSchedules(
		r.Context(),
		scheduleService.ArrivalScheduleBulkFilter{
			SchoolClass: req.SchoolClass,
			GroupID:     req.GroupID,
			StudentIDs:  req.StudentIDs,
			Authorize: func(ctx context.Context, student *users.Student) (bool, error) {
				return canUpdateStudent(ctx, jwt.PermissionsFromCtx(r.Context()), student, rs.UserContextService)
			},
		},
		req.Schedules,
		staffID,
	)
	if err != nil {
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

	// Deferred to the OUTER request tx's commit: BulkUpsertArrivalSchedules runs
	// inside the TenantTxMiddleware tx, which is still open here, so waking now
	// would let a client refetch before the writes are visible (#1725 review) —
	// true for the staff broadcast and the guardian wake alike, and worst here
	// where one rollback would strand a whole class of stale plans. The guardian
	// fan-out is separate (tenant-wide events never reach the parent SSE stream,
	// #1725) and bounded to the one class.
	affected := result.AffectedStudentIDs
	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.broadcastArrivalScheduleChanged(0)
		for _, studentID := range affected {
			rs.wakeChildGuardians(tenantID, studentID)
		}
	})
	common.Respond(w, r, http.StatusOK, result, "Bulk arrival schedules upserted successfully")
}

// BulkArrivalTimeRequest keeps the domain-specific public name while sharing
// validation and binding with pickup bulk effective-time requests.
type BulkArrivalTimeRequest = BulkEffectiveTimeRequest

// BulkArrivalDayNoteResponse represents a single day note in bulk arrival time responses
type BulkArrivalDayNoteResponse struct {
	ID      int64  `json:"id"`
	Content string `json:"content"`
}

// BulkArrivalTimeResponse represents arrival time data for a single student
type BulkArrivalTimeResponse struct {
	StudentID       int64                        `json:"student_id"`
	Date            string                       `json:"date"`
	WeekdayName     string                       `json:"weekday_name"`
	ExpectedArrival *string                      `json:"expected_arrival,omitempty"` // HH:MM format or null
	IsException     bool                         `json:"is_exception"`
	Notes           string                       `json:"notes,omitempty"`
	DayNotes        []BulkArrivalDayNoteResponse `json:"day_notes,omitempty"`
}

// getBulkArrivalTimes handles POST /students/arrival-times/bulk
func (rs *Resource) getBulkArrivalTimes(w http.ResponseWriter, r *http.Request) {
	handleBulkEffectiveTimes(
		rs,
		w,
		r,
		"Bulk arrival times retrieved successfully",
		rs.ArrivalScheduleService.GetBulkEffectiveArrivalTimesForDate,
		mapBulkArrivalTimeResponse,
	)
}

func mapBulkArrivalTimeResponse(
	studentID int64,
	effectiveTime *scheduleService.EffectiveArrivalTime,
) BulkArrivalTimeResponse {
	response := BulkArrivalTimeResponse{
		StudentID:   studentID,
		Date:        effectiveTime.Date.Format(dateFormatISO),
		WeekdayName: effectiveTime.WeekdayName,
		IsException: effectiveTime.IsException,
		Notes:       effectiveTime.Notes,
	}
	if effectiveTime.ArrivalTime != nil {
		formatted := effectiveTime.ArrivalTime.Format("15:04")
		response.ExpectedArrival = &formatted
	}
	if len(effectiveTime.DayNotes) > 0 {
		response.DayNotes = make([]BulkArrivalDayNoteResponse, 0, len(effectiveTime.DayNotes))
		for _, note := range effectiveTime.DayNotes {
			response.DayNotes = append(response.DayNotes, BulkArrivalDayNoteResponse{
				ID:      note.ID,
				Content: note.Content,
			})
		}
	}
	return response
}

// ClassArrivalTimesResponse is the current Unterrichtsschluss of one class.
type ClassArrivalTimesResponse struct {
	SchoolClass string            `json:"school_class"`
	Times       map[string]string `json:"times"`
	UpdatedAt   *string           `json:"updated_at,omitempty"`
}

// getClassArrivalTimes handles GET /students/class-arrival-times/{schoolClass}.
// The bulk screen reads it so it opens with what the class already carries
// instead of empty fields (#2414).
func (rs *Resource) getClassArrivalTimes(w http.ResponseWriter, r *http.Request) {
	schoolClass := chi.URLParam(r, "schoolClass")
	times, err := rs.ArrivalScheduleService.GetClassArrivalTimes(r.Context(), schoolClass)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	resp := ClassArrivalTimesResponse{SchoolClass: times.SchoolClass, Times: times.Times}
	if times.UpdatedAt != nil {
		formatted := times.UpdatedAt.Format(time.RFC3339)
		resp.UpdatedAt = &formatted
	}
	common.Respond(w, r, http.StatusOK, resp, "Class arrival times retrieved successfully")
}
