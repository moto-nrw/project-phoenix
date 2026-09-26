package students

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
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

const maxArrivalScheduleDateRangeDays = 7

// BulkUpsertArrivalScheduleRequest represents a request to bulk upsert arrival
// schedules for exactly one filtered student cohort.
type BulkUpsertArrivalScheduleRequest struct {
	SchoolClass string                          `json:"school_class"`
	GroupID     int64                           `json:"group_id"`
	StudentIDs  []int64                         `json:"student_ids"`
	Schedules   []careplan.ArrivalScheduleInput `json:"schedules"`
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
func toArrivalScheduleModels(items []ArrivalScheduleRequestItem, studentID, staffID int64) []*careplan.ArrivalSchedule {
	schedules := make([]*careplan.ArrivalSchedule, 0, len(items))
	for _, s := range items {
		arrivalTime, _ := parseTimeOnly(s.ExpectedArrival)
		schedules = append(schedules, &careplan.ArrivalSchedule{
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
		if s.Weekday < weekdayMonday || s.Weekday > weekdayFriday {
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
func mapArrivalScheduleToResponse(s *careplan.ArrivalSchedule) ArrivalScheduleResponse {
	resp := ArrivalScheduleResponse{
		ID:          s.ID,
		StudentID:   s.StudentID,
		Weekday:     s.Weekday,
		WeekdayName: weekdayNames[s.Weekday],
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
func mapArrivalExceptionToResponse(e *careplan.ArrivalException) ArrivalExceptionResponse {
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
func mapArrivalNoteToResponse(n *careplan.ArrivalNote) ArrivalNoteResponse {
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
func (rs *Resource) verifyArrivalExceptionOwnership(w http.ResponseWriter, r *http.Request, exceptionID, studentID int64) *careplan.ArrivalException {
	return verifyCareOwnership(
		w,
		r,
		exceptionID,
		studentID,
		"arrival exception not found",
		"exception does not belong to this student",
		rs.ArrivalScheduleService.GetStudentArrivalExceptionByID,
		func(exception *careplan.ArrivalException) int64 { return exception.StudentID },
	)
}

// verifyArrivalNoteOwnership checks that an arrival note exists and belongs to the given student.
func (rs *Resource) verifyArrivalNoteOwnership(w http.ResponseWriter, r *http.Request, noteID, studentID int64) *careplan.ArrivalNote {
	return verifyCareOwnership(
		w,
		r,
		noteID,
		studentID,
		"arrival note not found",
		"note does not belong to this student",
		rs.ArrivalScheduleService.GetStudentArrivalNoteByID,
		func(note *careplan.ArrivalNote) int64 { return note.StudentID },
	)
}

// requireArrivalReadAccess parses the student from URL params and checks read access.
func (rs *Resource) requireArrivalReadAccess(w http.ResponseWriter, r *http.Request) *Student {
	return rs.requireCareReadAccess(w, r, "arrival")
}

// requireArrivalWriteAccess parses the student from URL params and verifies full access.
func (rs *Resource) requireArrivalWriteAccess(w http.ResponseWriter, r *http.Request, action string) *Student {
	return rs.requireCareWriteAccess(w, r, action)
}
