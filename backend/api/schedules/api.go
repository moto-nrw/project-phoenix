package schedules

import (
	"errors"
	"net/http"
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
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Convert to response
	responses := make([]DateframeResponse, len(dateframes))
	for i, df := range dateframes {
		responses[i] = newDateframeResponse(df)
	}

	common.RespondPaginated(w, r, http.StatusOK, responses, common.PaginationParams{Page: page, PageSize: pageSize, Total: len(responses)}, "Dateframes retrieved successfully")
}

func (rs *Resource) getDateframe(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidDateframeID)))
		return
	}

	// Get dateframe
	dateframe, err := rs.ScheduleService.GetDateframe(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New("dateframe not found")))
		return
	}

	common.Respond(w, r, http.StatusOK, newDateframeResponse(dateframe), "Dateframe retrieved successfully")
}

func (rs *Resource) createDateframe(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &DateframeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
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
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, newDateframeResponse(dateframe), "Dateframe created successfully")
}

func (rs *Resource) updateDateframe(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidDateframeID)))
		return
	}

	// Parse request
	req := &DateframeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get existing dateframe
	dateframe, err := rs.ScheduleService.GetDateframe(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New("dateframe not found")))
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
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newDateframeResponse(dateframe), "Dateframe updated successfully")
}

func (rs *Resource) deleteDateframe(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidDateframeID)))
		return
	}

	// Delete dateframe
	if err := rs.ScheduleService.DeleteDateframe(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Dateframe deleted successfully")
}

func (rs *Resource) getDateframesByDate(w http.ResponseWriter, r *http.Request) {
	// Get date from query param
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("date parameter is required")))
		return
	}

	// Parse date
	date, err := time.Parse(dateLayout, dateStr)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid date format")))
		return
	}

	// Get dateframes
	dateframes, err := rs.ScheduleService.FindDateframesByDate(r.Context(), date)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
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
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("start_date and end_date parameters are required")))
		return
	}

	// Parse dates
	startDate, err := time.Parse(dateLayout, startStr)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidStartDate)))
		return
	}

	endDate, err := time.Parse(dateLayout, endStr)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidEndDate)))
		return
	}

	// Get overlapping dateframes
	dateframes, err := rs.ScheduleService.FindOverlappingDateframes(r.Context(), startDate, endDate)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
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
		common.RenderError(w, r, ErrorNotFound(errors.New("no current dateframe found")))
		return
	}

	common.Respond(w, r, http.StatusOK, newDateframeResponse(dateframe), "Current dateframe retrieved successfully")
}

// Advanced operations

func (rs *Resource) checkConflict(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &CheckConflictRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Parse times
	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidStartTime)))
		return
	}

	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidEndTime)))
		return
	}

	// Check conflict
	hasConflict, conflictingTimeframes, err := rs.ScheduleService.CheckConflict(r.Context(), startTime, endTime)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
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
		common.RenderError(w, r, ErrorInvalidRequest(err))
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
		common.RenderError(w, r, ErrorInternalServer(err))
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
