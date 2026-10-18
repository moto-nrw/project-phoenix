package httpadapter

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// SchedulesResource defines the schedules API resource
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
	if err := rs.transactions.WithinTenant(r.Context(), tenantID, func(ctx context.Context) error {
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
	if err := rs.transactions.WithinTenant(r.Context(), tenantID, func(ctx context.Context) error {
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
	if err := rs.transactions.WithinTenant(r.Context(), tenantID, func(ctx context.Context) error {
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
