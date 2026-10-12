package enrollmenthttp

import (
	"context"
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

// The read state of the enrollment queue is per account (#3778): each
// handler acts only on the caller's own read rows of the caller's school.

// unreadAdminRequestCount serves GET /admin/requests/unread-count: the
// badge on the Anmeldungen section. It is not part of the Anfragen badge.
func (rs *Resource) unreadAdminRequestCount(w http.ResponseWriter, r *http.Request) {
	if rs.DecisionService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("decision service not configured")))
		return
	}
	accountID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	var unread int
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		count, countErr := rs.DecisionService.CountUnreadRequests(ctx, accountID)
		unread = count
		return countErr
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]int{
		"unread_count": unread,
	}, "Unread enrollment count retrieved")
}

// markAdminRequestRead serves PUT /admin/requests/{id}/read.
func (rs *Resource) markAdminRequestRead(w http.ResponseWriter, r *http.Request) {
	rs.writeReadState(w, r, func(ctx context.Context, accountID, requestID int64) error {
		return rs.DecisionService.MarkRequestsRead(ctx, accountID, []int64{requestID})
	})
}

// markAdminRequestUnread serves DELETE /admin/requests/{id}/read.
func (rs *Resource) markAdminRequestUnread(w http.ResponseWriter, r *http.Request) {
	rs.writeReadState(w, r, rs.DecisionService.MarkRequestUnread)
}

func (rs *Resource) writeReadState(w http.ResponseWriter, r *http.Request, write func(ctx context.Context, accountID, requestID int64) error) {
	if rs.DecisionService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("decision service not configured")))
		return
	}
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}
	accountID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	if err := rs.runInTenantTx(r, func(ctx context.Context) error {
		return write(ctx, accountID, id)
	}); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// markAllAdminRequestsRead serves POST /admin/requests/mark-all-read.
func (rs *Resource) markAllAdminRequestsRead(w http.ResponseWriter, r *http.Request) {
	if rs.DecisionService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("decision service not configured")))
		return
	}
	accountID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	if err := rs.runInTenantTx(r, func(ctx context.Context) error {
		return rs.DecisionService.MarkAllRequestsRead(ctx, accountID)
	}); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
