package auth

import (
	"context"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// idAction returns a handler that parses one URL id parameter, invokes fn,
// and responds 204 No Content on success. Errors are rendered via renderErr.
func idAction(param, invalidMsg string, fn func(ctx context.Context, id int) error, renderErr func(error) render.Renderer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := common.ParseIntIDWithError(w, r, param, invalidMsg)
		if !ok {
			return
		}

		if err := fn(r.Context(), id); err != nil {
			common.RenderError(w, r, renderErr(err))
			return
		}

		common.RespondNoContent(w, r)
	}
}

// twoIDAction is idAction for handlers addressing a pair of resources
// (account+role, account+permission, role+permission).
func twoIDAction(param1, invalidMsg1, param2, invalidMsg2 string, fn func(ctx context.Context, id1, id2 int) error, renderErr func(error) render.Renderer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id1, ok := common.ParseIntIDWithError(w, r, param1, invalidMsg1)
		if !ok {
			return
		}

		id2, ok := common.ParseIntIDWithError(w, r, param2, invalidMsg2)
		if !ok {
			return
		}

		if err := fn(r.Context(), id1, id2); err != nil {
			common.RenderError(w, r, renderErr(err))
			return
		}

		common.RespondNoContent(w, r)
	}
}
