package api

import (
	"context"
	"fmt"

	classdayCompose "github.com/moto-nrw/project-phoenix/modules/classday/compose"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	blockDisplayCompose "github.com/moto-nrw/project-phoenix/modules/timetableblockdisplay/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	observabilityCompose "github.com/moto-nrw/project-phoenix/observability/compose"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	shiftplansyncCompose "github.com/moto-nrw/project-phoenix/workflows/shiftplansync/compose"
	studentdeletioncompose "github.com/moto-nrw/project-phoenix/workflows/studentdeletion/compose"
)

// composeViews binds the operator provisioning of Organisation & Tenancy
// (#3253), the Notfallliste (#2704), the slot lists (#2701), the printable
// weekly plans (#2079, #2706) and the push and PWA usage services.
func composeViews(b graphBuild) (graphBuild, error) {
	repos := b.TimetableWiring.Repositories
	provisioning, err := b.Platform.OperatorProvisioning(OperatorProvisioningSources{
		Identity: b.identityAccess, Accounts: NewOperatorAccountDirectory(b.identityAccess, b.Persons, b.Membership, b.Organizations),
		Devices: b.deviceFleet, Sessions: b.Presence.SessionRecords, Supervisions: b.groupSupervisions,
		Categories: arrivalTimetable.NewActivityCategorySeeder(repos.ActivityCategory),
		Settings:   b.settingsService, Logger: b.Logger.With("service", "platform"),
	})
	if err != nil {
		return b, fmt.Errorf("compose operator provisioning: %w", err)
	}
	b.graph.OperatorProvisioning = provisioning
	// The Notfallliste is the emergency snapshot read projection: the owner
	// facades are its tenant-safe reads, the retained student and guardian
	// repositories, the presence-mode read and the renderer its
	// compatibility bindings.
	emergency, err := newEmergencySnapshot(emergencySnapshotSources{
		Presence: b.Presence.Presence, PresenceMode: b.graph.Active, Students: b.People.Students, Persons: b.Persons,
		Contacts: b.People.StudentGuardians, Rooms: b.Rooms, Renderer: b.Platform.ListExport, Now: b.now,
		Logger: b.Logger.With("module", "emergency-snapshot"),
	})
	if err != nil {
		return b, fmt.Errorf("compose emergency snapshot projection: %w", err)
	}
	b.graph.Emergency = emergency
	b.graph.SlotLists = classdayCompose.NewSlotLists(classdayCompose.SlotListDependencies{
		Timetable: b.TimetableWiring.ClassDayTimetable, Presence: b.Presence.Presence, CarePlan: b.CarePlan,
		Students: b.Persons, Persons: b.Persons, Groups: b.Groups, Rooms: b.Rooms, CareDays: b.graph.CareDay,
		PickupTimes: b.graph.PickupSchedule, ArrivalTimes: b.graph.ArrivalSchedule, PickupBaselines: b.pickupBaselines,
		Settings: ClassDaySlotListSettings{Settings: b.settingsService}, Access: ClassDaySlotListAccess{Staff: b.userContext},
		ListExport: ClassDaySlotListRenderer{Renderer: b.Platform.ListExport}, EnrolledOn: ClassDayEnrolledOn,
	})
	return composeDeviceServices(composePlanExport(b)), nil
}

// composePlanExport binds the printable weekly plans: a pure projection over
// the reads the two planning screens use. It renders, it never writes.
func composePlanExport(b graphBuild) graphBuild {
	repos := b.TimetableWiring.Repositories
	b.graph.PlanExport = newPlanExport(planExportSources{
		Overview: b.shiftPlanning.Overview, ShiftTypes: b.WorkTime,
		Instances:     arrivalTimetable.NewTimetableInstanceReads(repos.ActivityInstance),
		InstanceStaff: arrivalTimetable.NewTimetableInstanceStaffReads(repos.InstanceStaff),
		Students:      repos.InstanceStudent, Rooms: b.Rooms, Staff: b.Workforce.PlanExportStaff,
		ActivityGroups: arrivalTimetable.NewTimetableGroupReads(repos.ActivityGroup), PlanningTracks: b.Timetable,
		ClosingDays: b.TimetableWiring.PlanExportClosingDays, Holidays: b.TimetableWiring.PlanExportHolidays,
		Renderer: b.Platform.ListExport, Logger: b.Logger.With("service", "plan_export"),
	})
	b.graph.StaffShifts = b.shiftPlanning.Planning(b.graph.PlanExport)
	return b
}

// composeDeviceServices binds the push subscriptions and the PWA standalone
// usage of Delivery.
func composeDeviceServices(b graphBuild) graphBuild {
	pushSubscriptions := deliveryCompose.NewPushSubscriptionRepository(b.DB.Pool())
	b.graph.PushSubscriptions = deliveryCompose.NewPushSubscriptionService(b.DB.Pool(), pushSubscriptions, b.identityAccess,
		b.vapid, b.Logger.With("service", "push_subscriptions"))
	b.graph.PWAUsage = deliveryCompose.NewPWAUsageService(b.DB.Pool(), observabilityCompose.NewPWAStandaloneUsageRepository(b.DB.Pool()),
		b.Platform.PWAUsageCounts(b.graph.OperatorProvisioning), b.identityAccess, b.settingsService,
		b.Logger.With("service", "pwa_usage"))
	return b
}

// composeProjections binds the reminders, the absence overview, the live
// group view and the supervision dashboard.
func composeProjections(b graphBuild) (graphBuild, error) {
	b.graph.Reminders = reminder.Module{
		Query: b.TimetableWiring.ReminderQuery(ReminderSources{
			CurrentStaff: b.userContext, Settings: b.settingsService, Pickups: b.graph.PickupSchedule,
			Sessions: b.graph.Active, Supervisions: b.groupSupervisions, Logger: b.Logger.With("service", "reminders"),
		}),
		Command:                   b.TimetableWiring.ReminderCommand(b.graph.Calendar),
		ParentAnnouncementCommand: b.graph.ParentAnnouncement,
	}
	b.graph.AbsenceOverview = presenceservice.NewStatusDayOverviews(b.CarePlanWiring.StatusDays, b.People.StatusDayPeople(b.graph.Users))
	live, err := newGroupLive(groupLiveSources{
		Presence: b.Presence.Presence, People: b.People.GroupRosterPeople(b.graph.Users), Education: b.graph.Education,
		Substitutions: b.graph.Substitution, UserContext: b.identity.GroupLiveCaller, Active: b.graph.Active,
		Settings: b.settingsService, Pickups: b.graph.PickupSchedule, Arrivals: b.graph.ArrivalSchedule,
		PlannedStudentIDs: b.instances.GetPlannedStudentIDsByDate, CareDays: b.graph.CareDay,
		CareParticipation: b.graph.CareLifecycle, ExcusedRequests: b.excusedRequests, StatusDays: b.Presence.StatusDays,
		Logger: b.Logger.With("service", "ogs-group-live"), Now: b.now,
	})
	if err != nil {
		return b, fmt.Errorf("compose OGS group live projection: %w", err)
	}
	b.graph.OGSGroupLive = live
	dashboard, err := newSupervisionDashboard(supervisiondashboard.Dependencies{
		Access:   newSupervisionAccess(b.settingsService, b.identity.SupervisionCaller),
		Sessions: newSupervisionSessions(b.graph.Active, b.Presence.OpenRoomSessions, b.identity.SupervisionCaller),
		Rooms:    b.Presence.OpenRooms, Yard: newSupervisionYard(b.graph.Schulhof),
		Groups:   newSupervisionGroups(b.graph.Education, b.identity.SupervisionCaller),
		Schedule: newSupervisionSchedule(b.graph.TimetableOperations),
		Presence: newSupervisionPresence(b.graph.Active, b.graph.Active),
		Planning: newSupervisionPlanning(b.graph.PickupSchedule, b.graph.ArrivalSchedule),
		Settings: newSupervisionSettings(b.settingsService), Calendar: supervisionCalendar{}, Now: b.now,
	})
	if err != nil {
		return b, fmt.Errorf("compose supervision dashboard projection: %w", err)
	}
	b.graph.SupervisionDashboard = dashboard
	b.graph.StudentHistory = presenceservice.NewStudentHistory(b.Presence.Presence, b.TimetableWiring.HistoryRooms,
		b.Platform.DataAccessAudit(b.audits.DataAccessLog), b.TimetableWiring.HistorySlots)
	b.graph.Statistics = b.Presence.Statistics(StatisticsSources{AccessLog: b.audits.DataAccessLog, Settings: b.settingsService})
	return b, nil
}

// composePlanning binds the Timetable owner's planning reads and writes:
// the planner's data, the template writes (#3424 slice S2) including "Dieser
// und alle folgenden", the series conversion and the attendance corrections.
// A split caps the old template, creates a successor and re-plans the window
// through the materialization, keeping the Vertretungsplan overrides;
// Enrollment's roster resync reconciles offering-sourced rosters (#2137,
// #2147).
func composePlanning(b graphBuild) (graphBuild, error) {
	data, err := b.TimetableWiring.Data(TimetableDataInputs{
		Rows: b.timetableRows, ArrivalBaselines: b.arrivalBaselines, PickupBaselines: b.pickupBaselines,
		Visits: b.Presence.Presence, Groups: b.Timetable, Sessions: b.Presence.SessionRecords, ConflictAcks: b.Timetable,
		BlockMetadata: arrivalTimetable.NewBlockDisplayMetadata(b.DB.Pool(), blockDisplayCompose.List),
		Transactional: true, Logger: b.Logger.With("service", "timetable-data"),
	})
	if err != nil {
		return b, fmt.Errorf("compose timetable data: %w", err)
	}
	templates, err := b.composeTemplates()
	if err != nil {
		return b, err
	}
	converter, err := arrivalTimetable.NewInstanceSeriesConversion(timetablePorts.InstanceSeriesConversionDependencies{
		Transactions: arrivalTimetable.NewTenantTransactions(b.DB.Pool()), InstanceRepo: b.TimetableWiring.Repositories.ActivityInstance, Lifecycle: b.instances,
		Templates: templates, RecurrenceLock: b.TimetableWiring.RecurrenceLock,
	})
	if err != nil {
		return b, fmt.Errorf("compose timetable series conversion: %w", err)
	}
	b.graph.InstanceSeriesConverter = converter
	corrections, err := b.TimetableWiring.AttendanceCorrections(TimetableCorrectionInputs{
		Rows: b.timetableRows, Trail: b.audits.AttendanceCorrection, People: timetableCorrectionPeople{rows: b.People.Persons},
		Logger: b.Logger.With("service", "attendance-correction"),
	})
	if err != nil {
		return b, fmt.Errorf("compose timetable attendance corrections: %w", err)
	}
	b.graph.TimetableData = arrivalTimetable.Planning{
		Templates: templates, RecurrenceLock: b.TimetableWiring.RecurrenceLock, Data: data,
		ConflictDetection: b.timetableConflicts, AttendanceCorrections: corrections, Deviations: b.staffDeviations,
	}
	return b, nil
}

// composeTemplates binds the template writes over the materialization, the
// lifecycle's series deviations, the care-offering guards and the roster
// resync.
func (b *graphBuild) composeTemplates() (timetable.TemplateAdministration, error) {
	deps, err := b.TimetableWiring.Templates(TimetableTemplateInputs{
		Rows: b.templateRows, PlanningTracks: b.TimetableWiring.PlanningTracks, Materialization: b.graph.Materialization,
		Deviations: b.instances.SeriesDeviations(), CareOfferings: b.careOfferings,
		ResyncOfferingRoster: b.CarePlanWiring.OfferingRosterResync(b.late.careBookings),
		RecurrenceLock:       b.TimetableWiring.RecurrenceLock, Broadcaster: b.graph.RealtimeHub,
		Logger: b.Logger.With("service", "timetable-templates"), Today: b.today,
	})
	if err != nil {
		return nil, fmt.Errorf("compose timetable templates: %w", err)
	}
	deps.Transactions = arrivalTimetable.NewTenantTransactions(b.DB.Pool())
	templates, err := arrivalTimetable.NewTemplateAdministration(deps)
	if err != nil {
		return nil, fmt.Errorf("compose timetable templates: %w", err)
	}
	return templates, nil
}

// composeParentRequests binds the Stammdaten decisions (#3354) and the
// cross-kind bulk approval and conflict resolution (#2267, stories 6-10).
// People Directory keeps the child's record and its change history behind
// the decision's records port; the review scope is the one the Stammdaten
// queue is listed with. Every queue is bound at construction, or the
// resolve route would answer conflict_kind_unsupported for the missing one.
func composeParentRequests(b graphBuild) (graphBuild, error) {
	masterData, err := b.People.MasterDataDecisions(MasterDataDecisionWiring{
		CarePlan: b.CarePlan, Fields: b.Persons, Students: b.People.Students, Persons: b.People.Persons,
		Audit: b.graph.StudentAudit, Scope: b.People.ParentRequestWriteScope(b.reviewPolicy, b.Identity.Permissions),
		Emitter: b.graph.ParentEventEmitter, Broadcaster: b.graph.RealtimeHub, Events: peopleCompose.NewParentRequestEventRepository(b.DB.Pool()),
		Shares: b.requestShares, Logger: b.Logger.With("service", "master-data-review"), Today: b.today,
	})
	if err != nil {
		return b, fmt.Errorf("compose master data decisions: %w", err)
	}
	b.graph.MasterDataReview = masterData
	// Co-guardian notices (#2267, story 47): every request domain reads the
	// parents domain's sharing rules through requestShares, which resolves
	// the parents portal bound here.
	if resolver, ok := any(b.graph.Parent).(communicationCompose.ParentShareVisibilityResolver); ok {
		b.late.requestShareVisibility = resolver
	}
	coordinator, err := b.People.ParentRequestCoordinator(masterData, b.excusedRequests, b.graph.CareRequests,
		b.Enrollment.OfferingConflicts(b.offeringChanges), peopleCompose.NewParentRequestEventRepository(b.DB.Pool()), b.Identity.Permissions)
	if err != nil {
		return b, fmt.Errorf("compose parent request coordinator: %w", err)
	}
	b.graph.ParentRequests = coordinator
	return b, nil
}

// composeSettingsWrites binds the settings side effects the API dispatches
// on a value change and the tenant settings operations.
func composeSettingsWrites(b graphBuild) (graphBuild, error) {
	registry := settingsCompose.NewSideEffectRegistry()
	registerCheckoutSettingsSideEffects(registry, b.graph.Schulhof, b.graph.WC)
	b.CarePlanWiring.SettingsSideEffects(registry, b.graph.CareLifecycle)
	if b.graph.StudentPhotos != nil {
		RegisterStudentPhotoSettingsSideEffects(registry, b.graph.StudentPhotos)
	}
	b.graph.SettingsSideEffects = registry
	b.graph.TenantSettings = settingsCompose.NewTenantOperations(
		b.settingsService, b.graph.PayrollStatus, b.settings, registry.Dispatch, b.settingsChanged,
	)
	return b, nil
}

// composeLateWorkflows binds what needs the assembled graph: the #1843 sick
// cascade, the People Directory's guardian provider (#2663) and the
// permanent child deletion (#2710), which unlinks a deleted child's photo
// through the photo lifecycle.
func composeLateWorkflows(b graphBuild) (graphBuild, error) {
	repos := b.TimetableWiring.Repositories
	cascade, err := shiftplansyncCompose.NewSickCascade(shiftplansyncCompose.SickCascadeDependencies{
		Planning: b.graph.StaffShifts, Workforce: b.WorkTime, LockStaffShifts: workforceCompose.NewStaffShiftLock(b.DB.Pool()),
		Deviations:    b.staffDeviations,
		TimetableData: b.timetableRows.SickCascadeRows(arrivalTimetable.SubstituteDayLock(b.DB.Pool())),
		InstanceStaff: arrivalTimetable.NewTimetableInstanceStaffReads(repos.InstanceStaff),
		Broadcaster:   b.graph.RealtimeHub, Logger: b.Logger.With("service", "shift_plan_sync"), Today: b.today,
	})
	if err != nil {
		return b, err
	}
	b.late.shiftPlanSyncer = cascade
	peopleCompose.BindGuardianDirectory(b.Persons, &peopleCompose.GuardianDirectoryProvider{
		Guardians: b.graph.Guardian, Runtime: peopleCompose.NewLegacyRepositoryRuntime(b.DB.Pool()),
		PermissionGranted: securityruntime.GuardianPermissionGranted,
	})
	deletionOwners, err := NewStudentDeletionOwners(b.DB)
	if err != nil {
		return b, fmt.Errorf("compose student deletion owners: %w", err)
	}
	deletion, err := studentdeletioncompose.New(studentdeletioncompose.Dependencies{
		DB: b.DB.Pool(), Owners: deletionOwners, Directory: b.Persons, CarePlan: b.CarePlan, Timetable: b.Timetable,
		Feedback: feedbackCounterOrUnconfigured(b.FeedbackCounter), IsVerifiedStaff: b.userContext.HasCurrentStaff,
		LockCareBookingWrites: b.TimetableWiring.RecurrenceLock.LockRecurrenceWrites,
		UnlinkPhoto:           unlinkStudentPhoto(b.graph.StudentPhotos),
		Broadcaster:           b.graph.RealtimeHub, Logger: b.Logger.With("workflow", "student_deletion"), Audit: b.graph.Audit,
	})
	if err != nil {
		return b, fmt.Errorf("compose student deletion workflow: %w", err)
	}
	b.graph.StudentDeletion = deletion
	return b, nil
}

// unlinkStudentPhoto schedules a deleted child's photo file for removal after
// the deletion commits; a graph without the photo lifecycle unlinks nothing.
func unlinkStudentPhoto(photos peoplePorts.StudentPhotoService) func(context.Context, string) {
	return func(ctx context.Context, path string) {
		if photos != nil {
			photos.ScheduleUnlinkAfterCommit(ctx, path)
		}
	}
}
