package careplanhttp

import (
	errors "errors"
	http "net/http"

	render "github.com/go-chi/render"
	common "github.com/moto-nrw/project-phoenix/api/common"
	careplan "github.com/moto-nrw/project-phoenix/modules/careplan"
	carerequests "github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

// getCareScheduleChangeRequest serves GET
// .../care-schedule-change-requests/{requestId}: one request of any status
// for a reader the review policy allows for the child. A row of another
// school is not found; a child outside the reader's scope is forbidden.
func (rs *Resource) getCareScheduleChangeRequest(w http.ResponseWriter, r *http.Request) {
	if rs.CareRequestReviews == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("care request service not configured")))
		return
	}
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "requestId", "invalid request id")
	if !ok {
		return
	}
	item, err := rs.CareRequestReviews.GetForReview(r.Context(), requestID)
	if err != nil {
		renderError(w, r, careRequestDetailErrorRenderer(err))
		return
	}
	rs.WriteCareDetail(w, r, item)
}

var careRequestDetailErrorRenderer = common.RulesRenderer(parentRequestRules(
	common.ErrorRule{Target: careplan.ErrCareScheduleRequestNotFound, Render: careRequestDetailNotFound},
	common.ErrorRule{Target: carerequests.ErrCareRequestForbidden, Render: common.ErrorForbidden},
), common.ErrorInternalServer)

// careRequestDetailNotFound names a withdrawn or deleted request by its code,
// so the detail dialog explains it without reading the HTTP status (#2517).
func careRequestDetailNotFound(err error) render.Renderer {
	return common.ErrorNotFoundWithCode(err, common.CodeCareRequestNotFound)
}
