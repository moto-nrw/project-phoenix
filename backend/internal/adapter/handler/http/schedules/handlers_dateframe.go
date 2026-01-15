package schedules

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
)

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
