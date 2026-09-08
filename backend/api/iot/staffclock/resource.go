// Package staffclock is the kiosk's HTTP resource for NFC staff time
// tracking. It calls the public device-scan contract and nothing else; the
// response envelope and the failure rendering come from the composition root
// through Runtime, so this package owns neither the HTTP platform nor the
// workflow.
package staffclock

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

// Failure is a classified kiosk failure. Status and Code are the wire
// contract PyrePortal maps to German UI text; Err carries the message and the
// logged cause, Details the code-specific payload of a conflict.
type Failure struct {
	Status  int
	Code    string
	Err     error
	Details map[string]string
	// ClientMessage replaces the error wording on the wire when set; the
	// cause then stays in the log only.
	ClientMessage string
}

// Runtime carries the HTTP-platform behavior this resource must not own.
// Failure receives the classified status, code, error, details and client
// message positionally, so a test-support renderer can stand in for the
// shared envelope without naming this package's types.
type Runtime struct {
	Success func(http.ResponseWriter, *http.Request, int, any, string)
	Failure func(w http.ResponseWriter, r *http.Request, status int, code string, err error, details map[string]string, clientMessage string)
}

func (rs *Resource) fail(w http.ResponseWriter, r *http.Request, failure Failure) {
	rs.runtime.Failure(w, r, failure.Status, failure.Code, failure.Err, failure.Details, failure.ClientMessage)
}

type Resource struct {
	service devicescan.StaffClock
	runtime Runtime
}

func NewResource(service devicescan.StaffClock, runtime Runtime) *Resource {
	if service == nil || runtime.Success == nil || runtime.Failure == nil {
		panic("staff clock resource: all dependencies are required")
	}
	return &Resource{service: service, runtime: runtime}
}

func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Post("/staff-clock", rs.execute)
	r.Post("/staff-clock/state", rs.getState)
	return r
}

func (rs *Resource) getState(w http.ResponseWriter, r *http.Request) {
	req := new(StateRequest)
	if err := render.Bind(r, req); err != nil {
		rs.fail(w, r, Failure{Status: http.StatusBadRequest, Code: codeInvalidRequest, Err: err})
		return
	}
	state, err := rs.service.StaffClockState(r.Context(), req.RFIDTag)
	if err != nil {
		rs.fail(w, r, classifyError(err))
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, state, "Staff clock state retrieved")
}

func (rs *Resource) execute(w http.ResponseWriter, r *http.Request) {
	req := new(CommandRequest)
	if err := render.Bind(r, req); err != nil {
		rs.fail(w, r, Failure{Status: http.StatusBadRequest, Code: codeInvalidRequest, Err: err})
		return
	}
	state, err := rs.service.ExecuteStaffClock(r.Context(), devicescan.StaffClockCommand{
		RFIDTag:                req.RFIDTag,
		Action:                 req.Action,
		Status:                 req.Status,
		Reason:                 req.Reason,
		PlannedDurationMinutes: req.PlannedDurationMinutes,
	})
	if err != nil {
		rs.fail(w, r, classifyError(err))
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, state, "Staff clock action completed")
}
