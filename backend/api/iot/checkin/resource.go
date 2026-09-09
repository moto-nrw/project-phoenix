// Package checkin is the kiosk's HTTP resource for student card scans,
// pickup queries, device heartbeats and daily attendance. It calls the
// public device-scan contract and nothing else; the response envelope and
// the failure rendering come from the composition root through Runtime, so
// this package owns neither the HTTP platform nor the workflow (#2698).
package checkin

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

// Runtime carries the HTTP-platform behavior this resource must not own.
// Failure receives the classified status, code, error, details and client
// message positionally, so a test-support renderer can stand in for the
// shared envelope without naming this package's types.
type Runtime struct {
	Success func(http.ResponseWriter, *http.Request, int, any, string)
	Failure func(w http.ResponseWriter, r *http.Request, status int, code string, err error, details map[string]string, clientMessage string)
}

func (rt Runtime) valid() bool { return rt.Success != nil && rt.Failure != nil }

// Resource defines the check-in API resource for student RFID scans.
type Resource struct {
	scans   devicescan.Scanner
	runtime Runtime
	logger  *slog.Logger
}

// NewResource creates the check-in resource over the public scan contract.
func NewResource(scans devicescan.Scanner, runtime Runtime, logger *slog.Logger) *Resource {
	if scans == nil || !runtime.valid() {
		panic("checkin resource: the scanner and the runtime are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Resource{scans: scans, runtime: runtime, logger: logger}
}

// Router returns the router for the student RFID scan endpoints. All routes
// require device authentication (API key + staff PIN).
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	r.Post("/checkin", rs.deviceCheckin)
	r.Post("/pickup-query", rs.devicePickupQuery)
	r.Post("/ping", rs.devicePing)
	r.Get("/status", rs.deviceStatus)

	return r
}

// renderFailure maps a workflow outcome to the exact wire the kiosk expects:
// the structured capacity and duplicate-scan conflicts render their own
// bodies, classified refusals go through the shared envelope, and anything
// else is a generic server fault carrying its own text.
func renderFailure(w http.ResponseWriter, r *http.Request, runtime Runtime, logger *slog.Logger, err error) {
	if roomCapacity, ok := errors.AsType[*devicescan.RoomCapacityExceededError](err); ok {
		renderer := ErrorRoomCapacityExceededNoDetails()
		if roomCapacity.Details {
			renderer = ErrorRoomCapacityExceeded(roomCapacity.RoomID, roomCapacity.RoomName, roomCapacity.CurrentOccupancy, roomCapacity.MaxCapacity)
		}
		renderConflict(w, r, logger, renderer)
		return
	}
	if activityCapacity, ok := errors.AsType[*devicescan.ActivityCapacityExceededError](err); ok {
		renderer := ErrorActivityCapacityExceededNoDetails()
		if activityCapacity.Details {
			renderer = ErrorActivityCapacityExceeded(activityCapacity.ActivityID, activityCapacity.ActivityName, activityCapacity.CurrentOccupancy, activityCapacity.MaxCapacity)
		}
		renderConflict(w, r, logger, renderer)
		return
	}
	if duplicate, ok := errors.AsType[*devicescan.StudentAlreadyActiveError](err); ok {
		renderConflict(w, r, logger, ErrorStudentAlreadyActive(duplicate.StudentID, duplicate.ExistingVisitID, duplicate.EntryTime, duplicate.RoomID, duplicate.RoomName))
		return
	}

	failure, ok := devicescan.IsFailure(err)
	if !ok {
		runtime.Failure(w, r, http.StatusInternalServerError, "", err, nil, err.Error())
		return
	}
	switch failure.Kind {
	case devicescan.FailureUnauthorized:
		runtime.Failure(w, r, http.StatusUnauthorized, failure.Code, errors.New(failure.Message), nil, "")
	case devicescan.FailureInvalidRequest:
		runtime.Failure(w, r, http.StatusBadRequest, failure.Code, errors.New(failure.Message), nil, "")
	case devicescan.FailureNotFound:
		runtime.Failure(w, r, http.StatusNotFound, failure.Code, errors.New(failure.Message), nil, "")
	case devicescan.FailureConflict:
		runtime.Failure(w, r, http.StatusConflict, failure.Code, errors.New(failure.Message), nil, "")
	default:
		cause := failure.Cause
		if cause == nil {
			cause = errors.New(failure.Message)
		}
		runtime.Failure(w, r, http.StatusInternalServerError, "", cause, nil, failure.Message)
	}
}

func renderConflict(w http.ResponseWriter, r *http.Request, logger *slog.Logger, renderer render.Renderer) {
	if err := render.Render(w, r, renderer); err != nil {
		logger.ErrorContext(r.Context(), "error rendering error response", slog.String("error", err.Error()))
	}
}

func (rs *Resource) fail(w http.ResponseWriter, r *http.Request, err error) {
	renderFailure(w, r, rs.runtime, rs.logger, err)
}

// invalidRequest renders a request the resource could not bind.
func invalidRequest(w http.ResponseWriter, r *http.Request, runtime Runtime, err error) {
	runtime.Failure(w, r, http.StatusBadRequest, "", err, nil, "")
}
