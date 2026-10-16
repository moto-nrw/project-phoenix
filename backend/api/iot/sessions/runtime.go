package sessions

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

// EndFailure classifies a failed session close without exposing its workflow.
type EndFailure uint8

const (
	EndFailed EndFailure = iota
	EndNotFound
	EndAlreadyEnded
)

// EndSession closes the selected session and returns its close instant and raw error.
// The root binds the workflow and translates its failure classification.
type EndSession func(context.Context, int64) (time.Time, EndFailure, error)

// Runtime binds authentication, responses and rollback at the composition root.
type Runtime struct {
	ParseID       func(*http.Request, string) (int64, error)
	Authenticated func(context.Context) bool
	Success       func(http.ResponseWriter, *http.Request, int, any, string)
	Failure       func(http.ResponseWriter, *http.Request, int, error, string)
	// Conflict answers a session conflict with 409 in the shared error
	// envelope, the conflict as details.
	Conflict     func(http.ResponseWriter, *http.Request, string, devicescan.ConflictInfoResponse)
	MarkRollback func(context.Context)
}

func (rs *Resource) requireDevice(w http.ResponseWriter, r *http.Request) bool {
	if rs.runtime.Authenticated(r.Context()) {
		return true
	}
	rs.runtime.Failure(w, r, http.StatusUnauthorized, errors.New(devicescan.MessageDeviceAPIKeyRequired), "")
	return false
}

func (rs *Resource) renderError(w http.ResponseWriter, r *http.Request, err error) {
	status, message := http.StatusInternalServerError, ""
	if failure, ok := devicescan.IsFailure(err); ok {
		message = failure.Message
		switch failure.Kind {
		case devicescan.FailureUnauthorized:
			status = http.StatusUnauthorized
		case devicescan.FailureInvalidRequest:
			status = http.StatusBadRequest
		case devicescan.FailureNotFound:
			status = http.StatusNotFound
		case devicescan.FailureConflict:
			status = http.StatusConflict
		}
	}
	rs.runtime.Failure(w, r, status, err, message)
}
