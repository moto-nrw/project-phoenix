package common

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/render"
)

// Handler builders — issue #575 B2.
//
// These generalize the parse → one service call → render handler shape so a
// route registration IS the handler (no named pass-through function). They
// were promoted from two near-identical package-local copies
// (api/auth/handler_builders.go, api/operator/handler_builders.go).
//
// Cutoff policy (backend-conventions.md): use a builder ONLY when the handler
// is purely parse/bind → one service call → respond. A handler that branches
// per request, calls more than one service, or shapes its response keeps a
// named function — do not grow option-struct variants here to force-fit those.

// IDType covers the two ID representations used by service signatures.
// Services keep whichever width they already have; the builder adapts.
type IDType interface{ ~int | ~int64 }

// parseIDOrRender parses one URL id parameter, rendering a 400 with
// invalidMsg on failure (same contract as ParseInt64IDWithError).
func parseIDOrRender[I IDType](w http.ResponseWriter, r *http.Request, param, invalidMsg string) (I, bool) {
	id, err := ParseIDParam(r, param)
	if err != nil {
		RenderError(w, r, ErrorInvalidRequest(errors.New(invalidMsg)))
		return 0, false
	}
	return I(id), true
}

// IDAction returns a handler that parses one URL id parameter, invokes fn,
// and responds 204 No Content on success. Errors render via renderErr.
func IDAction[I IDType](param, invalidMsg string, fn func(ctx context.Context, id I) error, renderErr func(error) render.Renderer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseIDOrRender[I](w, r, param, invalidMsg)
		if !ok {
			return
		}

		if err := fn(r.Context(), id); err != nil {
			RenderError(w, r, renderErr(err))
			return
		}

		RespondNoContent(w, r)
	}
}

// TwoIDAction is IDAction for handlers addressing a pair of resources
// (account+role, account+permission, role+permission).
func TwoIDAction[I IDType](param1, invalidMsg1, param2, invalidMsg2 string, fn func(ctx context.Context, id1, id2 I) error, renderErr func(error) render.Renderer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id1, ok := parseIDOrRender[I](w, r, param1, invalidMsg1)
		if !ok {
			return
		}

		id2, ok := parseIDOrRender[I](w, r, param2, invalidMsg2)
		if !ok {
			return
		}

		if err := fn(r.Context(), id1, id2); err != nil {
			RenderError(w, r, renderErr(err))
			return
		}

		RespondNoContent(w, r)
	}
}

// IDFetch returns a handler that parses one URL id parameter, fetches a
// payload via fn, and responds 200 with the payload wrapped in the standard
// success envelope.
func IDFetch[I IDType, T any](param, invalidMsg string, fn func(ctx context.Context, id I) (T, error), renderErr func(error) render.Renderer, successMsg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseIDOrRender[I](w, r, param, invalidMsg)
		if !ok {
			return
		}

		payload, err := fn(r.Context(), id)
		if err != nil {
			RenderError(w, r, renderErr(err))
			return
		}

		Respond(w, r, http.StatusOK, payload, successMsg)
	}
}

// Fetch returns a handler that fetches one payload and responds 200 with the
// payload wrapped in the standard success envelope.
func Fetch[T any](fn func(ctx context.Context) (T, error), renderErr func(error) render.Renderer, successMsg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payload, err := fn(r.Context())
		if err != nil {
			RenderError(w, r, renderErr(err))
			return
		}

		Respond(w, r, http.StatusOK, payload, successMsg)
	}
}

// BindAction returns a handler that binds the request body into a fresh
// Req (render.Bind runs its Bind() validation), invokes fn, and responds
// with the given status and payload. Bind failures render a 400.
func BindAction[Req render.Binder, T any](newReq func() Req, status int, fn func(ctx context.Context, req Req) (T, error), renderErr func(error) render.Renderer, successMsg string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req := newReq()
		if err := render.Bind(r, req); err != nil {
			RenderError(w, r, ErrorInvalidRequest(err))
			return
		}

		payload, err := fn(r.Context(), req)
		if err != nil {
			RenderError(w, r, renderErr(err))
			return
		}

		Respond(w, r, status, payload, successMsg)
	}
}
