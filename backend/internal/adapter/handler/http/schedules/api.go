package schedules

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/jwt"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/internal/core/service/schedule"
)

// Use shared constants from common package
var dateLayout = common.DateFormatISO

const (
	errMsgInvalidDateframeID      = "invalid dateframe ID"
	errMsgInvalidTimeframeID      = "invalid timeframe ID"
	errMsgInvalidRecurrenceRuleID = "invalid recurrence rule ID"
	errMsgInvalidStartDate        = "invalid start date format"
	errMsgInvalidEndDate          = "invalid end date format"
	errMsgInvalidStartTime        = "invalid start time format"
	errMsgInvalidEndTime          = "invalid end time format"
	msgRecurrenceRulesRetrieved   = "Recurrence rules retrieved successfully"
)

// Resource defines the schedules API resource
type Resource struct {
	ScheduleService scheduleSvc.Service
}

// NewResource creates a new schedules resource
func NewResource(scheduleService scheduleSvc.Service) *Resource {
	return &Resource{
		ScheduleService: scheduleService,
	}
}

// Router returns a configured router for schedule endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth := jwt.MustTokenAuth()

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// Current dateframe endpoint - requires schedules:read permission
		r.With(authorize.RequiresPermission(permissions.SchedulesRead)).Get("/current-dateframe", rs.getCurrentDateframe)

		// Dateframe endpoints
		r.Route("/dateframes", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/", rs.listDateframes)
			r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/{id}", rs.getDateframe)
			r.With(authorize.RequiresPermission(permissions.ActivitiesCreate)).Post("/", rs.createDateframe)
			r.With(authorize.RequiresPermission(permissions.ActivitiesUpdate)).Put("/{id}", rs.updateDateframe)
			r.With(authorize.RequiresPermission(permissions.ActivitiesDelete)).Delete("/{id}", rs.deleteDateframe)

			// Special dateframe queries
			r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/by-date", rs.getDateframesByDate)
			r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/overlapping", rs.getOverlappingDateframes)
		})

		// Timeframe endpoints
		r.Route("/timeframes", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/", rs.listTimeframes)
			r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/{id}", rs.getTimeframe)
			r.With(authorize.RequiresPermission(permissions.ActivitiesCreate)).Post("/", rs.createTimeframe)
			r.With(authorize.RequiresPermission(permissions.ActivitiesUpdate)).Put("/{id}", rs.updateTimeframe)
			r.With(authorize.RequiresPermission(permissions.ActivitiesDelete)).Delete("/{id}", rs.deleteTimeframe)

			// Special timeframe queries
			r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/active", rs.getActiveTimeframes)
			r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/by-range", rs.getTimeframesByRange)
		})

		// Recurrence rule endpoints
		r.Route("/recurrence-rules", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/", rs.listRecurrenceRules)
			r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/{id}", rs.getRecurrenceRule)
			r.With(authorize.RequiresPermission(permissions.ActivitiesCreate)).Post("/", rs.createRecurrenceRule)
			r.With(authorize.RequiresPermission(permissions.ActivitiesUpdate)).Put("/{id}", rs.updateRecurrenceRule)
			r.With(authorize.RequiresPermission(permissions.ActivitiesDelete)).Delete("/{id}", rs.deleteRecurrenceRule)

			// Special recurrence rule queries and operations
			r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/by-frequency", rs.getRecurrenceRulesByFrequency)
			r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Get("/by-weekday", rs.getRecurrenceRulesByWeekday)
			r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Post("/{id}/generate-events", rs.generateEvents)
		})

		// Advanced scheduling operations
		r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Post("/check-conflict", rs.checkConflict)
		r.With(authorize.RequiresPermission(permissions.ActivitiesRead)).Post("/find-available-slots", rs.findAvailableSlots)
	})

	return r
}

// Request and Response structures

// DateframeRequest represents a dateframe creation/update request
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
func (rs *Resource) parseDateframeDates(w http.ResponseWriter, r *http.Request, startStr, endStr string) (time.Time, time.Time, bool) {
	startDate, err := time.Parse(dateLayout, startStr)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidStartDate)))
		return time.Time{}, time.Time{}, false
	}

	endDate, err := time.Parse(dateLayout, endStr)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidEndDate)))
		return time.Time{}, time.Time{}, false
	}

	return startDate, endDate, true
}

// parseTimeframeTimes parses and validates start time and optional end time, handling errors internally
func (rs *Resource) parseTimeframeTimes(w http.ResponseWriter, r *http.Request, startStr string, endStr *string) (time.Time, *time.Time, bool) {
	startTime, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidStartTime)))
		return time.Time{}, nil, false
	}

	if endStr != nil {
		endTime, err := time.Parse(time.RFC3339, *endStr)
		if err != nil {
			common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidEndTime)))
			return time.Time{}, nil, false
		}
		return startTime, &endTime, true
	}

	return startTime, nil, true
}

// parseOptionalEndDate parses and validates an optional end date, handling errors internally
func (rs *Resource) parseOptionalEndDate(w http.ResponseWriter, r *http.Request, endDateStr *string) (*time.Time, bool) {
	if endDateStr == nil {
		return nil, true
	}

	endDate, err := time.Parse(dateLayout, *endDateStr)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidEndDate)))
		return nil, false
	}

	return &endDate, true
}

func newDateframeResponse(dateframe *schedule.Dateframe) DateframeResponse {
	return DateframeResponse{
		ID:          dateframe.ID,
		StartDate:   common.Time(dateframe.StartDate),
		EndDate:     common.Time(dateframe.EndDate),
		Name:        dateframe.Name,
		Description: dateframe.Description,
		CreatedAt:   common.Time(dateframe.CreatedAt),
		UpdatedAt:   common.Time(dateframe.UpdatedAt),
	}
}

func newTimeframeResponse(timeframe *schedule.Timeframe) TimeframeResponse {
	resp := TimeframeResponse{
		ID:          timeframe.ID,
		StartTime:   common.Time(timeframe.StartTime),
		IsActive:    timeframe.IsActive,
		Description: timeframe.Description,
		CreatedAt:   common.Time(timeframe.CreatedAt),
		UpdatedAt:   common.Time(timeframe.UpdatedAt),
	}

	if timeframe.EndTime != nil {
		endTime := common.Time(*timeframe.EndTime)
		resp.EndTime = &endTime
	}

	return resp
}
