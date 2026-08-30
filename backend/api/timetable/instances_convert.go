package timetable

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type convertInstanceToSeriesRequest struct {
	createTemplateRequest
	// InstanceNotes remains the Tagesnotiz of the existing seed occurrence;
	// Notes is the durable Wochennotiz of the new template.
	InstanceNotes *string `json:"instance_notes,omitempty"`
}

func (req *convertInstanceToSeriesRequest) Bind(r *http.Request) error {
	if err := req.createTemplateRequest.Bind(r); err != nil {
		return err
	}
	if req.InstanceNotes != nil && len(*req.InstanceNotes) > 2000 {
		return errors.New("instance_notes cannot exceed 2000 characters")
	}
	return nil
}

type convertInstanceToSeriesResponse struct {
	TemplateID       int64   `json:"template_id"`
	TimeframeID      int64   `json:"timeframe_id"`
	ScheduleIDs      []int64 `json:"schedule_ids"`
	LinkedInstanceID int64   `json:"linked_instance_id"`
}

// convertInstanceToSeries handles POST /instances/{id}/convert-to-series.
// Unlike the former client-side POST+PUT sequence, the service owns both
// writes in one tenant transaction.
func (rs *Resource) convertInstanceToSeries(w http.ResponseWriter, r *http.Request) {
	instanceID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid instance id")))
		return
	}
	if rs.InstanceSeriesConverter == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}

	req := &convertInstanceToSeriesRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	parsed, ok := parseBoundCreateTemplateRequest(w, r, &req.createTemplateRequest)
	if !ok {
		return
	}
	if parsed.startDate == nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("start_date is required for conversion")))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	if tenantID <= 0 {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("no tenant in context")))
		return
	}
	gradeLevelMax, rosterValidFrom, ok := rs.templateWritePreflight(
		w, r, parsed.req.CalendarPeriodID, parsed.startDate,
	)
	if !ok {
		return
	}

	result, err := rs.InstanceSeriesConverter.ConvertInstanceToSeries(
		r.Context(),
		scheduleSvc.ConvertInstanceToSeriesInput{
			InstanceID: instanceID,
			Template: buildCreateTemplateInput(
				parsed, tenantID, gradeLevelMax, rosterValidFrom, rs.resolveStartedByStaffID(r.Context()),
			),
			InstanceNotes:  normalizeNotes(req.InstanceNotes),
			ActorAccountID: jwt.ActorAccountIDFromCtx(r.Context()),
		},
	)
	if err != nil {
		// Convert runs under TenantTxMiddleware. Nested service WithTenantTx
		// reuses that outer transaction, so a 4xx after CreateTemplate would
		// otherwise commit an orphan series while the seed stays unlinked.
		tenant.MarkRollback(r.Context())
		renderConvertInstanceToSeriesError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusCreated, convertInstanceToSeriesResponse{
		TemplateID:       result.TemplateID,
		TimeframeID:      result.TimeframeID,
		ScheduleIDs:      result.ScheduleIDs,
		LinkedInstanceID: result.LinkedInstanceID,
	}, "Instance converted to series")
}

var convertInstanceToSeriesErrorRules = []common.ErrorRule{
	{Target: scheduleSvc.ErrInstanceNotFound, Render: common.ErrorNotFound},
	{
		Match: func(err error) bool {
			return errors.Is(err, scheduleSvc.ErrInvalidInstanceTransition) ||
				errors.Is(err, scheduleSvc.ErrInstanceAlreadyInSeries)
		},
		Render: func(err error) render.Renderer {
			return common.ErrorConflictWithCode(err, "instance_not_convertible")
		},
	},
	{
		Match: func(err error) bool {
			return errors.Is(err, scheduleSvc.ErrInstanceWeekend) ||
				errors.Is(err, scheduleSvc.ErrInstanceOutsideActiveCalendarPeriod) ||
				errors.Is(err, scheduleSvc.ErrOfferingSourceInvalid) ||
				errors.Is(err, scheduleSvc.ErrTemplateTargetGradeExceedsLimit)
		},
		Render: common.ErrorInvalidRequest,
	},
	{
		Target: scheduleSvc.ErrCategoryNotAssignable,
		Render: func(error) render.Renderer {
			return common.ErrorInvalidRequest(errors.New("category is archived or unavailable"))
		},
	},
	{
		Match: func(err error) bool {
			return errors.Is(err, scheduleSvc.ErrPlanningTrackNotFound) ||
				errors.Is(err, scheduleSvc.ErrPlanningTrackArchived)
		},
		Render: func(error) render.Renderer {
			return common.ErrorInvalidRequest(errors.New("planning track is archived or unavailable"))
		},
	},
	{
		Match: func(err error) bool {
			var educationGroupErr *scheduleSvc.TemplateEducationGroupError
			return errors.As(err, &educationGroupErr)
		},
		Render: common.ErrorInvalidRequest,
	},
}

func renderConvertInstanceToSeriesError(w http.ResponseWriter, r *http.Request, err error) {
	renderer := common.RenderWithRules(err, convertInstanceToSeriesErrorRules, func(err error) render.Renderer {
		return common.ErrorInternalServerWrap("convert instance to series failed", err)
	})
	common.RenderError(w, r, renderer)
}
