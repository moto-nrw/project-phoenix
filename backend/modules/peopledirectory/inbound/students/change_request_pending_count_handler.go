package students

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// pendingChangeRequestCount returns the combined number of pending parent
// change requests for the current tenant. It backs the Änderungsanfragen
// sidebar badge, which reuses the same UnreadBadge as the Nachrichten badge:
// a single number the deciding staffer clears by working the queue. The
// shared request-review projection (#2705) sums exactly the queues the
// caller can open (#2232): the excused queue always, the three
// users:update-gated queues only for callers who hold that right.
func (rs *Resource) pendingChangeRequestCount(w http.ResponseWriter, r *http.Request) {
	if rs.RequestReview == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("change request services not configured")))
		return
	}
	pending, err := rs.RequestReview.PendingCount(r.Context())
	if err != nil {
		renderError(w, r, parentRequestQueueErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]int{
		"pending_count": pending,
	}, "Pending change request count retrieved")
}
