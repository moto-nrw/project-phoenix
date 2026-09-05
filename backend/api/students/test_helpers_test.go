package students_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	activeSvc "github.com/moto-nrw/project-phoenix/services/active"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	studentsAPI "github.com/moto-nrw/project-phoenix/api/students"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test dependencies.
type testContext struct {
	db          *bun.DB
	resource    *studentsAPI.Resource
	broadcaster *testpkg.RecordingBroadcaster
}

func newStudentTestRepositories(db *bun.DB) repositories.StudentTestRepositories {
	repos, err := repositories.NewStudentTestRepositories(db, repositories.NewTestAuditStore(db))
	if err != nil {
		panic(err)
	}
	return repos
}

// setupStudentsRoute initializes the production students resource.
func setupStudentsRoute(t *testing.T, clocks ...func() time.Time) *testContext {
	t.Helper()

	db, svc := testutil.SetupStudentModule(t, clocks...)
	repoFactory, err := repositories.NewStudentTestRepositories(db, svc.Audit)
	require.NoError(t, err)
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
	testpkg.SetTenantRuntime(t, parentEventEmitter, db)

	studentPhotos := userService.NewStudentPhotoService(userService.StudentPhotoServiceDependencies{
		StudentRepo: repoFactory.Student,
		Settings:    svc.Settings,
		UserContext: svc.UserContext,
		Broadcaster: broadcaster,
		Unlinker:    studentsAPI.NewPhotoUnlinker(slog.Default(), "public"),
		DB:          db,
		Logger:      slog.Default(),
	})

	resource := studentsAPI.NewResource(studentsAPI.ResourceConfig{
		PersonService:           svc.Users,
		PeopleDirectory:         svc.PeopleDirectory,
		GradeTransitionService:  svc.GradeTransition,
		StudentService:          userService.NewStudentService(repoFactory.Student, repoFactory.PrivacyConsent, repoFactory.StudentCompanion, nil),
		EducationService:        svc.Education,
		UserContextService:      svc.UserContext,
		ActiveService:           svc.Active,
		IoTService:              svc.IoT,
		DevicePINFallback:       testDevicePIN,
		PickupScheduleService:   svc.PickupSchedule,
		PartialAbsenceService:   svc.PartialAbsence,
		ArrivalScheduleService:  svc.ArrivalSchedule,
		SchoolService:           svc.Schools,
		SettingsService:         svc.Settings,
		StudentHistoryService:   activeSvc.NewStudentHistoryService(repoFactory.Attendance, repoFactory.ActiveVisit, repoFactory.DataAccessLog, repoFactory.InstanceStudent),
		OGSGroupLiveService:     svc.OGSGroupLive,
		InstanceService:         svc.Instance,
		CareDayService:          svc.CareDay,
		CareLifecycleService:    svc.CareLifecycle,
		StudentStatusDayService: activeSvc.NewStudentStatusDayServiceWithPartialAbsences(repoFactory.StudentStatusDay, repoFactory.StudentPickupException, db),
		AbsenceOverview:         activeSvc.NewStudentStatusDayOverviewService(repoFactory.StudentStatusDay, svc.Users),
		ExcusedRequestService:   svc.ExcusedRequests,
		StudentAuditService:     svc.StudentAudit,
		EnrollmentDecision:      svc.EnrollmentDecision,
		// The three users:update-gated review queues, wired so the combined
		// pending-count endpoint can be exercised end to end (#2232).
		MasterDataReviewService:  svc.MasterDataReview,
		CareRequestService:       svc.CareRequests,
		OfferingChangeService:    svc.OfferingChanges,
		PickupAdjustmentService:  svc.PickupAdjustments,
		ParentRequestBulkService: svc.ParentRequests,
		FamilyProtectionService:  svc.FamilyProtection,
		Broadcaster:              broadcaster,
		ParentEventEmitter:       parentEventEmitter,
		StudentPhotos:            studentPhotos,
		StudentConsents:          userService.NewStudentConsentService(repoFactory.StudentConsentChange),
		ListExportService:        listexport.NewService(),
		Logger:                   slog.Default(),
		Now:                      firstClock(clocks),
		DB:                       db,
	})

	return &testContext{
		db:          db,
		resource:    resource,
		broadcaster: broadcaster,
	}
}

func firstClock(clocks []func() time.Time) func() time.Time {
	if len(clocks) == 0 {
		return nil
	}
	return clocks[0]
}

const studentsTestToday timezone.Date = "2026-08-24"

func fixedCalendarClock() time.Time {
	return studentsTestToday.BerlinMidnight().Add(12 * time.Hour)
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
	return testutil.ExecuteRequestForTest(t, tc.resource.Router(), req)
}
