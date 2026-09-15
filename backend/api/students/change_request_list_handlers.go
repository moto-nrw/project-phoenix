package students

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
)

// Aggregated Eltern request list (#2432): GET /students/change-requests
// serves all four guardian queues (Stammdaten, Betreuungszeiten, Angebote,
// Abwesenheiten) as ONE list — open or history — with server-side child
// name search, request-type filter and, in the history, status and decided-at
// range filters. The shared request-review projection (#2705) owns the
// merge, the paging and the per-type wire shapes; this route only parses the
// query and renders the page.

// AggregatedChangeRequestItem is one request of any of the four types.
type AggregatedChangeRequestItem = requestreview.Item

// AggregatedChangeRequestPage is the cursor envelope of the aggregated list.
type AggregatedChangeRequestPage = requestreview.Page

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
		renderError(w, r, parentRequestQueueErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, page, "Change requests retrieved")
}
