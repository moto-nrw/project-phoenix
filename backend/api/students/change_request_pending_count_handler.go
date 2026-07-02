package students

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// pendingChangeRequestCount returns the combined number of pending parent
// change requests (master-data + care-schedule) for the current tenant. It
// backs the Änderungsanfragen sidebar badge, which reuses the same UnreadBadge
// as the Nachrichten badge: a single number the deciding admin clears by
// working the queue. Both sub-queues are already admin-gated (UsersManage) and
// deliberately short (see the change-requests page), so summing the two
// existing ListPending results is cheaper than it looks and keeps the count on
// the exact same query path the review lists render — no second source that
// could drift.
func (rs *Resource) pendingChangeRequestCount(w http.ResponseWriter, r *http.Request) {
	if rs.MasterDataReviewService == nil || rs.CareRequestService == nil {
		renderError(w, r, ErrorInternalServer(errors.New("change request services not configured")))
		return
	}
	ctx := r.Context()

	masterData, err := rs.MasterDataReviewService.ListPending(ctx)
	if err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}
	care, err := rs.CareRequestService.ListPending(ctx)
	if err != nil {
		renderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, map[string]int{
		"pending_count": len(masterData) + len(care),
	}, "Pending change request count retrieved")
}
