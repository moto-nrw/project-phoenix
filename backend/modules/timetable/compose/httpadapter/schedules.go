package httpadapter

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// SchedulesResource defines the schedules API resource
type SchedulesResource struct {
	Dateframes     Dateframes
	Timeframes     Timeframes
	TimeframeGuard TimeframeChangeGuard
	db             *bun.DB
}

// NewSchedulesResource creates a new schedules resource
func NewSchedulesResource(dateframes Dateframes, timeframes Timeframes, guard TimeframeChangeGuard, db *bun.DB) *SchedulesResource {
	return &SchedulesResource{
		Dateframes:     dateframes,
		Timeframes:     timeframes,
		TimeframeGuard: guard,
		db:             db,
	}
}

// Router returns a configured router for schedule endpoints
func (rs *SchedulesResource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Protected routes that require authentication and permissions
	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {

		// Current dateframe endpoint - requires schedules:read permission
		r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).Get("/current-dateframe", rs.getCurrentDateframe)

		// Dateframe endpoints
		r.Route("/dateframes", func(r chi.Router) {
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/", rs.listDateframes)
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/{id}", rs.getDateframe)
			r.With(common.RequiresPermission(permissions.ActivitiesCreate), withTx).Post("/", rs.createDateframe)
			r.With(common.RequiresPermission(permissions.ActivitiesUpdate), withTx).Put("/{id}", rs.updateDateframe)
			r.With(common.RequiresPermission(permissions.ActivitiesDelete), withTx).Delete("/{id}", rs.deleteDateframe)

			// Special dateframe queries
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/by-date", rs.getDateframesByDate)
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/overlapping", rs.getOverlappingDateframes)
		})

		// Timeframe endpoints
		r.Route("/timeframes", func(r chi.Router) {
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/", rs.listTimeframes)
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/{id}", rs.getTimeframe)
			r.With(common.RequiresPermission(permissions.ActivitiesCreate), withTx).Post("/", rs.createTimeframe)
			r.With(common.RequiresPermission(permissions.ActivitiesUpdate), withTx).Put("/{id}", rs.updateTimeframe)
			r.With(common.RequiresPermission(permissions.ActivitiesDelete), withTx).Delete("/{id}", rs.deleteTimeframe)

			// Special timeframe queries
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/active", rs.getActiveTimeframes)
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/by-range", rs.getTimeframesByRange)
		})

		// Recurrence rule endpoints
		r.Route("/recurrence-rules", func(r chi.Router) {
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/", rs.listRecurrenceRules)
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/{id}", rs.getRecurrenceRule)
			r.With(common.RequiresPermission(permissions.ActivitiesCreate), withTx).Post("/", rs.createRecurrenceRule)
			r.With(common.RequiresPermission(permissions.ActivitiesUpdate), withTx).Put("/{id}", rs.updateRecurrenceRule)
			r.With(common.RequiresPermission(permissions.ActivitiesDelete), withTx).Delete("/{id}", rs.deleteRecurrenceRule)

			// Special recurrence rule queries and operations
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/by-frequency", rs.getRecurrenceRulesByFrequency)
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/by-weekday", rs.getRecurrenceRulesByWeekday)
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Post("/{id}/generate-events", rs.generateEvents)
		})

		// Advanced scheduling operations
		r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Post("/check-conflict", rs.checkConflict)
		r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Post("/find-available-slots", rs.findAvailableSlots)
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
func (rs *SchedulesResource) parseDateframeDates(w http.ResponseWriter, r *http.Request, startStr, endStr string) (time.Time, time.Time, bool) {
	startDate, err := time.Parse(scheduleDateLayout, startStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidStartDate)))
		return time.Time{}, time.Time{}, false
	}

	endDate, err := time.Parse(scheduleDateLayout, endStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidEndDate)))
		return time.Time{}, time.Time{}, false
	}

	return startDate, endDate, true
}

// parseTimeframeTimes parses and validates start time and optional end time, handling errors internally
func (rs *SchedulesResource) parseTimeframeTimes(w http.ResponseWriter, r *http.Request, startStr string, endStr *string) (time.Time, *time.Time, bool) {
	startTime, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidStartTime)))
		return time.Time{}, nil, false
	}

	if endStr != nil {
		endTime, err := time.Parse(time.RFC3339, *endStr)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidEndTime)))
			return time.Time{}, nil, false
		}
		return startTime, &endTime, true
	}

	return startTime, nil, true
}

// parseOptionalEndDate parses and validates an optional end date, handling errors internally
func (rs *SchedulesResource) parseOptionalEndDate(w http.ResponseWriter, r *http.Request, endDateStr *string) (*time.Time, bool) {
	if endDateStr == nil {
		return nil, true
	}

	endDate, err := time.Parse(scheduleDateLayout, *endDateStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidEndDate)))
		return nil, false
	}

	return &endDate, true
}

func newDateframeResponse(dateframe schoolcalendar.Dateframe) DateframeResponse {
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

func newTimeframeResponse(timeframe timeframeSlot) TimeframeResponse {
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

func newRecurrenceRuleResponse(rule timetableModule.RecurrenceRule) RecurrenceRuleResponse {
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

func (rs *SchedulesResource) listDateframes(w http.ResponseWriter, r *http.Request) {
	filter := schoolcalendar.DateframeFilter{}
	if name := r.URL.Query().Get("name"); name != "" {
		filter.NamePattern = "%" + name + "%"
	}
	page, pageSize := common.ParsePagination(r)
	filter.Limit, filter.Offset = pageSize, (page-1)*pageSize

	dateframes, err := rs.Dateframes.ListDateframes(r.Context(), filter)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Convert to response
	responses := make([]DateframeResponse, len(dateframes))
	for i, df := range dateframes {
		responses[i] = newDateframeResponse(df)
	}

	common.RespondPaginated(w, r, http.StatusOK, responses, common.PaginationParams{Page: page, PageSize: pageSize, Total: len(responses)}, "Dateframes retrieved successfully")
}

func (rs *SchedulesResource) getDateframe(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidDateframeID)))
		return
	}

	// Get dateframe
	dateframe, err := rs.Dateframes.FindDateframe(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, scheduleLookupError(err, "dateframe not found"))
		return
	}

	common.Respond(w, r, http.StatusOK, newDateframeResponse(dateframe), "Dateframe retrieved successfully")
}

func (rs *SchedulesResource) createDateframe(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &DateframeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Parse and validate dates
	startDate, endDate, ok := rs.parseDateframeDates(w, r, req.StartDate, req.EndDate)
	if !ok {
		return
	}

	// Create dateframe
	var dateframe schoolcalendar.Dateframe
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		created, err := rs.Dateframes.CreateDateframe(ctx, schoolcalendar.CreateDateframe{DateframeFields: schoolcalendar.DateframeFields{
			StartDate: startDate, EndDate: endDate, Name: req.Name, Description: req.Description,
		}})
		dateframe = created
		return err
	}); err != nil {
		common.RenderError(w, r, SchedulesErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, newDateframeResponse(dateframe), "Dateframe created successfully")
}

func (rs *SchedulesResource) updateDateframe(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidDateframeID)))
		return
	}

	// Parse request
	req := &DateframeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Get existing dateframe
	dateframe, err := rs.Dateframes.FindDateframe(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, scheduleLookupError(err, "dateframe not found"))
		return
	}

	// Parse and validate dates
	startDate, endDate, ok := rs.parseDateframeDates(w, r, req.StartDate, req.EndDate)
	if !ok {
		return
	}

	// Update dateframe
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		updated, err := rs.Dateframes.UpdateDateframe(ctx, schoolcalendar.UpdateDateframe{ID: dateframe.ID, DateframeFields: schoolcalendar.DateframeFields{
			StartDate: startDate, EndDate: endDate, Name: req.Name, Description: req.Description,
		}})
		if err == nil {
			dateframe = updated
		}
		return err
	}); err != nil {
		common.RenderError(w, r, SchedulesErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newDateframeResponse(dateframe), "Dateframe updated successfully")
}

func (rs *SchedulesResource) deleteDateframe(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidDateframeID)))
		return
	}

	// Delete dateframe
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.Dateframes.DeleteDateframe(ctx, id)
	}); err != nil {
		common.RenderError(w, r, SchedulesErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Dateframe deleted successfully")
}

func (rs *SchedulesResource) getDateframesByDate(w http.ResponseWriter, r *http.Request) {
	// Get date from query param
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("date parameter is required")))
		return
	}

	// Parse date
	date, err := time.Parse(scheduleDateLayout, dateStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid date format")))
		return
	}

	// Get dateframes holding the day
	instant := dateframeMidnight(date)
	dateframes, err := rs.Dateframes.ListDateframes(r.Context(), schoolcalendar.DateframeFilter{Contains: &instant})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Convert to response
	responses := make([]DateframeResponse, len(dateframes))
	for i, df := range dateframes {
		responses[i] = newDateframeResponse(df)
	}

	common.Respond(w, r, http.StatusOK, responses, "Dateframes retrieved successfully")
}

func (rs *SchedulesResource) getOverlappingDateframes(w http.ResponseWriter, r *http.Request) {
	// Get dates from query params
	startStr := r.URL.Query().Get("start_date")
	endStr := r.URL.Query().Get("end_date")

	if startStr == "" || endStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("start_date and end_date parameters are required")))
		return
	}

	// Parse dates
	startDate, err := time.Parse(scheduleDateLayout, startStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidStartDate)))
		return
	}

	endDate, err := time.Parse(scheduleDateLayout, endStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidEndDate)))
		return
	}

	// Get overlapping dateframes
	if startDate.After(endDate) {
		common.RenderError(w, r, common.ErrorInternalServer(timetableModule.ErrInvalidRecurrenceRange))
		return
	}
	from, to := dateframeMidnight(startDate), dateframeMidnight(endDate)
	dateframes, err := rs.Dateframes.ListDateframes(r.Context(), schoolcalendar.DateframeFilter{OverlappingFrom: &from, OverlappingTo: &to})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Convert to response
	responses := make([]DateframeResponse, len(dateframes))
	for i, df := range dateframes {
		responses[i] = newDateframeResponse(df)
	}

	common.Respond(w, r, http.StatusOK, responses, "Overlapping dateframes retrieved successfully")
}

func (rs *SchedulesResource) getCurrentDateframe(w http.ResponseWriter, r *http.Request) {
	// Get current dateframe: the first range holding today. If multiple
	// dateframes are active, the listing order (by ID) decides.
	now := dateframeMidnight(time.Now())
	dateframes, err := rs.Dateframes.ListDateframes(r.Context(), schoolcalendar.DateframeFilter{Contains: &now})
	if err != nil {
		common.RenderError(w, r, scheduleLookupError(err, "no current dateframe found"))
		return
	}
	if len(dateframes) == 0 {
		common.RenderError(w, r, scheduleLookupError(schoolcalendar.ErrDateframeNotFound, "no current dateframe found"))
		return
	}

	common.Respond(w, r, http.StatusOK, newDateframeResponse(dateframes[0]), "Current dateframe retrieved successfully")
}

// Timeframe endpoints

func (rs *SchedulesResource) listTimeframes(w http.ResponseWriter, r *http.Request) {
	filter := timetableModule.TimeframeFilter{DescriptionContains: r.URL.Query().Get("description")}
	page, pageSize := common.ParsePagination(r)
	filter.Limit, filter.Offset = pageSize, (page-1)*pageSize

	timeframes, err := rs.listTimeframeSlots(r.Context(), filter)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Convert to response
	responses := make([]TimeframeResponse, len(timeframes))
	for i, tf := range timeframes {
		responses[i] = newTimeframeResponse(tf)
	}

	common.RespondPaginated(w, r, http.StatusOK, responses, common.PaginationParams{Page: page, PageSize: pageSize, Total: len(responses)}, "Timeframes retrieved successfully")
}

func (rs *SchedulesResource) getTimeframe(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidTimeframeID)))
		return
	}

	// Get timeframe
	timeframe, err := rs.findTimeframeSlot(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, scheduleLookupError(err, "timeframe not found"))
		return
	}

	common.Respond(w, r, http.StatusOK, newTimeframeResponse(timeframe), "Timeframe retrieved successfully")
}

func (rs *SchedulesResource) createTimeframe(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &TimeframeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Parse and validate times
	startTime, endTime, ok := rs.parseTimeframeTimes(w, r, req.StartTime, req.EndTime)
	if !ok {
		return
	}

	// Create timeframe
	var timeframe timeframeSlot
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		created, err := rs.Timeframes.CreateTimeframe(ctx, timeframeInput(startTime, endTime, req.IsActive, req.Description))
		if err != nil {
			return err
		}
		timeframe, err = timeframeToSlot(created)
		return err
	}); err != nil {
		common.RenderError(w, r, SchedulesErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, newTimeframeResponse(timeframe), "Timeframe created successfully")
}

func (rs *SchedulesResource) updateTimeframe(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidTimeframeID)))
		return
	}

	// Parse request
	req := &TimeframeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Get existing timeframe
	timeframe, err := rs.findTimeframeSlot(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, scheduleLookupError(err, "timeframe not found"))
		return
	}

	// Parse and validate times
	startTime, endTime, ok := rs.parseTimeframeTimes(w, r, req.StartTime, req.EndTime)
	if !ok {
		return
	}

	// Update timeframe: the care-offering guard sees the replacement first.
	input := timeframeInput(startTime, endTime, req.IsActive, req.Description)
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := rs.guardTimeframeChange(ctx, timeframe.ID, &input); err != nil {
			return err
		}
		updated, err := rs.Timeframes.UpdateTimeframe(ctx, timeframe.ID, input)
		if err != nil {
			return err
		}
		timeframe, err = timeframeToSlot(updated)
		return err
	}); err != nil {
		common.RenderError(w, r, SchedulesErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newTimeframeResponse(timeframe), "Timeframe updated successfully")
}

func (rs *SchedulesResource) deleteTimeframe(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidTimeframeID)))
		return
	}

	// Delete timeframe: the care-offering guard runs before the row goes.
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := rs.guardTimeframeChange(ctx, id, nil); err != nil {
			return err
		}
		return rs.Timeframes.DeleteTimeframe(ctx, id)
	}); err != nil {
		common.RenderError(w, r, SchedulesErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Timeframe deleted successfully")
}

func (rs *SchedulesResource) getActiveTimeframes(w http.ResponseWriter, r *http.Request) {
	// Get active timeframes
	timeframes, err := rs.listTimeframeSlots(r.Context(), timetableModule.TimeframeFilter{ActiveOnly: true})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Convert to response
	responses := make([]TimeframeResponse, len(timeframes))
	for i, tf := range timeframes {
		responses[i] = newTimeframeResponse(tf)
	}

	common.Respond(w, r, http.StatusOK, responses, "Active timeframes retrieved successfully")
}

func (rs *SchedulesResource) getTimeframesByRange(w http.ResponseWriter, r *http.Request) {
	// Get times from query params
	startStr := r.URL.Query().Get("start_time")
	endStr := r.URL.Query().Get("end_time")

	if startStr == "" || endStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("start_time and end_time parameters are required")))
		return
	}

	// Parse times
	startTime, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidStartTime)))
		return
	}

	endTime, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidEndTime)))
		return
	}

	// Get timeframes
	if !endTime.IsZero() && startTime.After(endTime) {
		common.RenderError(w, r, common.ErrorInternalServer(timetableModule.ErrInvalidTimeRange))
		return
	}
	timeframes, err := overlappingTimeframes(r.Context(), rs.Timeframes, startTime, endTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
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

func (rs *SchedulesResource) listRecurrenceRules(w http.ResponseWriter, r *http.Request) {
	filter := timetableModule.RecurrenceRuleFilter{Frequency: r.URL.Query().Get("frequency")}
	page, pageSize := common.ParsePagination(r)
	filter.Limit, filter.Offset = pageSize, (page-1)*pageSize

	rules, err := rs.Timeframes.ListRecurrenceRules(r.Context(), filter)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Convert to response
	responses := make([]RecurrenceRuleResponse, len(rules))
	for i, rule := range rules {
		responses[i] = newRecurrenceRuleResponse(rule)
	}

	common.RespondPaginated(w, r, http.StatusOK, responses, common.PaginationParams{Page: page, PageSize: pageSize, Total: len(responses)}, msgRecurrenceRulesRetrieved)
}

func (rs *SchedulesResource) getRecurrenceRule(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidRecurrenceRuleID)))
		return
	}

	// Get recurrence rule
	rule, err := rs.Timeframes.FindRecurrenceRule(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, scheduleLookupError(err, "recurrence rule not found"))
		return
	}

	common.Respond(w, r, http.StatusOK, newRecurrenceRuleResponse(rule), "Recurrence rule retrieved successfully")
}

func (rs *SchedulesResource) createRecurrenceRule(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &RecurrenceRuleRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Create recurrence rule
	input := timetableModule.RecurrenceRuleInput{
		Frequency:     req.Frequency,
		IntervalCount: req.IntervalCount,
		Weekdays:      req.Weekdays,
		MonthDays:     req.MonthDays,
		Count:         req.Count,
	}

	// Parse end date if provided
	if req.EndDate != nil {
		endDate, err := time.Parse(scheduleDateLayout, *req.EndDate)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidEndDate)))
			return
		}
		input.EndDate = &endDate
	}

	var rule timetableModule.RecurrenceRule
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		created, err := rs.Timeframes.CreateRecurrenceRule(ctx, input)
		rule = created
		return err
	}); err != nil {
		common.RenderError(w, r, SchedulesErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, newRecurrenceRuleResponse(rule), "Recurrence rule created successfully")
}

func (rs *SchedulesResource) updateRecurrenceRule(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidRecurrenceRuleID)))
		return
	}

	// Parse request
	req := &RecurrenceRuleRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Get existing recurrence rule
	rule, err := rs.Timeframes.FindRecurrenceRule(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, scheduleLookupError(err, "recurrence rule not found"))
		return
	}

	// Parse and validate optional end date
	endDate, ok := rs.parseOptionalEndDate(w, r, req.EndDate)
	if !ok {
		return
	}
	input := timetableModule.RecurrenceRuleInput{
		Frequency:     req.Frequency,
		IntervalCount: req.IntervalCount,
		Weekdays:      req.Weekdays,
		MonthDays:     req.MonthDays,
		EndDate:       endDate,
		Count:         req.Count,
	}

	// Update recurrence rule
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		updated, err := rs.Timeframes.UpdateRecurrenceRule(ctx, rule.ID, input)
		if err == nil {
			rule = updated
		}
		return err
	}); err != nil {
		common.RenderError(w, r, SchedulesErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newRecurrenceRuleResponse(rule), "Recurrence rule updated successfully")
}

func (rs *SchedulesResource) deleteRecurrenceRule(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidRecurrenceRuleID)))
		return
	}

	// Delete recurrence rule
	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.Timeframes.DeleteRecurrenceRule(ctx, id)
	}); err != nil {
		common.RenderError(w, r, SchedulesErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Recurrence rule deleted successfully")
}

func (rs *SchedulesResource) getRecurrenceRulesByFrequency(w http.ResponseWriter, r *http.Request) {
	// Get frequency from query param
	frequency := r.URL.Query().Get("frequency")
	if frequency == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("frequency parameter is required")))
		return
	}

	// Get recurrence rules
	rules, err := rs.Timeframes.ListRecurrenceRules(r.Context(), timetableModule.RecurrenceRuleFilter{Frequency: frequency})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Convert to response
	responses := make([]RecurrenceRuleResponse, len(rules))
	for i, rule := range rules {
		responses[i] = newRecurrenceRuleResponse(rule)
	}

	common.Respond(w, r, http.StatusOK, responses, msgRecurrenceRulesRetrieved)
}

func (rs *SchedulesResource) getRecurrenceRulesByWeekday(w http.ResponseWriter, r *http.Request) {
	// Get weekday from query param
	weekday := r.URL.Query().Get("weekday")
	if weekday == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("weekday parameter is required")))
		return
	}

	// Get recurrence rules
	rules, err := rs.Timeframes.ListRecurrenceRules(r.Context(), timetableModule.RecurrenceRuleFilter{Weekday: weekday})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Convert to response
	responses := make([]RecurrenceRuleResponse, len(rules))
	for i, rule := range rules {
		responses[i] = newRecurrenceRuleResponse(rule)
	}

	common.Respond(w, r, http.StatusOK, responses, msgRecurrenceRulesRetrieved)
}

func (rs *SchedulesResource) generateEvents(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidRecurrenceRuleID)))
		return
	}

	// Parse request
	req := &GenerateEventsRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Parse and validate dates
	startDate, endDate, ok := rs.parseDateframeDates(w, r, req.StartDate, req.EndDate)
	if !ok {
		return
	}

	// Generate events
	events, err := rs.Timeframes.GenerateRecurrenceEvents(r.Context(), id, startDate, endDate)
	if err != nil {
		common.RenderError(w, r, SchedulesErrorRenderer(err))
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

func (rs *SchedulesResource) checkConflict(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &CheckConflictRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Parse times
	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidStartTime)))
		return
	}

	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidEndTime)))
		return
	}

	// Check conflict
	hasConflict, conflictingTimeframes, err := checkTimeframeConflict(r.Context(), rs.Timeframes, startTime, endTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
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

func (rs *SchedulesResource) findAvailableSlots(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &FindAvailableSlotsRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
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
	availableSlots, err := availableTimeframeSlots(r.Context(), rs.Timeframes, startDate, endDate, duration)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
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
