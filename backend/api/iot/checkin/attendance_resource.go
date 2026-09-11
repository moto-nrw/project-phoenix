package checkin

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

// AttendanceResource defines the daily attendance API resource.
type AttendanceResource struct {
	attendance devicescan.Attendance
	runtime    Runtime
	logger     *slog.Logger
}

// NewAttendanceResource creates the attendance resource over the public
// scan contract.
func NewAttendanceResource(attendance devicescan.Attendance, runtime Runtime, logger *slog.Logger) *AttendanceResource {
	if attendance == nil || !runtime.valid() {
		panic("attendance resource: the attendance contract and the runtime are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &AttendanceResource{attendance: attendance, runtime: runtime, logger: logger}
}

// Router returns the router for the attendance tracking endpoints. It is
// mounted under /iot/attendance/; all routes require device authentication
// (API key + staff PIN).
func (rs *AttendanceResource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	r.Get("/status/{rfid}", rs.getAttendanceStatus)
	r.Post("/toggle", rs.toggleAttendance)

	return r
}

func (rs *AttendanceResource) fail(w http.ResponseWriter, r *http.Request, err error) {
	if failure, ok := devicescan.IsFailure(err); ok && failure.Kind == devicescan.FailureUnauthorized {
		rs.logger.WarnContext(r.Context(), "device auth missing API key", slog.String("path", r.URL.Path))
	}
	renderFailure(w, r, rs.runtime, rs.logger, err)
}
