package timetablehttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
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

	req, parsed, ok := readConvertInstanceToSeriesRequest(w, r)
	if !ok {
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
	if !rs.validateSeriesEnd(w, r, parsed.endDate, rosterValidFrom, parsed.req.CalendarPeriodID) {
		return
	}

	result, err := rs.InstanceSeriesConverter.ConvertInstanceToSeries(
		r.Context(),
		timetableModule.ConvertInstanceToSeriesInput{
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

// readConvertInstanceToSeriesRequest binds and parses the series the instance
// becomes. It returns ok=false after rendering the 400 for an invalid body or
// a missing start date.
func readConvertInstanceToSeriesRequest(w http.ResponseWriter, r *http.Request) (*convertInstanceToSeriesRequest, *parsedCreateTemplate, bool) {
	req := &convertInstanceToSeriesRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, bindErrorRenderer(err))
		return nil, nil, false
	}
	parsed, ok := parseBoundCreateTemplateRequest(w, r, &req.createTemplateRequest)
	if !ok {
		return nil, nil, false
	}
	if parsed.startDate == nil {
		common.RenderError(w, r, invalidOnField(common.CodeTimetableTemplateInvalid, "start_date", "start_date is required for conversion"))
		return nil, nil, false
	}
	return req, parsed, true
}

var convertInstanceToSeriesErrorRules = []common.ErrorRule{
	{Target: timetableModule.ErrInstanceNotFound, Render: notFoundWithCode(common.CodeTimetableInstanceNotFound)},
	{
		Match: func(err error) bool {
			return errors.Is(err, timetableModule.ErrInvalidInstanceTransition) ||
				errors.Is(err, timetableModule.ErrInstanceAlreadyInSeries)
		},
		Render: conflictWithCode(common.CodeTimetableInstanceNotConvertible),
	},
	{Target: timetableModule.ErrInstanceWeekend, Render: invalidWithCode(common.CodeTimetableInstanceWeekend)},
	{Target: timetableModule.ErrInstanceOutsideActiveCalendarPeriod, Render: invalidWithCode(common.CodeTimetableInstanceOutsidePeriod)},
}

func renderConvertInstanceToSeriesError(w http.ResponseWriter, r *http.Request, err error) {
	renderer := common.RenderWithRules(err, convertInstanceToSeriesErrorRules, func(err error) render.Renderer {
		if refusal := templateRefusalRenderer(err); refusal != nil {
			return refusal
		}
		return common.ErrorInternalServerWrap("convert instance to series failed", err)
	})
	common.RenderError(w, r, renderer)
}
