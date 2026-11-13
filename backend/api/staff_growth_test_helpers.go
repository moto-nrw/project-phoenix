//go:build testhooks

package api

import (
	"log/slog"
	"net/http"

	transactionruntime "github.com/moto-ogs/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-ogs/project-phoenix/tenant"

	"github.com/go-chi/chi/v5"
	facilitiesCompose "github.com/moto-ogs/project-phoenix/modules/facilities/compose"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	timetableCompose "github.com/moto-ogs/project-phoenix/modules/timetable/compose"
	workforceCompose "github.com/moto-ogs/project-phoenix/modules/workforce/compose"
	"github.com/moto-ogs/project-phoenix/roots/owners"
)

// StaffGrowthTestRouter reuses the fully composed directory resource, including
// its real workforce enrichment and permission/tenant middleware.
func StaffGrowthTestRouter(database transactionruntime.Database, unit tenant.UnitOfWork) (http.Handler, error) {
	svc, work, err := NewStaffTestModule(database, unit)
	if err != nil {
		return nil, err
	}
	membership, err := owners.NewSchoolMembership(database)
	if err != nil {
		return nil, err
	}
	identity, err := NewAuthTestModule(database, unit)
	if err != nil {
		return nil, err
	}
	schedules := owners.MustNewWorkforce(database)
	capabilities := owners.NewWorkforceAdminCapabilities(svc.Users, work.StaffAdmin, work.TimeTracking)
	admin := newStaffAdminResource(capabilities, work.SickReports, schedules, nil, nil, work.PayrollDispatch, work.PayrollRules, svc.Users.StaffRows(), database, slog.Default())
	resource, err := newStaffResource(membership, svc.PeopleDirectory, identity.Auth, "", func(hooks StaffMembershipHooks) StaffMembershipRuntime {
		return svc.NewStaffMembershipRuntime(database, slog.Default(), hooks)
	}, admin, slog.Default())
	if err != nil {
		return nil, err
	}
	router := chi.NewRouter()
	router.Mount("/staff", resource.Router())
	return router, nil
}

// StaffShiftGrowthTestRouter binds the real planning composition to its route.
func StaffShiftGrowthTestRouter(database transactionruntime.Database) (http.Handler, error) {
	timetable, err := NewTimetableTestRepositories(database)
	if err != nil {
		return nil, err
	}
	rooms, err := owners.NewTimetableRoomRows(database)
	if err != nil {
		return nil, err
	}
	calendar, err := NewTimetableSchoolCalendar(database)
	if err != nil {
		return nil, err
	}
	planning, err := workforceCompose.NewShiftPlanning(workforceCompose.ShiftPlanningDependencies{
		Workforce: NewAbsenceTypeTestCapability(database), Staff: owners.NewPlanStaffDirectory(owners.MustNewMembershipTestRepositories(database).Staff), CalendarPeriods: calendar.SchoolCalendar(),
		Instances: timetableCompose.NewTimetableInstanceReads(timetable.ActivityInstance.Reads), InstanceStaff: timetableCompose.NewTimetableInstanceStaffReads(timetable.InstanceStaff),
		Rooms: facilitiesCompose.NewRoomNameDirectory(rooms.Room), ActivityGroups: timetableCompose.NewTimetableGroupReads(timetable.Timetable), DB: database.Pool(),
	})
	if err != nil {
		return nil, err
	}
	caller, _, err := newStaffIdentityForTests(database)
	if err != nil {
		return nil, err
	}
	resource := newStaffShiftsResource(staffShiftsDependencies{Planning: planning.Planning(nil), ResolveStaffID: currentStaffID(caller), ActorAccountID: sessiontoken.ActorAccountIDFromCtx})
	router := chi.NewRouter()
	router.Mount("/staff-shifts", resource.Router())
	return router, nil
}
