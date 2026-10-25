package owners

import (
	"fmt"

	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"

	appointmentsModule "github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	observabilityCompose "github.com/moto-nrw/project-phoenix/observability/compose"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	reminderCompose "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/compose"
	reminderPorts "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

// PresenceReports are Student Presence's cleanup, absence overview, student
// history and Statistik report (#2606).
type PresenceReports struct {
	ActiveCleanup   studentpresence.PresenceCleanup
	AbsenceOverview studentpresence.StatusDayOverviews
	StudentHistory  studentpresence.StudentHistory
	Statistics      studentpresence.StatisticsReports
}

// BuildPresenceReports binds the presence cleanup, the absence overview, the
// student history and the statistics.
func BuildPresenceReports(
	db transactionruntime.Database, env RootEnvironment, presence PresenceOwner, carePlanRows CarePlanRows, carePlan careplan.Capability,
	users peoplePorts.PersonDirectory, owner TimetableOwner, rooms facilitiesModule.Catalog, calendar schoolcalendar.Days,
	people PeopleRows, audit AuditOwner, settings settingsCompose.SettingsService,
) PresenceReports {
	return PresenceReports{
		ActiveCleanup:   BuildPresenceCleanup(env, presence, audit, settings),
		AbsenceOverview: presenceservice.NewStatusDayOverviews(carePlanRows.StatusDays, StatusDayOverviewPeople(users)),
		StudentHistory: presenceservice.NewStudentHistory(presence.Presence, historyRoomNames(rooms),
			NewDataAccessAudit(audit.Persistence.DataAccessLog), NewHistorySlots(owner.Repositories.InstanceStudent.Reads)),
		Statistics: newStatisticsReports(db, carePlan, calendar, people.Students, rooms, env.Logger, env.Now)(audit.Persistence.DataAccessLog, settings),
	}
}

// BuildPresenceCleanup binds the retention command used by jobs and setting writes.
func BuildPresenceCleanup(env RootEnvironment, presence PresenceOwner, audit AuditOwner, settings settingsCompose.SettingsService) studentpresence.PresenceCleanup {
	_, supervisors := presenceCompose.SessionRepositories(presence.SessionRecords)
	return presenceservice.NewPresenceCleanup(presence.Presence, supervisors, NewDeletionAudit(audit.Persistence.DataDeletion), settingsCompose.PresenceSettings(settings), env.Today)
}

// BuildCalendarApp binds the staff and parent personal calendars over the
// facts, the outbox and the notification router.
func BuildCalendarApp(
	db transactionruntime.Database, env RootEnvironment, people PeopleRows, structure StructureRows, portal carePlanCompose.ParentPortalRepositories,
	owner TimetableOwner, rooms facilitiesModule.Catalog, workTime workforce.Capability, organizations organizationtenancy.Query,
	identity identityaccess.Owner, caller identityaccess.CallerContext, appointments appointmentsModule.Capability,
	calendar schoolcalendar.Days, delivery DeliveryRuntime, notifications DeliveryNotifications, settings settingsCompose.BatchSettingsService,
	persons peopledirectory.GuardianPortalContacts,
) calendarCompose.Application {
	repos, links := owner.Repositories, env.Links
	cfg := NewCalendarFacts(people.Staff, people.Students, NewCalendarGuardianDirectory(people.GuardianProfiles, persons), people.StudentGuardians,
		carePlanCompose.NewParentChildLinks(portal.ParentChild), structure.Groups)
	cfg = BindCalendarScheduleFacts(cfg, repos.InstanceStaff, repos.ActivityInstance.Reads, facilitiesCompose.NewRoomRecords(rooms),
		newWorkforceShiftRows(workTime), newWorkforceShiftTypeRows(workTime))
	cfg = BindCalendarFeedFacts(cfg, organizations, identity, identity, people.Persons)
	cfg = BindCalendarCaller(cfg, caller)
	cfg.Appointments, cfg.SchoolDays = appointments, calendar
	cfg.Runtime, cfg.CalendarRenderer = calendarCompose.NewTenantRuntime(db.Pool()), calendarCompose.NewCalendarRenderer(calendar)
	cfg.Outbox, cfg.PushOutbox = NewCalendarOutbox(delivery.Outbox), deliveryCompose.NewDurablePushOutbox(delivery.Runtime.Module)
	cfg.Settings, cfg.CalDAVPolicy = settings, NewCalendarCalDAVPolicy(settings)
	cfg.StaffFeedTombstoneRepo = schoolCalendarCompose.NewFeedHistory(db.Pool())
	cfg.ParentsURL, cfg.FrontendURL, cfg.CalDAVURL = links.ParentsURL, links.FrontendURL, links.PublicAPIURL
	cfg.Notifier, cfg.ReminderNotifier = NewCalendarNotifier(notifications.Notifier), NewCalendarReminderNotifier(notifications.Notifier)
	cfg.Preferences, cfg.Logger = notifications.Preferences, env.Logger.With("service", "calendar")
	return NewCalendarPortal(cfg)
}

// BuildCalendarAppointmentRenderer renders the four appointment mail kinds
// of the calendars with one renderer.
func BuildCalendarAppointmentRenderer(db transactionruntime.Database, mailFrom email.Email, people PeopleRows) deliveryCompose.MailRenderer {
	return deliveryCompose.MailRendererFunc(NewCalendarAppointmentRenderer(mailFrom, calendarCompose.NewTenantRuntime(db.Pool()), people.StudentGuardians))
}

// BuildReminders binds the reminder evaluation's readers to the owners and
// the reminder commands to the calendars and the parent announcements.
func BuildReminders(
	db transactionruntime.Database, env RootEnvironment, caller CareRequestStaff, settings settingsCompose.SettingsService,
	schedules CarePlanSchedules, active studentpresence.Presence, supervisions studentpresence.SupervisionRecords,
	timetableOwner timetable.Capability, rooms facilitiesModule.Catalog, people PeopleRows, calendarApp calendarCompose.Application,
	announcements communication.ParentAnnouncementReminders, delivery DeliveryRuntime,
) reminder.Module {
	presence := NewLoggedStudentPresence(db, env.Logger)
	return reminder.Module{
		Query: reminderCompose.NewQuery(reminderPorts.QueryDependencies{
			Clock:        ReminderClock(),
			CurrentStaff: ReminderStaffIdentity(caller),
			Settings:     ReminderSettings{settings},
			Attendance:   presence,
			Pickup:       ReminderPickupReader{Source: schedules.Pickups},
			Instance:     ReminderTimetableReader{Source: timetableOwner},
			Room:         ReminderRoomReader{Source: rooms},
			Student:      ReminderStudentReader{Source: people.Students},
			Person:       ReminderPersonReader{Source: people.Persons},
			Supervision:  ReminderSupervisionReader{Source: active, Presence: presence},
			Visits:       ReminderVisitReader{Source: presence},
			Logger:       env.Logger.With("service", "reminders"),

			// Bulk readers for ComputeBatch. They answer the three genuinely
			// per-person facts for the whole tenant in one query each, which is what
			// keeps the per-minute cost flat in the number of staff.
			BulkSupervision:   ReminderBulkSupervisionReader{Source: supervisions},
			BulkInstanceStaff: ReminderTimetableReader{Source: timetableOwner},
		}),
		Command:                   reminderCompose.NewCommand(ReminderCommandRuntime(db), ReminderDeliveryDependencies(db, delivery.Runtime, calendarApp)),
		ParentAnnouncementCommand: announcements,
	}
}

// BuildProvisioning binds Organisation & Tenancy's operator
// provisioning (#3253) over the owners it provisions and the device fleet.
func BuildProvisioning(
	db transactionruntime.Database, env RootEnvironment, organizations organizationtenancy.Capability, identity identityaccess.Owner,
	persons peopledirectory.Capability, membership schoolmembership.Capability, people PeopleRows, devices devicefleet.Capability,
	sessions presenceCompose.SessionRecords, supervisions studentpresence.SupervisionRecords, owner TimetableOwner,
	operatorAuditLog OperatorAuditEntries, settings settingsCompose.SettingsService,
) (organizationtenancy.Provisioning, error) {
	adapters, err := NewOperatorProvisioningAdapters(
		operatorPersonStudentMembership(db), devices, persons, membership, people.Persons, people.Staff, identity,
		sessions, supervisions, arrivalTimetable.NewActivityCategorySeeder(owner.Repositories.ActivityCategory), operatorAuditLog,
	)
	if err != nil {
		return nil, fmt.Errorf("compose operator provisioning: %w", fmt.Errorf("compose operator provisioning adapters: %w", err))
	}
	provisioning, err := newOperatorProvisioning(organizations, identity, NewOperatorAccountDirectory(identity, persons, membership, organizations),
		adapters, settings, env.Logger.With("service", "platform"))
	if err != nil {
		return nil, fmt.Errorf("compose operator provisioning: %w", err)
	}
	return provisioning, nil
}

// DeliveryDevices are Delivery's push subscriptions and the PWA standalone
// usage (#2189).
type DeliveryDevices struct {
	PushSubscriptions deliveryCompose.PushSubscriptionService
	PWAUsage          deliveryCompose.PWAUsageService
}

// BuildDeliveryDevices binds the push subscriptions and the PWA standalone
// usage over the root's unit of work.
func BuildDeliveryDevices(db transactionruntime.Database, unit rootUnitOfWork, env RootEnvironment, pushVAPID deliveryCompose.VAPIDConfig, identity identityaccess.Owner, provisioning organizationtenancy.Provisioning, settings settingsCompose.SettingsService) (DeliveryDevices, error) {
	push, err := deliveryCompose.NewTenantPushSubscriptionService(unit, db.Pool(), deliveryCompose.NewPushSubscriptionRepository(db.Pool()), identity,
		pushVAPID, env.Logger.With("service", "push_subscriptions"))
	if err != nil {
		return DeliveryDevices{}, err
	}
	usage, err := deliveryCompose.NewTenantPWAUsageService(unit, db.Pool(), observabilityCompose.NewPWAStandaloneUsageRepository(db.Pool()),
		pwaUsageCounts{provisioning: provisioning}, identity, settings, env.Logger.With("service", "pwa_usage"))
	if err != nil {
		return DeliveryDevices{}, err
	}
	return DeliveryDevices{PushSubscriptions: push, PWAUsage: usage}, nil
}

// BuildPlanExport binds the printable weekly plans (#2079, #2706): a pure
// projection over the reads the two planning screens use. It renders, it
// never writes.
func BuildPlanExport(env RootEnvironment, shifts *workforceCompose.ShiftPlanning, workTime workforce.Capability, owner TimetableOwner, rooms facilitiesModule.Catalog, people PeopleRows, timetableOwner timetable.Capability, calendar schoolcalendar.Days, renderer *lists.RendererService) planexport.Service {
	repos := owner.Repositories
	return newPlanExport(planExportSources{
		Overview: shifts.Overview, ShiftTypes: workTime,
		Instances:     arrivalTimetable.NewTimetableInstanceReads(repos.ActivityInstance.Reads),
		InstanceStaff: arrivalTimetable.NewTimetableInstanceStaffReads(repos.InstanceStaff),
		Students:      repos.ParticipantCounts(), Rooms: rooms, Staff: planExportStaffNames{staff: people.Staff},
		ActivityGroups: arrivalTimetable.NewTimetableGroupReads(repos.Timetable), PlanningTracks: timetableOwner,
		ClosingDays: planExportClosingDays{calendar: calendar}, Holidays: planExportHolidays{calendar: calendar},
		BlockGroups: timetableOwner, EducationGroupNames: owner.EducationGroupNames,
		Renderer: renderer, Logger: env.Logger.With("service", "plan_export"),
	})
}

// DeliveryModuleOf is Delivery's module of a runtime.
func DeliveryModuleOf(delivery DeliveryRuntime) deliveryModule.Capability {
	return delivery.Runtime.Module
}
