package students_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	activeSvc "github.com/moto-nrw/project-phoenix/services/active"

	"github.com/uptrace/bun"

	studentsAPI "github.com/moto-nrw/project-phoenix/api/students"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test dependencies.
type testContext struct {
	db          *bun.DB
	services    *services.Factory
	resource    *studentsAPI.Resource
	broadcaster *testpkg.RecordingBroadcaster
}

// setupTestContext initializes the test environment.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)
	repoFactory := repositories.NewFactory(db)
	broadcaster := testpkg.NewRecordingBroadcaster()

	// Real emitter wired to the recording broadcaster so the staff-side guardian
	// wake (parent_child_updated fan-out after a care write, #1725) is exercised
	// and assertable via broadcaster.CallsByMethod("guardian"). Message-independent:
	// it reads the guardian list and broadcasts regardless of the messaging setting.
	parentEventEmitter := parentmessaging.NewEmitter(
		db,
		repoFactory.ParentMessageThread,
		repoFactory.ParentMessage,
		svc.Settings,
		broadcaster,
		slog.Default(),
	)

	studentPhotos := userService.NewStudentPhotoService(userService.StudentPhotoServiceDependencies{
		StudentRepo: repoFactory.Student,
		Settings:    svc.Settings,
		UserContext: svc.UserContext,
		Broadcaster: broadcaster,
		Unlinker:    studentsAPI.NewPhotoUnlinker(slog.Default()),
		DB:          db,
		Logger:      slog.Default(),
	})

	resource := studentsAPI.NewResource(studentsAPI.ResourceConfig{
		PersonService:           svc.Users,
		GuardianService:         svc.Guardian,
		StudentService:          userService.NewStudentService(repoFactory.Student, repoFactory.PrivacyConsent, repoFactory.StudentCompanion, nil),
		EducationService:        svc.Education,
		UserContextService:      svc.UserContext,
		ActiveService:           svc.Active,
		IoTService:              svc.IoT,
		PickupScheduleService:   svc.PickupSchedule,
		PartialAbsenceService:   svc.PartialAbsence,
		ArrivalScheduleService:  svc.ArrivalSchedule,
		SchoolService:           svc.Schools,
		SettingsService:         svc.Settings,
		StudentHistoryService:   activeSvc.NewStudentHistoryService(repoFactory.Attendance, repoFactory.ActiveVisit, repoFactory.DataAccessLog, repoFactory.InstanceStudent),
		OGSGroupLiveService:     svc.OGSGroupLive,
		InstanceService:         svc.Instance,
		CareDayService:          svc.CareDay,
		StudentStatusDayService: activeSvc.NewStudentStatusDayServiceWithPartialAbsences(repoFactory.StudentStatusDay, repoFactory.StudentPickupException, db),
		AbsenceOverview:         activeSvc.NewStudentStatusDayOverviewService(repoFactory.StudentStatusDay, svc.Users),
		ExcusedRequestService:   svc.ExcusedRequests,
		// The three users:update-gated review queues, wired so the combined
		// pending-count endpoint can be exercised end to end (#2232).
		MasterDataReviewService: svc.MasterDataReview,
		CareRequestService:      svc.CareRequests,
		OfferingChangeService:   svc.OfferingChanges,
		Broadcaster:             broadcaster,
		ParentEventEmitter:      parentEventEmitter,
		StudentPhotos:           studentPhotos,
		ListExportService:       listexport.NewService(),
		Logger:                  slog.Default(),
		DB:                      db,
	})

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	})

	return &testContext{
		db:          db,
		services:    svc,
		resource:    resource,
		broadcaster: broadcaster,
	}
}

// authExec signs a JWT carrying claims (narrowed to perms) and runs the request
// through the production Router() so the full middleware chain executes exactly
// as the real server does (Verifier → Authenticator → TenantMiddleware →
// RequiresPermission → TenantTxMiddleware). It replaces the old
// setupRouter+executeWithAuth context-injection pattern: paths must be the real
// route paths and perms must be the permission the endpoint actually gates on.
func authExec(t *testing.T, tc *testContext, req *http.Request, claims jwt.AppClaims, perms []string) *httptest.ResponseRecorder {
	t.Helper()
	claims.Permissions = perms
	req.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, claims))
	return testutil.ExecuteRequest(tc.resource.Router(), req)
}
