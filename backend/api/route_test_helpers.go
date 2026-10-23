//go:build testhooks

package api

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/go-chi/chi/v5"

	careStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/careplan/http"
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	staffHTTP "github.com/moto-nrw/project-phoenix/modules/schoolmembership/http"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	parentAPI "github.com/moto-nrw/project-phoenix/workflows/parentportal/http"
)

// These test-support builders share the root's adapter wiring without invoking
// its production bootstrap. Each accepts only the module under test.
func newStaffTestResource(module schoolmembership.Capability, svc StaffTestModule, work WorkforceTestModule, identity identityaccess.Owner, appEnv string, db transactionruntime.Database, logger *slog.Logger) *staffHTTP.Resource {
	capabilities := owners.NewWorkforceAdminCapabilities(svc.Users, work.StaffAdmin, work.TimeTracking)
	admin := newStaffAdminResource(capabilities, nil, nil, nil, db, logger)
	resource, err := newStaffResource(module, svc.PeopleDirectory, identity, appEnv, func(hooks StaffMembershipHooks) StaffMembershipRuntime {
		return svc.NewStaffMembershipRuntime(db, logger, hooks)
	}, admin, logger)
	if err != nil {
		panic(err)
	}
	return resource
}

func newCareScheduleTestRouter(db transactionruntime.Database, module StudentTestModule, live ActiveTestModule, liveSettings ActiveSettings, liveCaller *owners.CallerRows) (chi.Router, error) {
	parent, err := NewParentCareScheduleTestService(db, module, live, liveSettings)
	if err != nil {
		return nil, err
	}
	students := NewCareHTTPUnitResource(careStudentsHTTP.ResourceConfig{
		CareRequestService: module.CareRequests, UserContextService: liveCaller,
		SettingsService: liveSettings.Settings, Logger: slog.Default(),
	})
	router := chi.NewRouter()
	router.Mount("/parent", parentAPI.NewResource(parentAPI.ResourceConfig{Parent: parent}).Router(ParentSessionChain))
	router.Mount("/api/students", students.Router(ProtectedTenantRoutes))
	return router, nil
}

func newStudentPhotoTestUnlinker() peoplePorts.PhotoUnlinker {
	return peopleModule.NewPhotoUnlinker(slog.Default(), "public", nativeUploads.UploadFiles{})
}

func newDatabaseStatsTestRouter(db transactionruntime.Database) (chi.Router, error) {
	read, err := NewDatabaseStatsTestReader(db)
	if err != nil {
		return nil, err
	}
	return newDatabaseStatsRouter(read, slog.Default()), nil
}
