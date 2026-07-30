package students

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// pendingChangeRequestCount returns the combined number of pending parent
// change requests (master-data + care-schedule + excused absences + offering
// changes) for the current tenant. Every queue rendered on the
// Änderungsanfragen page must be summed here: a request missing from the count
// is a request nobody looks at until someone happens to open the page. It
// backs the Änderungsanfragen sidebar badge, which reuses the same UnreadBadge
// as the Nachrichten badge: a single number the deciding staffer clears by
// working the queue. Both sub-queues are users:update-gated and per-child
// write-scoped in the service (admin or the child's group supervisor), so
// summing the two ListPending results yields a count scoped to exactly the
// requests this caller can act on — and keeps the count on the same query path
// the review lists render, with no second source that could drift.
func (rs *Resource) pendingChangeRequestCount(w http.ResponseWriter, r *http.Request) {
	if rs.MasterDataReviewService == nil || rs.CareRequestService == nil || rs.ExcusedRequestService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("change request services not configured")))
		return
	}
	ctx := r.Context()

	masterData, err := rs.MasterDataReviewService.ListPending(ctx)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	care, err := rs.CareRequestService.ListPending(ctx)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	excused, err := rs.ExcusedRequestService.ListPending(ctx)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	// Offering changes (#1665) are counted when the service is wired; a nil
	// service (older wiring in tests) contributes zero rather than a 500.
	offeringChanges := 0
	if rs.OfferingChangeService != nil {
		offerings, offeringErr := rs.OfferingChangeService.ListPending(ctx)
		if offeringErr != nil {
			renderError(w, r, common.ErrorInternalServer(offeringErr))
			return
		}
		offeringChanges = len(offerings)
	}

	common.Respond(w, r, http.StatusOK, map[string]int{
		"pending_count": len(masterData) + len(care) + len(excused) + offeringChanges,
	}, "Pending change request count retrieved")
}
