package api

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/spf13/viper"

	"github.com/go-chi/chi/v5"
	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	classdayHTTP "github.com/moto-nrw/project-phoenix/modules/classday/http"
	staffMessagingAPI "github.com/moto-nrw/project-phoenix/modules/communication/http/staffmessages"
	dataImportCompose "github.com/moto-nrw/project-phoenix/modules/dataimport/compose"
	importAPI "github.com/moto-nrw/project-phoenix/modules/dataimport/http"
	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	roomsHTTPAdapter "github.com/moto-nrw/project-phoenix/modules/facilities/http"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	calendarAPI "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/http"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	schoolMembershipModule "github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	groupsHTTP "github.com/moto-nrw/project-phoenix/modules/schoolstructure/http"
	settingsFacts "github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	settingsHTTP "github.com/moto-nrw/project-phoenix/modules/settings/http"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetableAPI "github.com/moto-nrw/project-phoenix/modules/timetable/http"
	timetableHTTPAdapter "github.com/moto-nrw/project-phoenix/modules/timetable/http/activities"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/workflows/careexit"
	"github.com/moto-nrw/project-phoenix/workflows/gradetransition"
	gradeTransitionHTTP "github.com/moto-nrw/project-phoenix/workflows/gradetransition/http"
)

// mountStaffRoutes serves the staff administration, shift planning,
// substitutions and time tracking.
func mountStaffRoutes(
	tenant chi.Router, db transactionruntime.Database, logger *slog.Logger, membership schoolMembershipModule.Capability,
	workTime workforceModule.Capability, users peoplePorts.PersonDirectory, staffAdmin *workforceModule.StaffAdmin,
	workforce owners.WorkforcePlanning, shifts *workforceCompose.ShiftPlanning, education schoolstructure.GroupManagement,
	substitution SubstitutionOperations, persons peopledirectory.Capability, settings settingsCompose.SettingsService,
	hub *realtime.Hub, identity identityaccess.Owner, caller *owners.CallerRows, calendar schoolcalendar.Calendar,
) error {
	staff, _, err := newStaffComposition(membership, workTime, users, staffAdmin, workforce.TimeTracking, workforce.SickReports, education, persons, settings, hub, identity, viper.GetString("app_env"), db, logger.With("handler", "staff"))
	if err != nil {
		return err
	}
	staffShifts := newStaffShiftsResource(staffShiftsDependencies{
		Planning:       workforce.StaffShifts,
		ResolveStaffID: currentStaffID(caller),
		ActorAccountID: sessiontoken.ActorAccountIDFromCtx,
	})
	absenceTypes := newAbsenceTypesResource(workforceCompose.NewAbsenceTypeAdministration(workTime, logger.With("service", "active")), currentStaffID(caller)).Router()
	tenant.Mount("/staff", staff.Router())
	tenant.Mount("/work-time-models", newWorkTimeModelsResource(workTime, staffTimeTrackingNotifier(hub)).Router())
	tenant.Mount("/staff-shifts", staffShifts.Router())
	tenant.Mount("/shift-types", newShiftTypesResource(shifts.ShiftTypes).Router())
	tenant.Mount("/absence-types", absenceTypes)
	tenant.Mount("/substitutions", newSubstitutionsResource(SubstitutionCapability(substitution)).Router())
	tenant.Mount("/time-tracking", newTimeTrackingResource(users, staffAdmin, workforce.TimeTracking, workforce.SickReports, workforce.StaffShifts, shifts.Assignments, calendar, settings).Router(ProtectedTenantRoutes))
	return nil
}

// mountSchoolPortalRoutes serves the school portal ("moto schule", #2207) at
// the root: public /school/auth/* (login and the school-scope MFA exchange)
// plus the class-day surface. Token refresh and logout go through the shared
// scope-preserving /auth/refresh and /auth/logout. The portal reuses the
// timetable, colleague chat, staff notice and notification resources the
// staff reach under /api (#2207, #2527).
func mountSchoolPortalRoutes(
	root, tenant chi.Router, db transactionruntime.Database, logger *slog.Logger, limiters authRateLimiters,
	presence studentpresence.Owner, timetableOwner timetableModule.Capability, calendar schoolcalendar.Calendar,
	materialization timetableModule.MaterializationCapability, lifecycle owners.TimetableLifecycle, planning TimetablePlanning,
	planningTracks timetableModule.PlanningTrackAdministration, users peoplePorts.PersonDirectory, schedules owners.CarePlanSchedules,
	caller *owners.CallerRows, settings settingsCompose.SettingsService, careOfferings carePlanModule.CareOfferingCapability,
	planExport planexport.Service, chat owners.CommunicationMessaging, persons peopledirectory.Capability, notifications owners.DeliveryNotifications,
	devices owners.DeliveryDevices, classDay ClassDayViews, identity identityaccess.Owner, calendarApp calendarCompose.Application,
) error {
	pickupExtensions, err := careexit.NewPickupExtensions(timetableOwner, presence)
	if err != nil {
		return err
	}
	timetableResource := newTimetableResource(calendar, timetableOwner, materialization, lifecycle, planning, planningTracks, users, schedules,
		caller, settings, careOfferings, classDay, planExport, pickupExtensions, persons, logger)
	// OGS-internal colleague chat (#2598) — staff-to-staff, deliberately a
	// separate surface from /messages (which is parent-facing).
	staffMessaging := staffMessagingAPI.NewResource(chat.StaffMessaging)
	// Tagesinformationen (#2180) are Timetable's: the owner composes the
	// service over its own repository, the calendar periods for the week
	// pattern and the People Directory names of the acknowledgement list
	// (#3418). All staff read them, admins write them.
	staffNotices := timetableAPI.NewStaffNoticeResource(timetableCompose.NewStaffNotices(timetableCompose.StaffNoticeDependencies{
		DB: db.Pool(), Periods: timetableCompose.NewCalendarPeriodRepository(calendar, newCalendarPeriodUsage(timetableOwner)), Names: staffNoticeNames(persons),
		Logger: logger.With("service", "staffnotice"),
	}), func(ctx context.Context) int64 { return timeTrackingIdentity(ctx).AccountID })
	// Notification abstraction (#1624).
	notificationRoutes := NewNotificationHTTP(notifications.Notifier, devices.PushSubscriptions, notifications.Preferences)
	// The class-day surface reads the class-day projection (#2701): the day
	// report over Enrollment's day roster and the arrival-exception write
	// seam (#2970) behind its one public capability.
	classDayResource := classdayHTTP.NewResource(classDay.ClassDay, logger.With("handler", "class-day"))
	// The calendar of moto schule (#4798): the same calendar service as
	// /api/calendar, behind calendar:school and the school-portal mantle.
	schoolCalendar := calendarAPI.NewResource(calendarApp, logger.With("handler", "school-calendar"))
	school := NewSchoolPortalHTTP(SchoolPortalAuth(identity), SchoolPortalMFA(identity), schoolPasswordResets(identity), classDayResource, timetableResource, staffMessaging, staffNotices, notificationRoutes, schoolCalendar)
	tenant.Mount("/staff-messages", staffMessaging.Router(ProtectedTenantRoutes))
	tenant.Mount("/staff-notices", staffNotices.Router(ProtectedTenantRoutes))
	tenant.Mount("/timetable", timetableResource.Router(ProtectedTenantRoutesWith(classdayHTTP.NewSlotListResource(classDay.SlotLists).Register)))
	tenant.Mount("/notifications", notificationRoutes.Router())
	root.Mount("/school", school.RouterWithAuthRateLimiter(limiters.authMiddleware(), SchoolSessionChain, ProtectedSchoolRoutes))
	return nil
}

// newTimetableResource composes the timetable routes. Dateframes belong to
// the School Calendar; the conflict detection is the one the instance
// lifecycle uses.
func newTimetableResource(
	calendar schoolcalendar.Calendar, timetableOwner timetableModule.Capability, materialization timetableModule.MaterializationCapability,
	lifecycle owners.TimetableLifecycle, planning TimetablePlanning, planningTracks timetableModule.PlanningTrackAdministration,
	users peoplePorts.PersonDirectory, schedules owners.CarePlanSchedules, caller *owners.CallerRows, settings settingsCompose.SettingsService,
	careOfferings carePlanModule.CareOfferingCapability, classDay ClassDayViews, planExport planexport.Service,
	pickupExtensions timetableAPI.PickupExtensionDecisions, hints peopledirectory.SchoolDayHintQuery, logger *slog.Logger,
) *timetableAPI.Resource {
	return timetableAPI.NewResource(timetableAPI.Dependencies{
		CalendarPeriods:         calendar,
		ClosingDays:             calendar,
		CalendarPeriodUsage:     calendarPeriodUsage{usage: newCalendarPeriodUsage(timetableOwner)},
		MaterializationService:  materialization,
		InstanceService:         lifecycle.Instances,
		InstanceSeriesConverter: planning.SeriesConverter,
		OperationsService:       lifecycle.Operations,
		People:                  NewTimetablePeople(users),
		TeacherHints:            newTimetableTeacherHints(hints),
		Templates:               planning.Planning.Templates,
		RecurrenceLock:          planning.Planning.RecurrenceLock,
		AttendanceCorrections:   planning.Planning.AttendanceCorrections,
		Deviations:              planning.Planning.Deviations,
		TimetableData:           planning.Planning.Data,
		ConflictDetection:       planning.Planning.ConflictDetection,
		PlanningTracks:          planningTracks,
		CareDayService:          schedules.CareDay,
		UserContextService:      caller,
		SettingsService:         settings,
		OfferingSourceOptions:   NewTimetableOfferingSources(careOfferings),
		SupervisionSheets:       NewTimetableSupervisionSheets(classDay.ClassDay),
		HomeworkFeedback:        timetableOwner,
		PlanExportService:       planExport,
		PickupExtensions:        pickupExtensions,
		Staffing:                timetableStaffingAnnouncer(lifecycle.Instances),
		Logger:                  logger.With("handler", "timetable"),
	})
}

// mountSchoolRoutes serves the school's rooms, groups, activities, schedules,
// enrollment, settings, info-point displays and data administration under
// /api.
func mountSchoolRoutes(
	tenant chi.Router, db transactionruntime.Database, logger *slog.Logger, facilities facilitiesModule.Capability, rooms FacilitiesRooms,
	settings settingsCompose.SettingsService, caller *owners.CallerRows, active studentpresence.Presence, users peoplePorts.PersonDirectory,
	education schoolstructure.GroupManagement, renderer *lists.RendererService, activities activityContract.ActivityService,
	timetableOwner timetableModule.Capability, iot devicefleetCompose.IoTService, settingsWrites SettingsWrites,
	membership schoolMembershipModule.Capability, classListEntries schoolMembershipModule.ClassListEntries, databaseStats DatabaseStatsReader,
	dataImports imports, persons peopledirectory.Capability, gradeTransition *gradetransition.Workflow, enrollment *enrollmentAPI.Resource,
) {
	tenant.Mount("/rooms", roomsHTTPAdapter.NewResource(facilities, roomsHTTPAdapter.Dependencies{
		Facilities:          rooms.RoomService,
		RoomHistorySettings: NewRoomHistorySettings(settings, logger.With("handler", "rooms"), settingsFacts.PrefetchSettings),
		UserContext:         caller, Active: active,
		Users: peopleCompose.NewRoomSnapshotPeople(users), Education: education,
		ListExport: renderer,
	}, logger.With("handler", "rooms"), ProtectedTenantRoutes, transactionFailures.IsConstraintViolation).Router())
	tenant.Mount("/groups", groupsHTTP.NewResource(education, active, peopleCompose.NewGroupRoutePeople(users), caller, transactionFailures.IsConstraintViolation).Router(ProtectedTenantRoutes))
	tenant.Mount("/activities", timetableHTTPAdapter.NewResource(activities, timetableOwner, NewTimetableSupervisorDirectory(users), caller, TimetableHTTPTransactions).Router(ProtectedTenantRoutes))
	// Parent enrollment (PR 5+).
	tenant.Mount("/enrollment", enrollment.Router(TenantSessionChain))
	// Info-point displays (#1325). One Device Fleet owner serves every entry
	// point: the root composes it and the IoT service hands it back here
	// (#2676).
	tenant.Mount("/display", NewDisplayHTTPRouter(iot.Fleet(), DisplayFeatureEnabled(settings)))
	// The timeframe list the activity forms offer (Timetable owner).
	tenant.Mount("/schedules", timetableHTTPAdapter.NewSchedulesResource(timetableOwner).Router(ProtectedTenantRoutes))
	// The schema-driven settings system.
	homeLayouts := requireHomeLayoutOperations(settings)
	tenant.Mount("/settings", newSettingsResource(db, settingsWrites.Tenant, homeLayouts, owners.NewEnrollmentBookings().SchemaReferencesLegalDocument).SettingsRouter())
	// Class-list-only entries (#2382).
	tenant.Mount("/class-list-entries", newClassListEntriesResource(classListEntries, logger.With("handler", "class-list-entries")).Router())
	tenant.Mount("/database", newDatabaseStatsRouter(databaseStats, logger.With("handler", "database")))
	// CSV/Excel import endpoints.
	tenant.Mount("/import", importAPI.NewResource(importAPI.Dependencies{
		Students: dataImports.Student, Staff: dataImports.Staff, ClassList: dataImports.ClassList,
		Files: dataImportCompose.NewFileDecoder(), Runtime: dataImportHTTPRuntime(persons, membership, dataImports.OpeningBalance),
	}).Router())
	tenant.Mount("/admin/grade-transitions", gradeTransitionHTTP.NewGradeTransitionResource(gradeTransition).Router(ProtectedTenantRoutes))
}

// mountCalendarRoutes serves the personal calendars under /api, the
// read-only staff CalDAV and the public subscription feeds.
func mountCalendarRoutes(root, tenant chi.Router, logger *slog.Logger, calendarApp calendarCompose.Application) {
	calendar := calendarAPI.NewResource(calendarApp, logger.With("handler", "calendar"))
	tenant.Mount("/calendar", calendar.Router(TenantSessionChain))

	// Public parent calendar subscription feed (no auth — the token in the URL
	// is the capability). Calendar apps (Apple/Google/Outlook) poll this to keep
	// the parent's Termine in sync.
	root.Get("/public/calendar/{token}", calendarAPI.PublicFeed(calendarApp))

	// Read-only staff CalDAV. Authentication happens inside the protocol
	// handler with the tenant-bound calendar app password, before a tenant is
	// known, so these routes intentionally sit outside the JWT tenant group.
	calDAVHandler := restoreCalDAVMethod(http.HandlerFunc(calendar.ServeCalDAV))
	root.Handle("/.well-known/caldav", calDAVHandler)
	root.Handle("/api/caldav", calDAVHandler)
	root.Handle("/api/caldav/*", calDAVHandler)
}

func requireHomeLayoutOperations(settings any) settingsHTTP.HomeLayoutOperations {
	homeLayouts, ok := settings.(settingsHTTP.HomeLayoutOperations)
	if !ok {
		panic("settings platform does not provide home layout operations")
	}
	return homeLayouts
}

// currentStaffID resolves the caller's staff ID at request time.
func currentStaffID(caller *owners.CallerRows) func(context.Context) (int64, error) {
	return func(ctx context.Context) (int64, error) {
		return caller.Caller().StaffID(ctx)
	}
}

type calDAVOriginalMethodKey struct{}

var calDAVExtensionMethods = map[string]struct{}{
	"ACL": {}, "COPY": {}, "LOCK": {}, "MKCALENDAR": {}, "MKCOL": {},
	"MOVE": {}, "PROPFIND": {}, "PROPPATCH": {}, "REPORT": {}, "UNLOCK": {},
}

func normalizeCalDAVMethodForRouting(r *http.Request) *http.Request {
	isCalDAVPath := r.URL.Path == "/.well-known/caldav" || r.URL.Path == "/api/caldav" || strings.HasPrefix(r.URL.Path, "/api/caldav/")
	if _, ok := calDAVExtensionMethods[r.Method]; !ok || !isCalDAVPath {
		return r
	}
	ctx := context.WithValue(r.Context(), calDAVOriginalMethodKey{}, r.Method)
	routed := r.Clone(ctx)
	routed.Method = "QUERY"
	return routed
}

func restoreCalDAVMethod(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		original, _ := r.Context().Value(calDAVOriginalMethodKey{}).(string)
		if original == "" {
			handler.ServeHTTP(w, r)
			return
		}
		restored := r.Clone(r.Context())
		restored.Method = original
		handler.ServeHTTP(w, restored)
	})
}
