//go:build testhooks

package api

import (
	"errors"
	"net/http"

	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
)

// sentrySessionTestAdapters supplies the shared HTTP runtime at the root's
// composition boundary. Session observation tests need only these two handlers.
func sentrySessionTestAdapters() (func(http.Handler) http.Handler, http.HandlerFunc) {
	return ServerErrorReporting, func(w http.ResponseWriter, r *http.Request) {
		errResp := apiCommon.ErrorInternalServer(errors.New("load student: connection reset")).(*apiCommon.ErrResponse)
		errResp.Code = "STUDENT_LOAD_FAILED"
		apiCommon.RenderError(w, r, errResp)
	}
}
