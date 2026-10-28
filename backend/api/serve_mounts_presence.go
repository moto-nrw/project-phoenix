package api

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"

	"github.com/go-chi/chi/v5"
	iotAPI "github.com/moto-nrw/project-phoenix/api/iot/compose"
	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	eventContract "github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"
	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/deviceauth"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
	"github.com/moto-nrw/project-phoenix/modules/grouplive"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	settingsFacts "github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	presenceAPI "github.com/moto-nrw/project-phoenix/modules/studentpresence/inbound/presence"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	supervisionDashboardHTTP "github.com/moto-nrw/project-phoenix/modules/supervisiondashboard/http"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/realtime"
	scanBindings "github.com/moto-nrw/project-phoenix/roots/devicescanbinding"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/workflows/openroommove"
	openRoomMoveHTTP "github.com/moto-nrw/project-phoenix/workflows/openroommove/http"
	"github.com/moto-nrw/project-phoenix/workflows/sessionend"
)

func newDeviceAuthentication(iot devicefleetCompose.IoTService, organizations organizationtenancy.Query, settings settingsCompose.SettingsService, logger *slog.Logger) *deviceauth.Authenticators {
	return deviceauth.New(deviceauth.Dependencies{
		Logger:       logger,
		Devices:      iot.Fleet(),
		Schools:      deviceSchoolDirectory{schools: organizations},
		SchoolErrors: devicefleetCompose.NewSchoolReadErrors(),
		Settings:     settings,
	})
}

// mountPresenceRoutes serves the attendance routes and the kiosk API. Both
// move children through one open-room move over the Student Presence owner.
func mountPresenceRoutes(
	tenant chi.Router, db transactionruntime.Database, logger *slog.Logger, errorReportDSN string,
	deviceAuth *deviceauth.Authenticators, presence studentpresence.Owner, active studentpresence.Presence,
	facilities facilitiesModule.Capability, timetableOwner timetableModule.Capability, timetableRepos timetableCompose.Repositories,
	days owners.TimetableDays, persons peopledirectory.Capability, users peoplePorts.PersonDirectory, caller *owners.CallerRows,
	settings settingsCompose.SettingsService, hub *realtime.Hub, emitter eventContract.Emitter,
	groupLive grouplive.Query, dashboard supervisiondashboard.Query, yard FacilitiesYard, iot devicefleetCompose.IoTService,
	activities activityContract.ActivityService, education schoolstructure.GroupManagement, pickups carePlanModule.PickupScheduleService,
	timeTracking *workforceCompose.TimeTracking, rooms FacilitiesRooms, feedback feedbackModule.Capability,
	organizations organizationtenancy.Query, invitations identityaccess.SchoolInvitations,
) error {
	openRoomMove, err := newOpenRoomMove(facilities, timetableOwner, active, logger)
	if err != nil {
		return err
	}
	operations, err := presenceservice.NewPresenceOperations(active, groupLive, logger.With("service", "presence-operations"))
	if err != nil {
		return err
	}
	activeResource := presenceAPI.NewResource(operations, activePeople{source: NewAttendanceRoutePeople(users)}, teacherGroupIDs(education), activeStaffAccess{source: peopleCompose.NewAttendanceRouteStaff(caller)}, settings, ProtectedTenantRoutesWith(supervisionDashboardHTTP.NewResource(dashboard).Register), logger.With("handler", "active"), presence, activeRequestRuntime(), activeAuthorization(), settingsFacts.NewDisplaySettings(settings))
	sessionEnd, err := newSessionEnd(presence, timetableOwner, days.Bridge, persons, facilities, hub, emitter, logger)
	if err != nil {
		return err
	}
	iotResource, err := newIoTResource(db, logger, errorReportDSN, deviceAuth, presence, openRoomMove, sessionEnd, iot, facilities, active, users,
		activities, timetableOwner, timetableRepos, education, pickups, settings, timeTracking, rooms, feedback, organizations, hub, invitations)
	if err != nil {
		return err
	}
	move := &openRoomMoveHTTP.Resource{
		OpenRoomMove: openRoomMove, Authorize: activeResource.BulkStudentMoveAuthorization,
		MarkRollback: activeRequestRuntime().MarkRollback, RenderPresenceError: presenceAPI.ErrorRenderer,
		WebAttendance: settingsFacts.NewDisplaySettings(settings).WebAttendanceEnabledReader(),
	}
	yardResource := supervisionDashboardHTTP.NewSchulhofResource(newSupervisionYard(yard.Schulhof), func(ctx context.Context) (int64, bool, error) {
		staff, err := activeResource.UserContextService.GetCurrentStaff(ctx)
		if staff == nil {
			return 0, false, err
		}
		return staff.ID, true, err
	})
	tenant.Mount("/active", activeResource.RouterWithOwnerRoutes(move.Register, yardResource.Register))
	tenant.Mount("/iot", iotResource.Router())
	return nil
}

func teacherGroupIDs(education schoolstructure.GroupManagement) func(context.Context, int64) ([]int64, error) {
	return func(ctx context.Context, teacherID int64) ([]int64, error) {
		groups, err := education.GetTeacherGroups(ctx, teacherID)
		if err != nil {
			return nil, err
		}
		ids := make([]int64, 0, len(groups))
		for _, group := range groups {
			if group != nil {
				ids = append(ids, group.ID)
			}
		}
		return ids, nil
	}
}

// newIoTResource composes the kiosk API. The device-scan workflow runs every
// kiosk scan through one orchestrator over the public Device Fleet, Student
// Presence, Facilities and Timetable & Activities capabilities; the retained
// services behind its ports are compatibility bindings (#2698).
func newIoTResource(
	db transactionruntime.Database, logger *slog.Logger, errorReportDSN string, deviceAuth *deviceauth.Authenticators,
	presence studentpresence.Owner, openRoomMove openroommove.Command, sessionEnd sessionend.Command,
	iot devicefleetCompose.IoTService, facilities facilitiesModule.Capability, active studentpresence.Presence,
	users peoplePorts.PersonDirectory, activities activityContract.ActivityService, timetableOwner timetableModule.Capability,
	timetableRepos timetableCompose.Repositories, education schoolstructure.GroupManagement, pickups carePlanModule.PickupScheduleService,
	settings settingsCompose.SettingsService, timeTracking *workforceCompose.TimeTracking, rooms FacilitiesRooms,
	feedback feedbackModule.Capability, organizations organizationtenancy.Query, hub *realtime.Hub,
	invitations identityaccess.SchoolInvitations,
) (*iotAPI.Resource, error) {
	errorReports, err := NewIoTErrorReportRelay(errorReportDSN)
	if err != nil {
		return nil, err
	}
	deviceScan := scanBindings.New(devicescanCompose.Dependencies{Principal: DeviceScanPrincipal,
		Fleet:      iot.Fleet(),
		Presence:   presence,
		Rooms:      facilities,
		Active:     active,
		Users:      NewDeviceScanPeople(users),
		Activities: DeviceActivityCatalog(activities),
		Rosters:    timetableOwner,
		Education:  education,
		Pickups:    pickups,
		// A destination chosen at the kiosk runs the phone's open-room
		// move (#3067).
		OpenRooms: newDeviceOpenRoomMover(openRoomMove),
		Settings:  newDeviceScanSettings(settings),
		Logger:    logger.With("service", "device-scan"),
	})
	return iotAPI.NewResource(iotAPI.ServiceDependencies{
		Administration:   NewIoTAdministration(devicefleetCompose.NewAdministration(iot.Fleet())),
		Runtime:          NewIoTHTTPRuntime(),
		DeviceScan:       deviceScan,
		OpenRooms:        deviceScan,
		StaffClock:       newStaffClockService(users, owners.NewPeopleIdentityAccess(db, observability.ObserveIdentityAccessOperation), timeTracking.TimeClock),
		Configuration:    devicescanCompose.NewConfiguration(DeviceConfiguration{Settings: settings}, DeviceScanPrincipal),
		Rooms:            devicescanCompose.NewRoomAvailability(rooms.RoomService, DeviceScanPrincipal),
		Directory:        devicescanCompose.NewDirectory(NewDeviceScanDirectoryPeople(users, invitations), DeviceActivityCatalog(activities), logger, DeviceScanPrincipal),
		TagAssignments:   devicescanCompose.NewTagAssignments(NewDeviceScanPeople(users), logger, DeviceScanPrincipal),
		FeedbackStudents: devicescanCompose.NewFeedbackStudents(NewDeviceScanPeople(users)),
		FeedbackService:  feedback,
		FeedbackResponseObserver: func(status int, code string) {
			observability.ObserveFeedbackHTTPResponse("iot", status, code)
		},
		SchoolName:              devicescanCompose.NewSchoolName(schoolName(organizations), DeviceScanPrincipal),
		ErrorReports:            NewIoTErrorReporter(errorReports),
		SessionEnd:              bindIoTHostSessionEnd(BindIoTSessionEnd(sessionEnd)),
		SessionLifecycle:        scanBindings.NewSessionLifecycle(active, scanBindings.NewSupervisionQuery(presence), NewDeviceScanPeople(users), iot, NewKioskSessionMirror(timetableRepos, activities, KioskMirrorPublisher(hub, logger), logger), logger, DeviceScanPrincipal),
		Logger:                  logger.With("handler", "iot"),
		DeviceAuthenticator:     gateDeviceAuthentication(deviceAuth.Device()),
		DeviceOnlyAuthenticator: gateDeviceAuthentication(deviceAuth.DeviceOnly()),
	}), nil
}
