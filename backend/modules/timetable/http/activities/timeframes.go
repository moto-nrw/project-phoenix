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
	if err := rs.transactions.WithinTenant(r.Context(), tenantID, func(ctx context.Context) error {
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
	if err := rs.transactions.WithinTenant(r.Context(), tenantID, func(ctx context.Context) error {
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
	if err := rs.transactions.WithinTenant(r.Context(), tenantID, func(ctx context.Context) error {
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
