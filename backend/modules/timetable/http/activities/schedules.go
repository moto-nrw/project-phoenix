package httpadapter

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
)

// SchedulesResource serves the timeframe list the activity forms offer.
type SchedulesResource struct {
	Timeframes timetableModule.TimeframeQuery
}

// NewSchedulesResource creates a new schedules resource
func NewSchedulesResource(timeframes timetableModule.TimeframeQuery) *SchedulesResource {
	return &SchedulesResource{Timeframes: timeframes}
}

// Router returns a configured router for schedule endpoints
func (rs *SchedulesResource) Router(protectedTenantRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	protectedTenantRoutes(r, func(r chi.Router, withTx common.Middleware) {
		r.Route("/timeframes", func(r chi.Router) {
			r.With(common.RequiresPermission(permissions.ActivitiesRead), withTx).Get("/", rs.listTimeframes)
		})
	})

	return r
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

func (rs *SchedulesResource) listTimeframes(w http.ResponseWriter, r *http.Request) {
	filter := timetableModule.TimeframeFilter{DescriptionContains: r.URL.Query().Get("description")}
	page, pageSize := common.ParsePagination(r)
	filter.Limit, filter.Offset = pageSize, (page-1)*pageSize

	values, err := rs.Timeframes.ListTimeframes(r.Context(), filter)
	var timeframes []timeframeSlot
	if err == nil {
		timeframes, err = timeframesToSlots(values)
	}
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]TimeframeResponse, len(timeframes))
	for i, tf := range timeframes {
		responses[i] = newTimeframeResponse(tf)
	}

	common.RespondPaginated(w, r, http.StatusOK, responses, common.PaginationParams{Page: page, PageSize: pageSize, Total: len(responses)}, "Timeframes retrieved successfully")
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
