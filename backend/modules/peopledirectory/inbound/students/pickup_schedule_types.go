package students

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
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
	NoteDate  string `json:"note_date,omitempty"` // YYYY-MM-DD format; empty for a recurring note
	Weekday   int    `json:"weekday,omitempty"`   // 1-5; set for a recurring weekday note (#3369)
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
	StudentIDs         []int64                        `json:"student_ids"`
	Schedules          []careplan.PickupScheduleInput `json:"schedules"`
	ConfirmedException bool                           `json:"confirmed_exception"`
}

// PickupExceptionRequest represents a request to create/update a pickup exception
type PickupExceptionRequest struct {
	ExceptionDate   string  `json:"exception_date"` // YYYY-MM-DD format
	PickupTime      *string `json:"pickup_time,omitempty"`
	ClearPickupTime bool    `json:"clear_pickup_time,omitempty"`
	Reason          *string `json:"reason,omitempty"`
}

// PickupNoteRequest represents a request to create/update a pickup note. A
// note is either dated (note_date) or recurs on a weekday (weekday, #3369):
// the recurring shape is what lets a day WITHOUT a pickup time carry a note,
// because it does not mark the child as expected the way a weekly row does.
type PickupNoteRequest struct {
	NoteDate string `json:"note_date"` // YYYY-MM-DD format
	Weekday  int    `json:"weekday"`   // 1 (Monday) to 5 (Friday)
	Content  string `json:"content"`
}

// WeekdayPickupNotesRequest replaces the notes that recur every week. Dated
// notes stay owned by the single-day editor and are deliberately excluded.
type WeekdayPickupNotesRequest struct {
	Notes []PickupNoteRequest `json:"notes"`
}

// Bind implements render.Binder.
func (r *WeekdayPickupNotesRequest) Bind(_ *http.Request) error {
	if r.Notes == nil {
		return errors.New("notes is required")
	}
	seen := make(map[int]struct{}, len(r.Notes))
	for _, note := range r.Notes {
		if note.NoteDate != "" {
			return errors.New("weekday notes cannot have note_date")
		}
		if note.Weekday < weekdayMonday || note.Weekday > weekdayFriday {
			return errors.New("weekday must be between 1 (Monday) and 5 (Friday)")
		}
		if _, exists := seen[note.Weekday]; exists {
			return errors.New("weekday notes must be unique")
		}
		seen[note.Weekday] = struct{}{}
		if err := validateCareNoteContent(note.Content); err != nil {
			return err
		}
	}
	return nil
}

// Bind implements render.Binder
func (r *PickupNoteRequest) Bind(_ *http.Request) error {
	if r.Weekday == 0 {
		return validateCareNoteRequest(r.NoteDate, r.Content)
	}
	if r.NoteDate != "" {
		return errors.New("a note takes note_date or weekday, not both")
	}
	if r.Weekday < weekdayMonday || r.Weekday > weekdayFriday {
		return errors.New("weekday must be between 1 (Monday) and 5 (Friday)")
	}
	return validateCareNoteContent(r.Content)
}

// toModel maps the validated request onto a note for the given student.
func (r *PickupNoteRequest) toModel(studentID, createdBy int64) *careplan.PickupNote {
	note := &careplan.PickupNote{StudentID: studentID, Weekday: r.Weekday, Content: r.Content, CreatedBy: createdBy}
	if r.Weekday == 0 {
		noteDate, _ := timezone.ParseDate(r.NoteDate)
		note.NoteDate = careplan.Date(noteDate)
	}
	return note
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
func toPickupScheduleModels(items []PickupScheduleRequest, studentID, staffID int64) []*careplan.PickupSchedule {
	schedules := make([]*careplan.PickupSchedule, 0, len(items))
	for _, s := range items {
		pickupTime, _ := parseTimeOnly(s.PickupTime)
		schedules = append(schedules, &careplan.PickupSchedule{
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
func mapScheduleToResponse(s *careplan.PickupSchedule) PickupScheduleResponse {
	resp := PickupScheduleResponse{
		ID:               s.ID,
		StudentID:        s.StudentID,
		Weekday:          s.Weekday,
		WeekdayName:      weekdayNames[s.Weekday],
		PickupTime:       s.PickupTime.Format("15:04"),
		Notes:            s.Notes,
		Source:           s.Source,
		CareOfferingName: s.CareOfferingName,
		CreatedBy:        s.CreatedBy,
		CreatedAt:        s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        s.UpdatedAt.Format(time.RFC3339),
	}
	if resp.Source == "" {
		resp.Source = careplan.ScheduleSourceStaff
	}
	if s.CareOfferingID != nil {
		id := strconv.FormatInt(*s.CareOfferingID, 10)
		resp.CareOfferingID = &id
	}
	return resp
}

// mapExceptionToResponse converts an exception model to API response
func mapExceptionToResponse(e *careplan.PickupException) PickupExceptionResponse {
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
		formatted := timezone.NormalizeWallClock(*e.ExcusedFrom).Format("15:04")
		resp.ExcusedFrom = &formatted
	}
	return resp
}

// mapNoteToResponse converts a note model to API response
func mapNoteToResponse(n *careplan.PickupNote) PickupNoteResponse {
	return PickupNoteResponse{
		ID:        n.ID,
		StudentID: n.StudentID,
		NoteDate:  n.NoteDate.String(),
		Weekday:   n.Weekday,
		Content:   n.Content,
		CreatedBy: n.CreatedBy,
		CreatedAt: n.CreatedAt.Format(time.RFC3339),
		UpdatedAt: n.UpdatedAt.Format(time.RFC3339),
	}
}

// verifyExceptionOwnership checks that an exception exists and belongs to the given student.
// Returns the exception if valid, or writes an error response and returns nil.
func (rs *Resource) verifyExceptionOwnership(w http.ResponseWriter, r *http.Request, exceptionID, studentID int64) *careplan.PickupException {
	return verifyCareOwnership(
		w,
		r,
		exceptionID,
		studentID,
		"pickup exception not found",
		"exception does not belong to this student",
		rs.PickupScheduleService.GetStudentPickupExceptionByID,
		func(exception *careplan.PickupException) int64 { return exception.StudentID },
	)
}

// verifyNoteOwnership checks that a note exists and belongs to the given student.
// Returns the note if valid, or writes an error response and returns nil.
func (rs *Resource) verifyNoteOwnership(w http.ResponseWriter, r *http.Request, noteID, studentID int64) *careplan.PickupNote {
	return verifyCareOwnership(
		w,
		r,
		noteID,
		studentID,
		"pickup note not found",
		"note does not belong to this student",
		rs.PickupScheduleService.GetStudentPickupNoteByID,
		func(note *careplan.PickupNote) int64 { return note.StudentID },
	)
}

var (
	errPersonNotFoundForAccount = errors.New("person not found for account")
	errUserNotStaff             = errors.New("user is not a staff member")
)
