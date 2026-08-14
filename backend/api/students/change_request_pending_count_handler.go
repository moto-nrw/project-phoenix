package students

import (
	"context"
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

// pendingChangeRequestCount returns the combined number of pending parent
// change requests for the current tenant. Every queue the caller can actually
// open on the Änderungsanfragen page must be summed here: a request missing
// from the count is a request nobody looks at until someone happens to open the
// page. It backs the Änderungsanfragen sidebar badge, which reuses the same
// UnreadBadge as the Nachrichten badge: a single number the deciding staffer
// clears by working the queue.
//
// The count follows the queue gates exactly, because a badge for a queue that
// answers 403 is worse than no badge (#2232):
//   - Excused absences are gated users:update OR users:absence and scoped per
//     child by the absence gate, so they always count here — the route carries
//     the same pair.
//   - Stammdaten, Betreuungszeiten and Angebotswechsel are users:update-gated.
//     Their services scope per child (admin or the child's group supervisor)
//     but never re-check the permission, so an absence-only supervisor would
//     otherwise get a badge for three queues the route denies them.
//
// Offering changes use their dedicated count to avoid building every review
// diff just to render the badge.
func (rs *Resource) pendingChangeRequestCount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if rs.ExcusedRequestService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("change request services not configured")))
		return
	}

	excused, err := rs.ExcusedRequestService.ListPending(ctx)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	pending := len(excused)

	if authorize.HasPermission(permissions.UsersUpdate, jwt.PermissionsFromCtx(ctx)) {
		writeQueues, writeErr := rs.pendingWriteQueueCount(ctx)
		if writeErr != nil {
			renderError(w, r, common.ErrorInternalServer(writeErr))
			return
		}
		pending += writeQueues
	}

	common.Respond(w, r, http.StatusOK, map[string]int{
		"pending_count": pending,
	}, "Pending change request count retrieved")
}

// pendingWriteQueueCount sums the three users:update-gated queues
// (Stammdaten, Betreuungszeiten, Angebotswechsel). Only called for callers who
// hold that permission.
func (rs *Resource) pendingWriteQueueCount(ctx context.Context) (int, error) {
	if rs.MasterDataReviewService == nil || rs.CareRequestService == nil {
		return 0, errors.New("change request services not configured")
	}

	masterData, err := rs.MasterDataReviewService.ListPending(ctx)
	if err != nil {
		return 0, err
	}
	care, err := rs.CareRequestService.ListPending(ctx)
	if err != nil {
		return 0, err
	}
	pending := len(masterData) + len(care)

	// Offering changes (#1665) are counted when the service is wired; a nil
	// service (older wiring in tests) contributes zero rather than a 500.
	if rs.OfferingChangeService != nil {
		offeringCount, offeringErr := rs.OfferingChangeService.PendingCount(ctx)
		if offeringErr != nil {
			return 0, offeringErr
		}
		pending += offeringCount
	}

	return pending, nil
}
