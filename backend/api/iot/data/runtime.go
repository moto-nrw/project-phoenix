package data

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

// Runtime binds device principals and the shared response envelope at the
// composition root. Handlers classify errors without importing that runtime.
type Runtime struct {
	ParseID func(*http.Request, string) (int64, error)
	Device  func(context.Context) (id int64, deviceID string, found bool)
	Success func(http.ResponseWriter, *http.Request, int, any, string)
	Failure func(http.ResponseWriter, *http.Request, int, error, string)
}

func (rt Runtime) valid() bool {
	return rt.ParseID != nil && rt.Device != nil && rt.Success != nil && rt.Failure != nil
}

type deviceIdentity struct {
	ID       int64
	DeviceID string
}

func (rt Runtime) requireDevice(w http.ResponseWriter, r *http.Request) (deviceIdentity, bool) {
	id, deviceID, found := rt.Device(r.Context())
	if !found {
		slog.WarnContext(r.Context(), "device auth missing API key", slog.String("path", r.URL.Path))
		rt.Failure(w, r, http.StatusUnauthorized, errors.New(devicescan.MessageDeviceAPIKeyRequired), "")
		return deviceIdentity{}, false
	}
	return deviceIdentity{ID: id, DeviceID: deviceID}, true
}
