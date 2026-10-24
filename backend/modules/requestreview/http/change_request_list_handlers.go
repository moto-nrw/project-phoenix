package requestreviewhttp

import (
	errors "errors"
	http "net/http"

	common "github.com/moto-nrw/project-phoenix/api/common"
	requestreview "github.com/moto-nrw/project-phoenix/modules/requestreview"
)

// listAggregatedChangeRequests serves the unified Eltern request list. The
// route is gated users:update OR users:absence like the pending-count badge;
// inside, the projection narrows an absence-only caller to the excused queue
// (#2232) and scopes per child through the owner queues, exactly as on the
// per-type routes.
func (rs *Resource) listAggregatedChangeRequests(w http.ResponseWriter, r *http.Request) {
	if rs.RequestReview == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("change request services not configured")))
		return
	}
	q, err := requestreview.ParseListQuery(r.URL.Query())
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	page, err := rs.RequestReview.ListRequests(r.Context(), q)
	if err != nil {
		renderError(w, r, rs.QueueError(err))
		return
	}
	common.Respond(w, r, http.StatusOK, page, "Change requests retrieved")
}
