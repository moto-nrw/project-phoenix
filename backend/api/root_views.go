package api

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	realtime "github.com/moto-nrw/project-phoenix/realtime"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	classdayCompose "github.com/moto-nrw/project-phoenix/modules/classday/compose"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/emergencysnapshot"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/grouplive"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	presenceStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/studentpresence/http"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	arrivalTimetable "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/workflows/shiftplansync"
	shiftplansyncCompose "github.com/moto-nrw/project-phoenix/workflows/shiftplansync/compose"
)

// StructureServices are School Structure's group management and the
// substitutions (Terminvertretungen move Betreuungsplan staffing across the
// Workforce and Timetable line, #3418).
type StructureServices struct {
	Education    schoolstructure.GroupManagement
	Substitution education.SubstitutionModule
}

// buildGroupManagement binds School Structure's group management over the
// People Directory, School Membership and Facilities directories; its group
// events wake the open views.
func buildGroupManagement(db transactionruntime.Database, hub *realtime.Hub, structure owners.StructureRows, people owners.PeopleRows, substitutions owners.GroupSubstitutions, audit owners.AuditOwner) schoolstructure.GroupManagement {
	return education.NewGroupManagement(
		structure.Groups, structure.GroupTeachers, structure.ClassTeachers, structure.GroupRooms, structure.GroupTeacherDirectory,
		structure.StaffChecks, people.Students, substitutions, education.NewLegacyRepositoryRuntime(db.Pool()),
		education.GroupServiceOptions{Broadcaster: NewGroupEvents(hub, nil), Audit: NewEducationClassAssignmentAudit(audit.Persistence.StaffMasterDataChange)},
	)
}

// buildSubstitutions binds the substitution module; the shift-plan-sync
// workflow serves its schedule port.
func buildSubstitutions(
	db transactionruntime.Database, env owners.RootEnvironment, hub *realtime.Hub, structure owners.StructureRows, people owners.PeopleRows,
	substitutions owners.GroupSubstitutions, workTime workforce.Capability, callerContext identityaccess.CallerContext,
	sessions presenceCompose.SessionRecords, supervisions studentpresence.SupervisionRecords, active studentpresence.Presence,
	deviations timetable.StaffDeviations, owner owners.TimetableOwner, audit owners.AuditOwner, settings settingsCompose.SettingsService,
	invitations identityaccess.SchoolInvitations,
) (education.SubstitutionModule, error) {
	repos := owner.Repositories
	schedule, err := shiftplansyncCompose.NewSubstitution(shiftplansyncCompose.SubstitutionDependencies{
		Deviations: deviations, Staff: owners.NewPlanStaffDirectory(people.Staff), Broadcaster: hub,
		Logger:            env.Logger.With("service", "schedule-substitution"),
		ActivityInstances: arrivalTimetable.NewTimetableInstanceReads(repos.ActivityInstance.Reads),
		InstanceStaff:     arrivalTimetable.NewTimetableInstanceStaffReads(repos.InstanceStaff),
	})
	if err != nil {
		return nil, err
	}
	logger := env.Logger.With("service", "substitution")
	return education.NewSubstitutionModule(education.SubstitutionDependencies{
		Groups: structure.Groups, Substitutions: NewEducationHandovers(substitutions, shiftplansync.NewGroupHandovers(workTime)),
		Persons: structure.SubstitutionPersons, Teachers: structure.Caregivers,
		Staff: structure.StaffChecks, ExternalCaregivers: structure.ExternalCaregivers.WithInvitations(invitations),
		Actors:       SubstitutionActorResolver{Identity: callerContext},
		ActiveGroups: sessions, ActiveSupervisors: supervisions,
		ActiveSupervisorCreator: active,
		Audit:                   NewEducationSubstitutionAudit(audit.Persistence.SubstitutionChange),
		Runtime:                 education.NewLegacyRepositoryRuntime(db.Pool()), Broadcaster: NewGroupEvents(hub, logger),
		Logger: logger, Schedule: schedule,
		CanSeeAll: substitutionOverviewScope(settings), Now: env.Now,
	}), nil
}

// buildEmergencySnapshot binds the Notfallliste (#2704): the owner facades
// are its tenant-safe reads, the retained student and guardian repositories,
// the presence-mode read and the renderer its compatibility bindings.
func buildEmergencySnapshot(env owners.RootEnvironment, presence owners.PresenceOwner, active studentpresence.Presence, people owners.PeopleRows, persons peopledirectory.Capability, rooms facilitiesModule.Catalog, renderer *lists.RendererService) (emergencysnapshot.Query, error) {
	emergency, err := newEmergencySnapshot(emergencySnapshotSources{
		Presence: presence.Presence, PresenceMode: active, Students: people.Students, Persons: persons,
		Contacts: people.StudentGuardians, Rooms: rooms, Renderer: renderer, Now: env.Now,
		Logger: env.Logger.With("module", "emergency-snapshot"),
	})
	if err != nil {
		return nil, fmt.Errorf("compose emergency snapshot projection: %w", err)
	}
	return emergency, nil
}

// ClassDayViews are the school portal's class-day capability, the day
// report, the supervision sheet and the arrival exception write seam
// (#2701, #2970, #3563), and the slot lists (#2701).
type ClassDayViews struct {
	ClassDay  classday.ClassDay
	SlotLists classday.SlotLists
}

// buildClassDayViews binds the class day and the slot lists.
func buildClassDayViews(
	db transactionruntime.Database, env owners.RootEnvironment, hub *realtime.Hub, caller *owners.CallerRows, reports enrollmentOwner.Reports,
	carePlanRows owners.CarePlanRows, schedules owners.CarePlanSchedules, carePlan careplan.Capability, people owners.PeopleRows, audit owners.AuditOwner,
	settings settingsCompose.BatchSettingsService, operations timetable.OperationCapability, presence owners.PresenceOwner,
	persons peopledirectory.Capability, groups schoolstructure.Query, rooms facilitiesModule.Catalog, renderer *lists.RendererService,
	emitter presenceStudentsHTTP.GuardianWake,
) (ClassDayViews, error) {
	classDayReads, err := arrivalTimetable.NewPresenceReads(owners.NewTimetablePresenceProjection(db))
	if err != nil {
		return ClassDayViews{}, fmt.Errorf("compose class day timetable reads: %w", err)
	}
	classDay := NewClassDay(classdayCompose.ClassDayDependencies{
		Caller: caller, PickupTimes: schedules.Pickups, ArrivalTimes: schedules.Arrivals, CareDays: schedules.CareDay,
		Companions: owners.NewStudentCompanionRepository(carePlan), ClassArrivalExceptions: schedules.Arrivals,
		BlockStarts: operations,
		TripStatusDays: NewClassDayTripStatusDays(presenceStudentsHTTP.NewSchoolClassTrips(presenceStudentsHTTP.ResourceConfig{
			Broadcaster: hub, Logger: env.Logger.With("service", "class-day-class-trips"), ParentEventEmitter: emitter,
			PeopleDirectory: persons, StudentStatusDayService: presence.StatusDays,
		})),
		DayHints: persons, Today: calendar.CalendarDateClock(env.Now),
	}, reports, carePlanRows.StatusDays, people.Students, people.Persons, people.StudentGuardians,
		audit.Persistence.DataAccessLog, settings, hub, env.Logger.With("service", "class-day-arrival-exceptions"))
	slotLists := classdayCompose.NewSlotLists(classdayCompose.SlotListDependencies{
		Timetable: NewClassDayTimetableRows(classDayReads), Presence: presence.Presence, CarePlan: carePlan,
		Students: persons, Persons: persons, Groups: groups, Rooms: rooms, CareDays: schedules.CareDay,
		PickupTimes: schedules.Pickups, ArrivalTimes: schedules.Arrivals, PickupBaselines: schedules.PickupBaselines,
		Settings: ClassDaySlotListSettings{Settings: settings}, Access: ClassDaySlotListAccess{Staff: caller},
		ListExport: ClassDaySlotListRenderer{Renderer: renderer}, EnrolledOn: ClassDayEnrolledOn,
	})
	return ClassDayViews{ClassDay: classDay, SlotLists: slotLists}, nil
}

// buildGroupLive binds the live group view (#2253).
func buildGroupLive(
	env owners.RootEnvironment, presence owners.PresenceOwner, users peoplePorts.PersonDirectory, structure StructureServices,
	caller *owners.CallerRows, active studentpresence.Presence, settings settingsCompose.BatchSettingsService, schedules owners.CarePlanSchedules,
	lifecycle owners.TimetableLifecycle, requests owners.CarePlanRequests, hints peopledirectory.SchoolDayHintQuery,
) (grouplive.Query, error) {
	live, err := newGroupLive(groupLiveSources{
		Presence: presence.Presence, People: GroupRosterPeople(users), Education: structure.Education,
		Substitutions: structure.Substitution, UserContext: GroupLiveCaller{caller}, Active: active,
		Settings: settings, Pickups: schedules.Pickups, Arrivals: schedules.Arrivals,
		PlannedStudentIDs: lifecycle.Instances.GetPlannedStudentIDsByDate, CareDays: schedules.CareDay,
		CareParticipation: schedules.Lifecycle, ExcusedRequests: requests.Excused, StatusDays: presence.StatusDays,
		Logger: env.Logger.With("service", "ogs-group-live"), Now: env.Now,
	}, hints)
	if err != nil {
		return nil, fmt.Errorf("compose OGS group live projection: %w", err)
	}
	return live, nil
}

// buildSupervisionDashboard binds the supervision dashboard projection.
func buildSupervisionDashboard(
	env owners.RootEnvironment, settings settingsCompose.BatchSettingsService, caller *owners.CallerRows, active studentpresence.Presence,
	presence owners.PresenceOwner, rooms facilitiesModule.Catalog, timetableOwner timetable.Capability, yard FacilitiesYard,
	structure StructureServices, lifecycle owners.TimetableLifecycle, schedules owners.CarePlanSchedules, users peoplePorts.PersonDirectory,
	hints peopledirectory.SchoolDayHintQuery,
) (supervisiondashboard.Query, error) {
	supervision := SupervisionCaller{caller}
	dashboard, err := newSupervisionDashboard(supervisiondashboard.Dependencies{
		Access:   newSupervisionAccess(settings, supervision),
		Sessions: newSupervisionSessions(active, owners.OpenRoomSessionPresence{Presence: presence.Presence, Activities: timetableOwner}, supervision),
		Rooms:    OpenRoomDirectory{Rooms: rooms}, Yard: newSupervisionYard(yard.Schulhof),
		Groups:   newSupervisionGroups(structure.Education, supervision),
		Schedule: newSupervisionSchedule(lifecycle.Operations),
		Presence: newSupervisionPresence(active, active),
		Planning: newSupervisionPlanning(schedules.Pickups, schedules.Arrivals),
		Settings: newSupervisionSettings(settings), Staff: NewTimetablePeople(users), Calendar: supervisionCalendar{}, Now: env.Now,
		TeacherHints: newSupervisionTeacherHints(hints),
	})
	if err != nil {
		return nil, fmt.Errorf("compose supervision dashboard projection: %w", err)
	}
	return dashboard, nil
}

// substitutionOverviewScope asks Workforce, which owns
// education.group_substitution, who sees all of them; the Settings Platform
// answers its overview-scope port.
func substitutionOverviewScope(settings settingsCompose.SettingsService) func(ctx context.Context, assignmentBound, admin, hasStaff bool) (bool, error) {
	return func(ctx context.Context, assignmentBound, admin, hasStaff bool) (bool, error) {
		return workforce.CanSeeAllSubstitutions(ctx, func(ctx context.Context) (bool, error) {
			scope, err := settings.ResolveString(ctx, configModels.KeyOperationalOverviewScope)
			if err != nil {
				return false, fmt.Errorf("resolve operational overview scope: %w", err)
			}
			return configModels.OverviewOpenToAllStaff(scope), nil
		}, assignmentBound, admin, hasStaff)
	}
}
