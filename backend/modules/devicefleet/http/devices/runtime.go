package devices

import (
	"errors"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
)

type Middleware = func(http.Handler) http.Handler

// Runtime supplies permission enforcement and the shared wire envelope.
type Runtime struct {
	ParseID    func(*http.Request, string) (int64, error)
	Permission func(string) Middleware
	Success    func(http.ResponseWriter, *http.Request, int, any, string)
	Failure    func(http.ResponseWriter, *http.Request, int, error, string)
	// ConflictOnField renders a 409 with a registered code that names the
	// field the conflict is about (#2517).
	ConflictOnField     func(w http.ResponseWriter, r *http.Request, err error, code, field string)
	ConstraintViolation func(error) bool
}

// codeDeviceIDTaken is the registered code of a device ID that another
// device already uses (error-registry.json). This package does not import
// api/common, so it names the code once here.
const codeDeviceIDTaken = "iot.device_id_taken"

func (rs *Resource) renderError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, devicefleet.ErrAdministrationDuplicateDeviceID) && rs.runtime.ConflictOnField != nil {
		rs.runtime.ConflictOnField(w, r, err, codeDeviceIDTaken, "device_id")
		return
	}
	status := http.StatusInternalServerError
	if failure, ok := err.(*devicefleet.AdministrationError); ok { //nolint:errorlint // preserve the historic outer-wrapper dispatch
		switch {
		case errors.Is(failure.Err, devicefleet.ErrAdministrationDeviceNotFound):
			status = http.StatusNotFound
		case errors.Is(failure.Err, devicefleet.ErrAdministrationInvalidDeviceData), errors.Is(failure.Err, devicefleet.ErrAdministrationInvalidStatus):
			status = http.StatusBadRequest
		case errors.Is(failure.Err, devicefleet.ErrAdministrationDuplicateDeviceID), errors.Is(failure.Err, devicefleet.ErrAdministrationDeviceOffline):
			status = http.StatusConflict
		case errors.Is(failure.Err, devicefleet.ErrAdministrationDeviceProtected):
			status = http.StatusForbidden
		}
	}
	rs.runtime.Failure(w, r, status, err, "")
}

// wireTime retains the administrative API's second-precision timestamps.
type wireTime time.Time

func (t wireTime) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Time(t).Format(time.RFC3339) + `"`), nil
}
