package schedules

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/schedule"
)

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

func (rs *Resource) listRecurrenceRules(w http.ResponseWriter, r *http.Request) {
	// Create query options
	queryOptions := base.NewQueryOptions()

	// Add filters if provided
	frequency := r.URL.Query().Get("frequency")
	if frequency != "" {
		queryOptions.Filter.Equal("frequency", frequency)
	}

	// Add pagination
	page, pageSize := common.ParsePagination(r)
	queryOptions.WithPagination(page, pageSize)

	// Get recurrence rules
	rules, err := rs.ScheduleService.ListRecurrenceRules(r.Context(), queryOptions)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Convert to response
	responses := make([]RecurrenceRuleResponse, len(rules))
	for i, rule := range rules {
		responses[i] = newRecurrenceRuleResponse(rule)
	}

	common.RespondPaginated(w, r, http.StatusOK, responses, common.PaginationParams{Page: page, PageSize: pageSize, Total: len(responses)}, msgRecurrenceRulesRetrieved)
}

func (rs *Resource) getRecurrenceRule(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidRecurrenceRuleID)))
		return
	}

	// Get recurrence rule
	rule, err := rs.ScheduleService.GetRecurrenceRule(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New("recurrence rule not found")))
		return
	}

	common.Respond(w, r, http.StatusOK, newRecurrenceRuleResponse(rule), "Recurrence rule retrieved successfully")
}

func (rs *Resource) createRecurrenceRule(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &RecurrenceRuleRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
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
			common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidEndDate)))
			return
		}
		rule.EndDate = &endDate
	}

	if err := rs.ScheduleService.CreateRecurrenceRule(r.Context(), rule); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, newRecurrenceRuleResponse(rule), "Recurrence rule created successfully")
}

func (rs *Resource) updateRecurrenceRule(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidRecurrenceRuleID)))
		return
	}

	// Parse request
	req := &RecurrenceRuleRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get existing recurrence rule
	rule, err := rs.ScheduleService.GetRecurrenceRule(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New("recurrence rule not found")))
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
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newRecurrenceRuleResponse(rule), "Recurrence rule updated successfully")
}

func (rs *Resource) deleteRecurrenceRule(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidRecurrenceRuleID)))
		return
	}

	// Delete recurrence rule
	if err := rs.ScheduleService.DeleteRecurrenceRule(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Recurrence rule deleted successfully")
}

func (rs *Resource) getRecurrenceRulesByFrequency(w http.ResponseWriter, r *http.Request) {
	// Get frequency from query param
	frequency := r.URL.Query().Get("frequency")
	if frequency == "" {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("frequency parameter is required")))
		return
	}

	// Get recurrence rules
	rules, err := rs.ScheduleService.FindRecurrenceRulesByFrequency(r.Context(), frequency)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
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
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("weekday parameter is required")))
		return
	}

	// Get recurrence rules
	rules, err := rs.ScheduleService.FindRecurrenceRulesByWeekday(r.Context(), weekday)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
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
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidRecurrenceRuleID)))
		return
	}

	// Parse request
	req := &GenerateEventsRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
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
		common.RenderError(w, r, ErrorRenderer(err))
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
