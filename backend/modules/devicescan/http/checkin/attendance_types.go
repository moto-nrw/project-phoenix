package checkin

import (
	"errors"
	"net/http"
	"time"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

// AttendanceStatusResponse represents the response for checking a student's attendance status
type AttendanceStatusResponse struct {
	Student    AttendanceStudentInfo `json:"student"`
	Attendance AttendanceInfo        `json:"attendance"`
}

// AttendanceStudentInfo represents student information in attendance responses
type AttendanceStudentInfo struct {
	ID        int64                `json:"id"`
	FirstName string               `json:"first_name"`
	LastName  string               `json:"last_name"`
	Group     *AttendanceGroupInfo `json:"group,omitempty"`
}

// AttendanceInfo represents attendance status and timing information.
// Date is a calendar day and marshals as "YYYY-MM-DD" or null when the
// response carries no row (cancel, daily checkout). PyrePortal types the
// field as string and never reads it.
type AttendanceInfo struct {
	Status       string     `json:"status"` // "not_checked_in", "checked_in", "checked_out"
	Date         *string    `json:"date"`
	CheckInTime  *time.Time `json:"check_in_time"`
	CheckOutTime *time.Time `json:"check_out_time"`
	CheckedInBy  string     `json:"checked_in_by"`  // Formatted as "FirstName LastName"
	CheckedOutBy string     `json:"checked_out_by"` // Formatted as "FirstName LastName"
}

// AttendanceGroupInfo represents group information from education.groups table
type AttendanceGroupInfo struct {
	ID   int64  `json:"id"`   // From education.groups.id
	Name string `json:"name"` // From education.groups.name (NOT student.SchoolClass)
}

// AttendanceToggleRequest represents a request to toggle student attendance
type AttendanceToggleRequest struct {
	RFID        string  `json:"rfid"`
	Action      string  `json:"action"`                // "confirm", "cancel", or "confirm_daily_checkout"
	Destination *string `json:"destination,omitempty"` // "zuhause" or "unterwegs" (required for confirm_daily_checkout)
}

// Bind validates the attendance toggle request
func (req *AttendanceToggleRequest) Bind(_ *http.Request) error {
	if err := validation.ValidateStruct(req,
		validation.Field(&req.RFID, validation.Required),
		validation.Field(&req.Action, validation.Required, validation.In(
			devicescan.AttendanceActionConfirm, devicescan.AttendanceActionCancel, devicescan.AttendanceActionDailyCheckout,
		)),
	); err != nil {
		return err
	}

	// Conditional validation: destination required for confirm_daily_checkout
	if req.Action == devicescan.AttendanceActionDailyCheckout {
		if req.Destination == nil || *req.Destination == "" {
			return errors.New("destination is required for confirm_daily_checkout")
		}
		if *req.Destination != devicescan.DestinationHome && *req.Destination != devicescan.DestinationTransit {
			return errors.New("destination must be 'zuhause' or 'unterwegs'")
		}
	}

	return nil
}

// AttendanceToggleResponse represents the response after toggling attendance
type AttendanceToggleResponse struct {
	Action          string                `json:"action"` // "checked_in", "checked_out", "cancelled"
	Student         AttendanceStudentInfo `json:"student"`
	Attendance      AttendanceInfo        `json:"attendance"`
	Message         string                `json:"message"`                    // User-friendly message for RFID device display
	FeedbackEnabled *bool                 `json:"feedback_enabled,omitempty"` // Whether the feedback modal should be shown
}
