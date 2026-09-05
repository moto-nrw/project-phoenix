package api

import (
	"log/slog"

	"github.com/go-chi/chi/v5"
	parentAPI "github.com/moto-nrw/project-phoenix/api/parent"
	studentsAPI "github.com/moto-nrw/project-phoenix/api/students"
	timeTrackingAPI "github.com/moto-nrw/project-phoenix/api/time-tracking"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	staffHTTP "github.com/moto-nrw/project-phoenix/modules/schoolmembership/http"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/uptrace/bun"
)

// These test-support builders share the root's adapter wiring without invoking
// its production bootstrap. Each accepts only the module under test.
func newStaffTestResource(module schoolmembership.Capability, svc services.StaffTestModule, db *bun.DB, logger *slog.Logger) *staffHTTP.Resource {
	admin := timeTrackingAPI.NewStaffAdminResource(svc.Users, svc.StaffDocuments, svc.WorkSession, svc.StaffAbsence, svc.WorkTimeMonth, svc.StaffBalanceAdjust, svc.StaffMonthClose, svc.StaffOverview, svc.TimeTrackingAuditLog, svc.StaffTimeExport, db, logger)
	return newStaffResource(module, func(hooks services.StaffMembershipHooks) services.StaffMembershipRuntime {
		return svc.NewStaffMembershipRuntime(db, logger, hooks)
	}, admin, db, logger)
}

func newCareScheduleTestRouter(db *bun.DB, module services.StudentTestModule) (chi.Router, error) {
	parent, err := services.NewParentCareScheduleTestService(db, module)
	if err != nil {
		return nil, err
	}
	students := studentsAPI.NewResource(studentsAPI.ResourceConfig{
		CareRequestService: module.CareRequests, UserContextService: module.UserContext,
		SettingsService: module.Settings, DB: db, Logger: slog.Default(),
	})
	router := chi.NewRouter()
	router.Mount("/parent", parentAPI.NewResource(nil, parent, nil, nil, nil, db).Router())
	router.Mount("/api/students", students.Router())
	return router, nil
}

func newStudentPhotoTestBootstrap() services.StudentPhotoBootstrap {
	return services.StudentPhotoBootstrap{Unlinker: studentsAPI.NewPhotoUnlinker(slog.Default(), "public")}
}

func newDatabaseStatsTestRouter(db *bun.DB) (chi.Router, error) {
	read, err := services.NewDatabaseStatsTestReader(db)
	if err != nil {
		return nil, err
	}
	return newDatabaseStatsRouter(db, read, slog.Default()), nil
}
