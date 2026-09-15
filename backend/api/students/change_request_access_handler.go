package students

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

// changeRequestAccess reports the caller's effective parent-request
// capability for the shared navigation. The retained review policy answers
// it; an unwired policy is a configuration error.
func (rs *Resource) changeRequestAccess(w http.ResponseWriter, r *http.Request) {
	if rs.RequestReviewAccess == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("parent request review policy is not configured")))
		return
	}
	access, err := rs.RequestReviewAccess.AccessLevel(r.Context(), jwt.PermissionsFromCtx(r.Context()))
	if err != nil {
		renderError(w, r, parentRequestQueueErrorRenderer(err))
		return
	}
	if access == "" {
		renderError(w, r, common.ErrorInternalServer(errors.New("parent request review policy is not configured")))
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{
		"review_access": access,
	}, "Change request access retrieved")
}
