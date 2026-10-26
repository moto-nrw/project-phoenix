package compose

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
)

// Middleware is the native transport shape bound by Root.
type Middleware = func(http.Handler) http.Handler

// Administration mounts the native Device Fleet router built by Root.
type Administration func(chi.Router, Middleware)

// RouteResource is the router-shaped value Root constructs from an HTTP leaf.
type RouteResource interface{ Router() chi.Router }

// DataResource supplies the two existing data route mantles.
type DataResource interface {
	Router() chi.Router
	TeachersRouter() chi.Router
}

// ErrorReporter is the existing error-report handler bound by Root.
type ErrorReporter interface {
	Post(http.ResponseWriter, *http.Request)
}

// ErrorReports constructs the native reporter with the route's logger.
type ErrorReports func(*slog.Logger) ErrorReporter

// Feedback is the public provider contract used by the feedback leaf.
type Feedback interface {
	Available(context.Context) (bool, error)
	Submit(context.Context, feedbackModule.CreateEntry) (feedbackModule.Entry, error)
}

// EndSession carries the close instant and the existing byte-sized failure discriminator.
type EndSession func(context.Context, int64) (time.Time, uint8, error)

// RouterRuntime binds leaf constructors, guards, metrics and transactions from Root.
// AuthTiming runs right behind device authentication and TxTiming inside the
// tenant transaction; with Metrics in front of the authentication they time
// the sections of a kiosk request (#5004).
type RouterRuntime struct {
	Protected  func(chi.Router, func(chi.Router, Middleware))
	Required   func(string, Middleware) Middleware
	Metrics    Middleware
	TenantTx   Middleware
	AuthTiming Middleware
	TxTiming   Middleware
	Info       func(devicescan.ConfigurationQuery, devicescan.SchoolNameQuery) RouteResource
	Data       func(devicescan.Directory, devicescan.TagAssignments, devicescan.RoomAvailability) DataResource
	Feedback   func(devicescan.FeedbackStudents, Feedback, func(int, string), *slog.Logger) RouteResource
	Checkin    func(devicescan.DeviceScan, *slog.Logger) RouteResource
	OpenRooms  func(devicescan.OpenRoomBooking, *slog.Logger) RouteResource
	StaffClock func(devicescan.StaffClock) RouteResource
	Attendance func(devicescan.DeviceScan, *slog.Logger) RouteResource
	Session    func(devicescan.SessionLifecycle, EndSession) RouteResource
	RFID       func(devicescan.TagAssignments) RouteResource
}
