package httpadapter

import (
	"net/http"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/moto-nrw/project-phoenix/api/common"
)

// SchedulesResource defines the schedules API resource
type DateframeRequest struct {
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// Bind validates the dateframe request
func (req *DateframeRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.StartDate, validation.Required),
		validation.Field(&req.EndDate, validation.Required),
	)
}

// DateframeResponse represents a dateframe response
type DateframeResponse struct {
	ID          int64       `json:"id"`
	StartDate   common.Time `json:"start_date"`
	EndDate     common.Time `json:"end_date"`
	Name        string      `json:"name,omitempty"`
	Description string      `json:"description,omitempty"`
	CreatedAt   common.Time `json:"created_at"`
	UpdatedAt   common.Time `json:"updated_at"`
}

// TimeframeRequest represents a timeframe creation/update request
type TimeframeRequest struct {
	StartTime   string  `json:"start_time"`
	EndTime     *string `json:"end_time,omitempty"`
	IsActive    bool    `json:"is_active"`
	Description string  `json:"description,omitempty"`
}

// Bind validates the timeframe request
func (req *TimeframeRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.StartTime, validation.Required),
	)
}

// TimeframeResponse represents a timeframe response
type TimeframeResponse struct {
	ID          int64        `json:"id"`
	StartTime   common.Time  `json:"start_time"`
	EndTime     *common.Time `json:"end_time,omitempty"`
	IsActive    bool         `json:"is_active"`
	Description string       `json:"description,omitempty"`
	CreatedAt   common.Time  `json:"created_at"`
	UpdatedAt   common.Time  `json:"updated_at"`
}

// RecurrenceRuleRequest represents a recurrence rule creation/update request
type RecurrenceRuleRequest struct {
	Frequency     string   `json:"frequency"`
	IntervalCount int      `json:"interval_count"`
	Weekdays      []string `json:"weekdays,omitempty"`
	MonthDays     []int    `json:"month_days,omitempty"`
	EndDate       *string  `json:"end_date,omitempty"`
	Count         *int     `json:"count,omitempty"`
}

// Bind validates the recurrence rule request
func (req *RecurrenceRuleRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.Frequency, validation.Required, validation.In(
			recurrenceFrequencyDaily,
			recurrenceFrequencyWeekly,
			recurrenceFrequencyMonthly,
			recurrenceFrequencyYearly,
		)),
		validation.Field(&req.IntervalCount, validation.Min(1)),
	)
}

// RecurrenceRuleResponse represents a recurrence rule response
type RecurrenceRuleResponse struct {
	ID            int64        `json:"id"`
	Frequency     string       `json:"frequency"`
	IntervalCount int          `json:"interval_count"`
	Weekdays      []string     `json:"weekdays,omitempty"`
	MonthDays     []int        `json:"month_days,omitempty"`
	EndDate       *common.Time `json:"end_date,omitempty"`
	Count         *int         `json:"count,omitempty"`
	CreatedAt     common.Time  `json:"created_at"`
	UpdatedAt     common.Time  `json:"updated_at"`
}

// GenerateEventsRequest represents a request to generate events from a recurrence rule
type GenerateEventsRequest struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

// Bind validates the generate events request
func (req *GenerateEventsRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.StartDate, validation.Required),
		validation.Field(&req.EndDate, validation.Required),
	)
}

// CheckConflictRequest represents a request to check for schedule conflicts
type CheckConflictRequest struct {
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

// Bind validates the check conflict request
func (req *CheckConflictRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.StartTime, validation.Required),
		validation.Field(&req.EndTime, validation.Required),
	)
}

// FindAvailableSlotsRequest represents a request to find available time slots
type FindAvailableSlotsRequest struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Duration  int    `json:"duration"` // in minutes
}

// Bind validates the find available slots request
func (req *FindAvailableSlotsRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.StartDate, validation.Required),
		validation.Field(&req.EndDate, validation.Required),
		validation.Field(&req.Duration, validation.Required, validation.Min(1)),
	)
}

// Helper functions

// parseDateframeDates parses and validates start and end dates, handling errors internally
