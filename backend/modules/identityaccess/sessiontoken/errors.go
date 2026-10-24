package sessiontoken

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
)

// The list of jwt token errors presented to the end user.
var (
	ErrTokenUnauthorized  = errors.New("token unauthorized")
	ErrTokenExpired       = errors.New("token expired")
	ErrInvalidAccessToken = errors.New("invalid access token")
)

// ErrResponse is the shared error envelope (ADR 0006) of a refused token;
// ProblemResponseMiddleware adds type, title and instance. This package may
// not import api/common, so it names the registered class code itself.
type ErrResponse struct {
	Err            error  `json:"-"` // low-level runtime error
	HTTPStatusCode int    `json:"-"` // http response status code
	Status         int    `json:"status"`
	Detail         string `json:"detail"`
	Code           string `json:"code"`
}

const codeGeneralPermission = "general.permission"

// Render sets the response status.
func (e *ErrResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	e.Status = e.HTTPStatusCode
	render.Status(r, e.HTTPStatusCode)
	return nil
}

// ErrUnauthorized renders status 401 Unauthorized with custom error message.
func ErrUnauthorized(err error) render.Renderer {
	return &ErrResponse{Err: err, HTTPStatusCode: http.StatusUnauthorized, Detail: err.Error(), Code: codeGeneralPermission}
}
