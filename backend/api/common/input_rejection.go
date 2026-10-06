package common

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
)

// InputRejection is a rejected input value with a registered code (ADR 0006,
// #2515). Packages that may not import api/common raise it as a typed error;
// ErrorField names the JSON key path of the rejected value, joined with dots
// for nested values ("children.0.date_of_birth"), or "" when the rejection
// concerns the request as a whole.
type InputRejection interface {
	error
	ErrorCode() string
	ErrorField() string
}

// asInputRejection finds the InputRejection in err's chain.
func asInputRejection(err error) (InputRejection, bool) {
	var rejection InputRejection
	if err == nil || !errors.As(err, &rejection) {
		return nil, false
	}
	return rejection, true
}

// HasInputRejection reports whether err's chain holds an InputRejection.
func HasInputRejection(err error) bool {
	_, ok := asInputRejection(err)
	return ok
}

// ErrorInputRejection answers err with 400. An InputRejection in its chain
// adds its code and marks its field; any other error stays a plain 400.
func ErrorInputRejection(err error) render.Renderer {
	resp := newErrResponse(http.StatusBadRequest, err)
	resp.Errors = validationFieldErrors(err)
	applyInputRejection(resp, err)
	return resp
}

// applyInputRejection copies code and field of an InputRejection in err's
// chain onto resp. An explicit code or field list already set wins.
func applyInputRejection(resp *ErrResponse, err error) {
	rejection, ok := asInputRejection(err)
	if !ok {
		return
	}
	if resp.Code == "" {
		resp.Code = rejection.ErrorCode()
	}
	if field := rejection.ErrorField(); field != "" && len(resp.Errors) == 0 {
		resp.Errors = []FieldError{{Field: field, Reason: rejection.Error()}}
	}
}
