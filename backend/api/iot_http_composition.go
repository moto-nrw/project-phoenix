package api

import (
	"context"
	"log/slog"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	iotCore "github.com/moto-nrw/project-phoenix/api/iot"
	iotCompose "github.com/moto-nrw/project-phoenix/api/iot/compose"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/deviceauth"
	devicesAPI "github.com/moto-nrw/project-phoenix/modules/devicefleet/http/devices"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	checkinAPI "github.com/moto-nrw/project-phoenix/modules/devicescan/http/checkin"
	dataAPI "github.com/moto-nrw/project-phoenix/modules/devicescan/http/data"
	sessionsAPI "github.com/moto-nrw/project-phoenix/modules/devicescan/http/sessions"
	staffclockAPI "github.com/moto-nrw/project-phoenix/modules/devicescan/http/staffclock"
)

// NewIoTHTTPRuntime binds the native HTTP leaf constructors without a Root service graph.
func NewIoTHTTPRuntime() iotCompose.RouterRuntime {
	return iotCompose.RouterRuntime{
		Protected: func(r chi.Router, register func(chi.Router, iotCompose.Middleware)) {
			ProtectedTenantRoutes(r, func(r chi.Router, withTx common.Middleware) { register(r, withTx) })
		},
		Required: deviceauth.Required, Metrics: iotMetricsMiddleware, TenantTx: common.TenantTxMiddleware,
		AuthTiming: iotAuthTiming, TxTiming: iotTxTiming,
		Info: func(cfg devicescan.ConfigurationQuery, school devicescan.SchoolNameQuery) iotCompose.RouteResource {
			return iotCore.NewResource(cfg, school, iotInfoRuntime())
		},
		Data: func(directory devicescan.Directory, tags devicescan.TagAssignments, rooms devicescan.RoomAvailability) iotCompose.DataResource {
			return dataAPI.NewResource(directory, tags, rooms, iotDataRuntime())
		},
		Feedback: func(students devicescan.FeedbackStudents, service iotCompose.Feedback, observe func(int, string), logger *slog.Logger) iotCompose.RouteResource {
			return dataAPI.NewFeedbackResource(students, service, observe, iotDataRuntime(), logger)
		},
		Checkin: func(scan devicescan.DeviceScan, logger *slog.Logger) iotCompose.RouteResource {
			return checkinAPI.NewResource(scan, iotCheckinRuntime(), logger)
		},
		OpenRooms: func(booking devicescan.OpenRoomBooking, logger *slog.Logger) iotCompose.RouteResource {
			return checkinAPI.NewOpenRoomResource(booking, iotCheckinRuntime(), logger)
		},
		StaffClock: func(clock devicescan.StaffClock) iotCompose.RouteResource {
			return staffclockAPI.NewResource(clock, iotStaffClockRuntime())
		},
		Attendance: func(scan devicescan.DeviceScan, logger *slog.Logger) iotCompose.RouteResource {
			return checkinAPI.NewAttendanceResource(scan, iotCheckinRuntime(), logger)
		},
		Session: newIoTSessionResource,
		RFID: func(tags devicescan.TagAssignments) iotCompose.RouteResource {
			return dataAPI.NewRFIDResource(tags, iotDataRuntime())
		},
	}
}
func newIoTSessionResource(lifecycle devicescan.SessionLifecycle, end iotCompose.EndSession) iotCompose.RouteResource {
	var native sessionsAPI.EndSession
	if end != nil {
		native = func(ctx context.Context, id int64) (time.Time, sessionsAPI.EndFailure, error) {
			when, kind, err := end(ctx, id)
			return when, sessionsAPI.EndFailure(kind), err
		}
	}
	return sessionsAPI.NewResource(lifecycle, native, iotSessionRuntime())
}
func bindIoTHostSessionEnd(end sessionsAPI.EndSession) iotCompose.EndSession {
	if end == nil {
		return nil
	}
	return func(ctx context.Context, id int64) (time.Time, uint8, error) {
		when, kind, err := end(ctx, id)
		return when, uint8(kind), err
	}
}

// NewIoTAdministration mounts the same native Device Fleet resource in its JWT/tenant group.
func NewIoTAdministration(administration devicefleet.Administration) iotCompose.Administration {
	return func(r chi.Router, withTx iotCompose.Middleware) {
		devicesResource := devicesAPI.NewResource(administration, iotDevicesRuntime())
		r.With(withTx).Mount("/", devicesResource.Router())
	}
}

// NewIoTErrorReporter builds the same native report handler with its route logger.
func NewIoTErrorReporter(relay iotCore.ErrorReportRelay) iotCompose.ErrorReports {
	return func(logger *slog.Logger) iotCompose.ErrorReporter {
		return iotCore.NewErrorReports(relay, iotErrorReportsRuntime(), logger)
	}
}
