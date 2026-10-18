package httpadapter

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// SchedulesResource defines the schedules API resource
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
	if err := rs.transactions.WithinTenant(r.Context(), tenantID, func(ctx context.Context) error {
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
	if err := rs.transactions.WithinTenant(r.Context(), tenantID, func(ctx context.Context) error {
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
	if err := rs.transactions.WithinTenant(r.Context(), tenantID, func(ctx context.Context) error {
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
