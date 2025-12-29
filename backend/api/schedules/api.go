package schedules

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

const (
	errMsgRenderError               = "Error rendering error response: %v"
	errMsgInvalidDateframeID        = "invalid dateframe ID"
	errMsgInvalidTimeframeID        = "invalid timeframe ID"
	errMsgInvalidRecurrenceRuleID   = "invalid recurrence rule ID"
	errMsgRender                    = "Render error: %v"
	dateLayout                      = "2006-01-02"
	errMsgInvalidStartDate          = "invalid start date format"
	errMsgInvalidEndDate            = "invalid end date format"
	errMsgInvalidStartTime          = "invalid start time format"
	errMsgInvalidEndTime            = "invalid end time format"
	msgRecurrenceRulesRetrieved     = "Recurrence rules retrieved successfully"
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
	tokenAuth, _ := jwt.NewTokenAuth()

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
			schedule.FrequencyDaily,
			schedule.FrequencyWeekly,
			schedule.FrequencyMonthly,
			schedule.FrequencyYearly,
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
func (rs *Resource) parseDateframeDates(w http.ResponseWriter, r *http.Request, startStr, endStr string) (time.Time, time.Time, bool) {
	startDate, err := time.Parse(dateLayout, startStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidStartDate))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return time.Time{}, time.Time{}, false
	}

	endDate, err := time.Parse(dateLayout, endStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidEndDate))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return time.Time{}, time.Time{}, false
	}

	return startDate, endDate, true
}

// parseTimeframeTimes parses and validates start time and optional end time, handling errors internally
func (rs *Resource) parseTimeframeTimes(w http.ResponseWriter, r *http.Request, startStr string, endStr *string) (time.Time, *time.Time, bool) {
	startTime, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidStartTime))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return time.Time{}, nil, false
	}

	if endStr != nil {
		endTime, err := time.Parse(time.RFC3339, *endStr)
		if err != nil {
			if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidEndTime))); err != nil {
				log.Printf(errMsgRenderError, err)
			}
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
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidEndDate))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
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

func newRecurrenceRuleResponse(rule *schedule.RecurrenceRule) RecurrenceRuleResponse {
	resp := RecurrenceRuleResponse{
		ID:            rule.ID,
		Frequency:     rule.Frequency,
		IntervalCount: rule.IntervalCount,
		Weekdays:      rule.Weekdays,
		MonthDays:     rule.MonthDays,
		Count:         rule.Count,
		CreatedAt:     common.Time(rule.CreatedAt),
		UpdatedAt:     common.Time(rule.UpdatedAt),
	}

	if rule.EndDate != nil {
		endDate := common.Time(*rule.EndDate)
		resp.EndDate = &endDate
	}

	return resp
}

// Dateframe endpoints

func (rs *Resource) listDateframes(w http.ResponseWriter, r *http.Request) {
	// Create query options
	queryOptions := base.NewQueryOptions()

	// Add filters if provided
	name := r.URL.Query().Get("name")
	if name != "" {
		queryOptions.Filter.ILike("name", "%"+name+"%")
	}

	// Add pagination
	page, pageSize := common.ParsePagination(r)
	queryOptions.WithPagination(page, pageSize)

	// Get dateframes
	dateframes, err := rs.ScheduleService.ListDateframes(r.Context(), queryOptions)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Convert to response
	responses := make([]DateframeResponse, len(dateframes))
	for i, df := range dateframes {
		responses[i] = newDateframeResponse(df)
	}

	common.RespondWithPagination(w, r, http.StatusOK, responses, page, pageSize, len(responses), "Dateframes retrieved successfully")
}

func (rs *Resource) getDateframe(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidDateframeID))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Get dateframe
	dateframe, err := rs.ScheduleService.GetDateframe(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("dateframe not found"))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, newDateframeResponse(dateframe), "Dateframe retrieved successfully")
}

func (rs *Resource) createDateframe(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &DateframeRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf(errMsgRender, err)
		}
		return
	}

	// Parse and validate dates
	startDate, endDate, ok := rs.parseDateframeDates(w, r, req.StartDate, req.EndDate)
	if !ok {
		return
	}

	// Create dateframe
	dateframe := &schedule.Dateframe{
		StartDate:   startDate,
		EndDate:     endDate,
		Name:        req.Name,
		Description: req.Description,
	}

	if err := rs.ScheduleService.CreateDateframe(r.Context(), dateframe); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf(errMsgRender, err)
		}
		return
	}

	common.Respond(w, r, http.StatusCreated, newDateframeResponse(dateframe), "Dateframe created successfully")
}

func (rs *Resource) updateDateframe(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidDateframeID))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Parse request
	req := &DateframeRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf(errMsgRender, err)
		}
		return
	}

	// Get existing dateframe
	dateframe, err := rs.ScheduleService.GetDateframe(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("dateframe not found"))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Parse and validate dates
	startDate, endDate, ok := rs.parseDateframeDates(w, r, req.StartDate, req.EndDate)
	if !ok {
		return
	}

	// Update fields
	dateframe.StartDate = startDate
	dateframe.EndDate = endDate
	dateframe.Name = req.Name
	dateframe.Description = req.Description

	// Update dateframe
	if err := rs.ScheduleService.UpdateDateframe(r.Context(), dateframe); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf(errMsgRender, err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, newDateframeResponse(dateframe), "Dateframe updated successfully")
}

func (rs *Resource) deleteDateframe(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidDateframeID))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Delete dateframe
	if err := rs.ScheduleService.DeleteDateframe(r.Context(), id); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf(errMsgRender, err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Dateframe deleted successfully")
}

func (rs *Resource) getDateframesByDate(w http.ResponseWriter, r *http.Request) {
	// Get date from query param
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("date parameter is required"))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Parse date
	date, err := time.Parse(dateLayout, dateStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid date format"))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Get dateframes
	dateframes, err := rs.ScheduleService.FindDateframesByDate(r.Context(), date)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Convert to response
	responses := make([]DateframeResponse, len(dateframes))
	for i, df := range dateframes {
		responses[i] = newDateframeResponse(df)
	}

	common.Respond(w, r, http.StatusOK, responses, "Dateframes retrieved successfully")
}

func (rs *Resource) getOverlappingDateframes(w http.ResponseWriter, r *http.Request) {
	// Get dates from query params
	startStr := r.URL.Query().Get("start_date")
	endStr := r.URL.Query().Get("end_date")

	if startStr == "" || endStr == "" {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("start_date and end_date parameters are required"))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Parse dates
	startDate, err := time.Parse(dateLayout, startStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidStartDate))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	endDate, err := time.Parse(dateLayout, endStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidEndDate))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Get overlapping dateframes
	dateframes, err := rs.ScheduleService.FindOverlappingDateframes(r.Context(), startDate, endDate)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Convert to response
	responses := make([]DateframeResponse, len(dateframes))
	for i, df := range dateframes {
		responses[i] = newDateframeResponse(df)
	}

	common.Respond(w, r, http.StatusOK, responses, "Overlapping dateframes retrieved successfully")
}

func (rs *Resource) getCurrentDateframe(w http.ResponseWriter, r *http.Request) {
	// Get current dateframe
	dateframe, err := rs.ScheduleService.GetCurrentDateframe(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("no current dateframe found"))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, newDateframeResponse(dateframe), "Current dateframe retrieved successfully")
}

// Timeframe endpoints

func (rs *Resource) listTimeframes(w http.ResponseWriter, r *http.Request) {
	// Create query options
	queryOptions := base.NewQueryOptions()

	// Add filters if provided
	description := r.URL.Query().Get("description")
	if description != "" {
		queryOptions.Filter.ILike("description", "%"+description+"%")
	}

	// Add pagination
	page := 1
	pageSize := 50

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	queryOptions.WithPagination(page, pageSize)

	// Get timeframes
	timeframes, err := rs.ScheduleService.ListTimeframes(r.Context(), queryOptions)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Convert to response
	responses := make([]TimeframeResponse, len(timeframes))
	for i, tf := range timeframes {
		responses[i] = newTimeframeResponse(tf)
	}

	common.RespondWithPagination(w, r, http.StatusOK, responses, page, pageSize, len(responses), "Timeframes retrieved successfully")
}

func (rs *Resource) getTimeframe(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidTimeframeID))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Get timeframe
	timeframe, err := rs.ScheduleService.GetTimeframe(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("timeframe not found"))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, newTimeframeResponse(timeframe), "Timeframe retrieved successfully")
}

func (rs *Resource) createTimeframe(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &TimeframeRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf(errMsgRender, err)
		}
		return
	}

	// Parse start time
	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidStartTime))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Create timeframe
	timeframe := &schedule.Timeframe{
		StartTime:   startTime,
		IsActive:    req.IsActive,
		Description: req.Description,
	}

	// Parse end time if provided
	if req.EndTime != nil {
		endTime, err := time.Parse(time.RFC3339, *req.EndTime)
		if err != nil {
			if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidEndTime))); err != nil {
				log.Printf(errMsgRenderError, err)
			}
			return
		}
		timeframe.EndTime = &endTime
	}

	if err := rs.ScheduleService.CreateTimeframe(r.Context(), timeframe); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf(errMsgRender, err)
		}
		return
	}

	common.Respond(w, r, http.StatusCreated, newTimeframeResponse(timeframe), "Timeframe created successfully")
}

func (rs *Resource) updateTimeframe(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidTimeframeID))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Parse request
	req := &TimeframeRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf(errMsgRender, err)
		}
		return
	}

	// Get existing timeframe
	timeframe, err := rs.ScheduleService.GetTimeframe(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("timeframe not found"))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Parse and validate times
	startTime, endTime, ok := rs.parseTimeframeTimes(w, r, req.StartTime, req.EndTime)
	if !ok {
		return
	}

	// Update fields
	timeframe.StartTime = startTime
	timeframe.EndTime = endTime
	timeframe.IsActive = req.IsActive
	timeframe.Description = req.Description

	// Update timeframe
	if err := rs.ScheduleService.UpdateTimeframe(r.Context(), timeframe); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf(errMsgRender, err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, newTimeframeResponse(timeframe), "Timeframe updated successfully")
}

func (rs *Resource) deleteTimeframe(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidTimeframeID))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Delete timeframe
	if err := rs.ScheduleService.DeleteTimeframe(r.Context(), id); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf(errMsgRender, err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Timeframe deleted successfully")
}

func (rs *Resource) getActiveTimeframes(w http.ResponseWriter, r *http.Request) {
	// Get active timeframes
	timeframes, err := rs.ScheduleService.FindActiveTimeframes(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Convert to response
	responses := make([]TimeframeResponse, len(timeframes))
	for i, tf := range timeframes {
		responses[i] = newTimeframeResponse(tf)
	}

	common.Respond(w, r, http.StatusOK, responses, "Active timeframes retrieved successfully")
}

func (rs *Resource) getTimeframesByRange(w http.ResponseWriter, r *http.Request) {
	// Get times from query params
	startStr := r.URL.Query().Get("start_time")
	endStr := r.URL.Query().Get("end_time")

	if startStr == "" || endStr == "" {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("start_time and end_time parameters are required"))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Parse times
	startTime, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidStartTime))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	endTime, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidEndTime))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Get timeframes
	timeframes, err := rs.ScheduleService.FindTimeframesByTimeRange(r.Context(), startTime, endTime)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Convert to response
	responses := make([]TimeframeResponse, len(timeframes))
	for i, tf := range timeframes {
		responses[i] = newTimeframeResponse(tf)
	}

	common.Respond(w, r, http.StatusOK, responses, "Timeframes retrieved successfully")
}

// Recurrence rule endpoints

func (rs *Resource) listRecurrenceRules(w http.ResponseWriter, r *http.Request) {
	// Create query options
	queryOptions := base.NewQueryOptions()

	// Add filters if provided
	frequency := r.URL.Query().Get("frequency")
	if frequency != "" {
		queryOptions.Filter.Equal("frequency", frequency)
	}

	// Add pagination
	page := 1
	pageSize := 50

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	queryOptions.WithPagination(page, pageSize)

	// Get recurrence rules
	rules, err := rs.ScheduleService.ListRecurrenceRules(r.Context(), queryOptions)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Convert to response
	responses := make([]RecurrenceRuleResponse, len(rules))
	for i, rule := range rules {
		responses[i] = newRecurrenceRuleResponse(rule)
	}

	common.RespondWithPagination(w, r, http.StatusOK, responses, page, pageSize, len(responses), msgRecurrenceRulesRetrieved)
}

func (rs *Resource) getRecurrenceRule(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidRecurrenceRuleID))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Get recurrence rule
	rule, err := rs.ScheduleService.GetRecurrenceRule(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("recurrence rule not found"))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, newRecurrenceRuleResponse(rule), "Recurrence rule retrieved successfully")
}

func (rs *Resource) createRecurrenceRule(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &RecurrenceRuleRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf(errMsgRender, err)
		}
		return
	}

	// Create recurrence rule
	rule := &schedule.RecurrenceRule{
		Frequency:     req.Frequency,
		IntervalCount: req.IntervalCount,
		Weekdays:      req.Weekdays,
		MonthDays:     req.MonthDays,
		Count:         req.Count,
	}

	// Parse end date if provided
	if req.EndDate != nil {
		endDate, err := time.Parse(dateLayout, *req.EndDate)
		if err != nil {
			if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidEndDate))); err != nil {
				log.Printf(errMsgRenderError, err)
			}
			return
		}
		rule.EndDate = &endDate
	}

	if err := rs.ScheduleService.CreateRecurrenceRule(r.Context(), rule); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf(errMsgRender, err)
		}
		return
	}

	common.Respond(w, r, http.StatusCreated, newRecurrenceRuleResponse(rule), "Recurrence rule created successfully")
}

func (rs *Resource) updateRecurrenceRule(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidRecurrenceRuleID))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Parse request
	req := &RecurrenceRuleRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf(errMsgRender, err)
		}
		return
	}

	// Get existing recurrence rule
	rule, err := rs.ScheduleService.GetRecurrenceRule(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("recurrence rule not found"))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Update fields
	rule.Frequency = req.Frequency
	rule.IntervalCount = req.IntervalCount
	rule.Weekdays = req.Weekdays
	rule.MonthDays = req.MonthDays
	rule.Count = req.Count

	// Parse and validate optional end date
	endDate, ok := rs.parseOptionalEndDate(w, r, req.EndDate)
	if !ok {
		return
	}
	rule.EndDate = endDate

	// Update recurrence rule
	if err := rs.ScheduleService.UpdateRecurrenceRule(r.Context(), rule); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf(errMsgRender, err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, newRecurrenceRuleResponse(rule), "Recurrence rule updated successfully")
}

func (rs *Resource) deleteRecurrenceRule(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidRecurrenceRuleID))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Delete recurrence rule
	if err := rs.ScheduleService.DeleteRecurrenceRule(r.Context(), id); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf(errMsgRender, err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Recurrence rule deleted successfully")
}

func (rs *Resource) getRecurrenceRulesByFrequency(w http.ResponseWriter, r *http.Request) {
	// Get frequency from query param
	frequency := r.URL.Query().Get("frequency")
	if frequency == "" {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("frequency parameter is required"))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Get recurrence rules
	rules, err := rs.ScheduleService.FindRecurrenceRulesByFrequency(r.Context(), frequency)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Convert to response
	responses := make([]RecurrenceRuleResponse, len(rules))
	for i, rule := range rules {
		responses[i] = newRecurrenceRuleResponse(rule)
	}

	common.Respond(w, r, http.StatusOK, responses, msgRecurrenceRulesRetrieved)
}

func (rs *Resource) getRecurrenceRulesByWeekday(w http.ResponseWriter, r *http.Request) {
	// Get weekday from query param
	weekday := r.URL.Query().Get("weekday")
	if weekday == "" {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("weekday parameter is required"))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Get recurrence rules
	rules, err := rs.ScheduleService.FindRecurrenceRulesByWeekday(r.Context(), weekday)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Convert to response
	responses := make([]RecurrenceRuleResponse, len(rules))
	for i, rule := range rules {
		responses[i] = newRecurrenceRuleResponse(rule)
	}

	common.Respond(w, r, http.StatusOK, responses, msgRecurrenceRulesRetrieved)
}

func (rs *Resource) generateEvents(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidRecurrenceRuleID))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Parse request
	req := &GenerateEventsRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf(errMsgRender, err)
		}
		return
	}

	// Parse and validate dates
	startDate, endDate, ok := rs.parseDateframeDates(w, r, req.StartDate, req.EndDate)
	if !ok {
		return
	}

	// Generate events
	events, err := rs.ScheduleService.GenerateEvents(r.Context(), id, startDate, endDate)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Convert to response
	eventResponses := make([]common.Time, len(events))
	for i, event := range events {
		eventResponses[i] = common.Time(event)
	}

	common.Respond(w, r, http.StatusOK, map[string]interface{}{
		"events": eventResponses,
		"count":  len(events),
	}, "Events generated successfully")
}

// Advanced operations

func (rs *Resource) checkConflict(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &CheckConflictRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf(errMsgRender, err)
		}
		return
	}

	// Parse times
	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidStartTime))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidEndTime))); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Check conflict
	hasConflict, conflictingTimeframes, err := rs.ScheduleService.CheckConflict(r.Context(), startTime, endTime)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Convert conflicting timeframes to response
	conflictResponses := make([]TimeframeResponse, len(conflictingTimeframes))
	for i, tf := range conflictingTimeframes {
		conflictResponses[i] = newTimeframeResponse(tf)
	}

	common.Respond(w, r, http.StatusOK, map[string]interface{}{
		"has_conflict":           hasConflict,
		"conflicting_timeframes": conflictResponses,
	}, "Conflict check completed")
}

func (rs *Resource) findAvailableSlots(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &FindAvailableSlotsRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf(errMsgRender, err)
		}
		return
	}

	// Parse and validate dates
	startDate, endDate, ok := rs.parseDateframeDates(w, r, req.StartDate, req.EndDate)
	if !ok {
		return
	}

	// Convert duration from minutes to time.Duration
	duration := time.Duration(req.Duration) * time.Minute

	// Find available slots
	availableSlots, err := rs.ScheduleService.FindAvailableSlots(r.Context(), startDate, endDate, duration)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf(errMsgRenderError, err)
		}
		return
	}

	// Convert to response
	slotResponses := make([]TimeframeResponse, len(availableSlots))
	for i, slot := range availableSlots {
		slotResponses[i] = newTimeframeResponse(slot)
	}

	common.Respond(w, r, http.StatusOK, map[string]interface{}{
		"available_slots": slotResponses,
		"count":           len(availableSlots),
	}, "Available slots found")
}
