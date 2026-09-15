package testutil

import (
	"net/http"
)

// The request and response shapes this package's helpers take and return.
// Aliased so a route test that already drives its assertions through this
// boundary names the whole vocabulary here instead of half of it.
type (
	Request        = http.Request
	ResponseWriter = http.ResponseWriter
	HandlerFunc    = http.HandlerFunc
)

// Request methods the route tests exercise.
const (
	MethodGet    = http.MethodGet
	MethodPost   = http.MethodPost
	MethodDelete = http.MethodDelete
)

// Response statuses the route assertions expect. These are the values passed
// to AssertSuccessResponse and friends, so they belong to that API's
// vocabulary rather than being reached for past it.
const (
	StatusOK                  = http.StatusOK
	StatusCreated             = http.StatusCreated
	StatusNoContent           = http.StatusNoContent
	StatusBadRequest          = http.StatusBadRequest
	StatusUnauthorized        = http.StatusUnauthorized
	StatusForbidden           = http.StatusForbidden
	StatusNotFound            = http.StatusNotFound
	StatusConflict            = http.StatusConflict
	StatusInternalServerError = http.StatusInternalServerError
)
