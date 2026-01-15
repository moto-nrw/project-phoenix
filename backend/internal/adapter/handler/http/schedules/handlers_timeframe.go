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
	page, pageSize := common.ParsePagination(r)
	queryOptions.WithPagination(page, pageSize)

	// Get timeframes
	timeframes, err := rs.ScheduleService.ListTimeframes(r.Context(), queryOptions)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Convert to response
	responses := make([]TimeframeResponse, len(timeframes))
	for i, tf := range timeframes {
		responses[i] = newTimeframeResponse(tf)
	}

	common.RespondPaginated(w, r, http.StatusOK, responses, common.PaginationParams{Page: page, PageSize: pageSize, Total: len(responses)}, "Timeframes retrieved successfully")
}

func (rs *Resource) getTimeframe(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidTimeframeID)))
		return
	}

	// Get timeframe
	timeframe, err := rs.ScheduleService.GetTimeframe(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New("timeframe not found")))
		return
	}

	common.Respond(w, r, http.StatusOK, newTimeframeResponse(timeframe), "Timeframe retrieved successfully")
}

func (rs *Resource) createTimeframe(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &TimeframeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Parse and validate times
	startTime, endTime, ok := rs.parseTimeframeTimes(w, r, req.StartTime, req.EndTime)
	if !ok {
		return
	}

	// Create timeframe
	timeframe := &schedule.Timeframe{
		StartTime:   startTime,
		EndTime:     endTime,
		IsActive:    req.IsActive,
		Description: req.Description,
	}

	if err := rs.ScheduleService.CreateTimeframe(r.Context(), timeframe); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, newTimeframeResponse(timeframe), "Timeframe created successfully")
}

func (rs *Resource) updateTimeframe(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidTimeframeID)))
		return
	}

	// Parse request
	req := &TimeframeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get existing timeframe
	timeframe, err := rs.ScheduleService.GetTimeframe(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New("timeframe not found")))
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
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newTimeframeResponse(timeframe), "Timeframe updated successfully")
}

func (rs *Resource) deleteTimeframe(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidTimeframeID)))
		return
	}

	// Delete timeframe
	if err := rs.ScheduleService.DeleteTimeframe(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Timeframe deleted successfully")
}

func (rs *Resource) getActiveTimeframes(w http.ResponseWriter, r *http.Request) {
	// Get active timeframes
	timeframes, err := rs.ScheduleService.FindActiveTimeframes(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
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
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("start_time and end_time parameters are required")))
		return
	}

	// Parse times
	startTime, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidStartTime)))
		return
	}

	endTime, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidEndTime)))
		return
	}

	// Get timeframes
	timeframes, err := rs.ScheduleService.FindTimeframesByTimeRange(r.Context(), startTime, endTime)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Convert to response
	responses := make([]TimeframeResponse, len(timeframes))
	for i, tf := range timeframes {
		responses[i] = newTimeframeResponse(tf)
	}

	common.Respond(w, r, http.StatusOK, responses, "Timeframes retrieved successfully")
}
