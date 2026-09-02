package students

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
)

func (rs *Resource) changeRequestAccess(w http.ResponseWriter, r *http.Request) {
	access, err := rs.reviewAccessLevel(r.Context())
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
