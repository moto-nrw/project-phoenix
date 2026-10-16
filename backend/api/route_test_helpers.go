//go:build testhooks

package api

import (
	"log/slog"

	"github.com/go-chi/chi/v5"

	parentAPI "github.com/moto-nrw/project-phoenix/modules/careplan/inbound/parent"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	studentsAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/students"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	staffHTTP "github.com/moto-nrw/project-phoenix/modules/schoolmembership/http"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// These test-support builders share the root's adapter wiring without invoking
// its production bootstrap. Each accepts only the module under test.
func newStaffTestResource(module schoolmembership.Capability, svc StaffTestModule, db transactionruntime.Database, logger *slog.Logger) *staffHTTP.Resource {
	capabilities := NewWorkforceAdminCapabilities(svc.Users, svc.StaffAdmin, svc.TimeTracking)
	admin := newStaffAdminResource(capabilities, nil, nil, nil, db, logger)
	return newStaffResource(module, svc.PeopleDirectory, func(hooks StaffMembershipHooks) StaffMembershipRuntime {
		return svc.NewStaffMembershipRuntime(db, logger, hooks)
	}, admin, logger)
}

func newCareScheduleTestRouter(db transactionruntime.Database, module StudentTestModule) (chi.Router, error) {
	parent, err := NewParentCareScheduleTestService(db, module)
	if err != nil {
		return nil, err
	}
	students := studentsAPI.NewResource(studentsAPI.ResourceConfig{
		CareRequestService: module.CareRequests, UserContextService: module.UserContext,
		SettingsService: module.Settings, Logger: slog.Default(),
	})
	router := chi.NewRouter()
	router.Mount("/parent", parentAPI.NewResource(parentAPI.ResourceConfig{Parent: parent}).Router())
	router.Mount("/api/students", students.Router())
	return router, nil
}

func newStudentPhotoTestUnlinker() peopleCompose.PhotoUnlinker {
	return studentsAPI.NewPhotoUnlinker(slog.Default(), "public")
}

func newDatabaseStatsTestRouter(db transactionruntime.Database) (chi.Router, error) {
	read, err := NewDatabaseStatsTestReader(db)
	if err != nil {
		return nil, err
	}
	return newDatabaseStatsRouter(read, slog.Default()), nil
}
