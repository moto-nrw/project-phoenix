package activities

import (
	"errors"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/models/activities"
)

const (
	routeScheduleByID         = "/{id}/schedules/{scheduleId}"
	msgActivityCreatedSuccess = "Activity created successfully"
	msgActivityUpdatedSuccess = "Activity updated successfully"
)

// CategoryResponse represents a category API response
type CategoryResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Color       string `json:"color,omitempty"`
	// ShiftTypeID is the optional Dienstplan-Schichtart this category maps to
	// (#1836/#1837 follow-up); nil = no mapping. Consumers resolve the shift
	// type's name/color from the shift-types list.
	ShiftTypeID *int64 `json:"shift_type_id,omitempty"`
	// IsSystem marks auto-provisioned infrastructure categories (Schulhof,
	// WC). They are read-only for schools (#2131).
	IsSystem bool `json:"is_system"`
	// ArchivedAt is set once a category has been retired. Archived categories
	// stay resolvable for existing Termine but are not offered for new ones.
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
	// UsageCount is how many Aktivitäten/Termin-Vorlagen reference the
	// category. Only populated by the category list endpoint; nil elsewhere.
	UsageCount *int      `json:"usage_count,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// SupervisorResponse represents a supervisor in activity response
type SupervisorResponse struct {
	ID        int64  `json:"id"`
	StaffID   int64  `json:"staff_id"`
	IsPrimary bool   `json:"is_primary"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

// ActivityResponse represents an activity group API response
type ActivityResponse struct {
	ID              int64                `json:"id"`
	Name            string               `json:"name"`
	MaxParticipants int                  `json:"max_participants"`
	IsOpen          bool                 `json:"is_open"`
	CategoryID      int64                `json:"category_id"`
	PlannedRoomID   *int64               `json:"planned_room_id,omitempty"`
	CreatedBy       *int64               `json:"created_by"`
	CreatedByName   string               `json:"created_by_name,omitempty"`
	Category        *CategoryResponse    `json:"category,omitempty"`
	SupervisorID    *int64               `json:"supervisor_id,omitempty"`  // Primary supervisor
	SupervisorIDs   []int64              `json:"supervisor_ids,omitempty"` // All supervisors
	Supervisors     []SupervisorResponse `json:"supervisors,omitempty"`    // Detailed supervisor info
	Schedules       []ScheduleResponse   `json:"schedules,omitempty"`
	EnrollmentCount int                  `json:"enrollment_count,omitempty"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
}

// ScheduleResponse represents a schedule API response
type ScheduleResponse struct {
	ID              int64     `json:"id"`
	Weekday         int       `json:"weekday"`
	TimeframeID     *int64    `json:"timeframe_id,omitempty"`
	ActivityGroupID int64     `json:"activity_group_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// StudentResponse represents a simplified student in activity response
type StudentResponse struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// TimespanResponse represents a time span for activities
type TimespanResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	Description string `json:"description,omitempty"`
}

// ActivityRequest represents an activity creation/update request
type ActivityRequest struct {
	Name            string            `json:"name"`
	MaxParticipants int               `json:"max_participants"`
	IsOpen          bool              `json:"is_open"`
	CategoryID      int64             `json:"category_id"`
	PlannedRoomID   *int64            `json:"planned_room_id,omitempty"`
	Schedules       []ScheduleRequest `json:"schedules,omitempty"`
	SupervisorIDs   []int64           `json:"supervisor_ids,omitempty"`
}

// QuickActivityRequest represents a simplified activity creation request for mobile devices
type QuickActivityRequest struct {
	Name            string `json:"name"`
	CategoryID      int64  `json:"category_id"`
	RoomID          *int64 `json:"room_id,omitempty"`
	MaxParticipants int    `json:"max_participants"`
}

// QuickActivityResponse represents the response after creating an activity via quick-create
type QuickActivityResponse struct {
	ActivityID     int64     `json:"activity_id"`
	Name           string    `json:"name"`
	CategoryName   string    `json:"category_name"`
	RoomName       string    `json:"room_name,omitempty"`
	SupervisorName string    `json:"supervisor_name"`
	Status         string    `json:"status"`
	Message        string    `json:"message"`
	CreatedAt      time.Time `json:"created_at"`
}

// ScheduleRequest represents a schedule in activity creation/update request
type ScheduleRequest struct {
	Weekday     int    `json:"weekday"`
	TimeframeID *int64 `json:"timeframe_id,omitempty"`
}

// Bind validates the activity request
func (req *ActivityRequest) Bind(_ *http.Request) error {
	if req.Name == "" {
		return errors.New("activity name is required")
	}
	if req.MaxParticipants <= 0 {
		return errors.New("max participants must be greater than zero")
	}
	if req.CategoryID <= 0 {
		return errors.New("category ID is required")
	}

	// Validate schedules if provided
	if len(req.Schedules) > 0 {
		for _, schedule := range req.Schedules {
			if !activities.IsValidWeekday(schedule.Weekday) {
				return errors.New("invalid weekday in schedule")
			}
		}
	}

	return nil
}

// Bind validates the quick activity request
func (req *QuickActivityRequest) Bind(_ *http.Request) error {
	if req.Name == "" {
		return errors.New("activity name is required")
	}
	if req.CategoryID <= 0 {
		return errors.New("category ID is required")
	}
	if req.MaxParticipants <= 0 {
		return errors.New("max participants must be greater than zero")
	}
	return nil
}

// BatchEnrollmentRequest represents a request for updating enrollments in batch
type BatchEnrollmentRequest struct {
	StudentIDs []int64 `json:"student_ids"`
}

// Bind validates the batch enrollment request
func (req *BatchEnrollmentRequest) Bind(_ *http.Request) error {
	if req.StudentIDs == nil {
		return errors.New("student IDs are required")
	}
	return nil
}

// SupervisorRequest represents a supervisor assignment request
type SupervisorRequest struct {
	StaffID   int64 `json:"staff_id"`
	IsPrimary bool  `json:"is_primary"`
}

// Bind validates the supervisor request
func (req *SupervisorRequest) Bind(_ *http.Request) error {
	if req.StaffID <= 0 {
		return errors.New("staff ID is required and must be greater than 0")
	}
	return nil
}
