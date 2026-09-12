package students_test

import (
	"context"
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
	"github.com/moto-nrw/project-phoenix/modules/communication/communicationtest"
	reviewidentity "github.com/moto-nrw/project-phoenix/modules/identityaccess/requestreview"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
	requestreviewcompose "github.com/moto-nrw/project-phoenix/modules/requestreview/compose"
	reviewsettings "github.com/moto-nrw/project-phoenix/modules/settings/review"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/services/listexport"
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
	parentEventEmitter := communicationtest.NewParentEventEmitter(
		db, testpkg.TenantRuntime(t, db),
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
		Unlinker:    studentsAPI.NewPhotoUnlinker(slog.Default(), "public"),
		DB:          db,
		Logger:      slog.Default(),
	})

	presence, err := presenceCompose.New(presenceCompose.Dependencies{DB: db, Observe: func(presenceCompose.Observation) {}})
	require.NoError(t, err)
	// The shared request-review projection over the same retained queues the
	// production root binds (#2705).
	reviewStudents, err := requestreviewcompose.NewStudentDirectory(db, svc.PeopleDirectory, func(requestreviewcompose.DirectoryObservation) {})
	require.NoError(t, err)
	reviewAccess, err := requestreviewcompose.NewAccess(studentsAPI.RequestReviewPrincipal, nil)
	require.NoError(t, err)
	policy, err := reviewidentity.New(reviewidentity.Dependencies{
		Principal: studentsAPI.RequestReviewPrincipal,
		GroupLeaderEnabled: func(ctx context.Context) (bool, error) {
			return svc.Settings.ResolveBool(ctx, reviewsettings.GroupLeaderEnabled)
		},
		GroupIDs: func(ctx context.Context) ([]int64, error) {
			groups, err := svc.UserContext.GetMyGroups(ctx)
			if err != nil {
				return nil, err
			}
			ids := make([]int64, 0, len(groups))
			for _, group := range groups {
				if group != nil {
					ids = append(ids, group.ID)
				}
			}
			return ids, nil
		},
	})
	require.NoError(t, err)
	clock := firstClock(clocks)
	if clock == nil {
		clock = time.Now
	}
	masterDataReviews, err := requestreviewcompose.NewMasterDataReviews(db, svc.PeopleDirectory,
		func(ctx context.Context) (requestreviewcompose.ReviewScope, error) {
			scope, err := policy.Scope(ctx)
			return requestreviewcompose.ReviewScope{SchoolWide: scope.SchoolWide, GroupIDs: scope.GroupIDs}, err
		}, func() requestreviewcompose.ReviewDate {
			return requestreviewcompose.ReviewDate(timezone.DateFromTime(clock()))
		},
		func(requestreviewcompose.CareObservation) {})
	require.NoError(t, err)
	careReviews, err := requestreviewcompose.NewScheduleReviews(db, requestreviewcompose.ScheduleReviewDependencies{
		People: svc.PeopleDirectory,
		Scope: func(ctx context.Context) (requestreviewcompose.ReviewScope, error) {
			scope, err := policy.Scope(ctx)
			return requestreviewcompose.ReviewScope{SchoolWide: scope.SchoolWide, GroupIDs: scope.GroupIDs}, err
		},
		BookingsAuthoritative: func(ctx context.Context) (bool, error) {
			return svc.Settings.ResolveBool(ctx, reviewsettings.BookingsAuthoritative)
		},
		Today: func() requestreviewcompose.ReviewDate {
			return requestreviewcompose.ReviewDate(timezone.DateFromTime(clock()))
		},
		ObserveCare: func(requestreviewcompose.CareObservation) {}, ObserveTimetable: func(requestreviewcompose.TimetableObservation) {},
	})
	require.NoError(t, err)
	careQueue, err := requestreviewcompose.NewCareScheduleQueue(careReviews, func() requestreviewcompose.ReviewDate {
		return requestreviewcompose.ReviewDate(timezone.DateFromTime(clock()))
	})
	require.NoError(t, err)
	offeringReviews, err := requestreviewcompose.NewOfferingReviews(db, requestreviewcompose.OfferingReviewDependencies{
		People: svc.PeopleDirectory,
		Scope: func(ctx context.Context) (requestreviewcompose.ReviewScope, error) {
			scope, err := policy.Scope(ctx)
			return requestreviewcompose.ReviewScope{SchoolWide: scope.SchoolWide, GroupIDs: scope.GroupIDs}, err
		},
		Today: func() requestreviewcompose.ReviewDate {
			return requestreviewcompose.ReviewDate(timezone.DateFromTime(clock()))
		},
		ObserveCare: func(requestreviewcompose.CareObservation) {}, ObserveTimetable: func(requestreviewcompose.TimetableObservation) {},
	})
	require.NoError(t, err)
	offeringQueue, err := requestreviewcompose.NewOfferingQueue(offeringReviews, func() requestreviewcompose.ReviewDate {
		return requestreviewcompose.ReviewDate(timezone.DateFromTime(clock()))
	})
	require.NoError(t, err)
	corrections, err := requestreviewcompose.NewCorrectionLog(db, svc.PeopleDirectory, func(ctx context.Context) bool {
		return studentsAPI.RequestReviewCorrectionAccess(ctx, svc.UserContext.HasCurrentStaff)
	}, func(requestreviewcompose.AuditObservation) {})
	require.NoError(t, err)
	masterQueue, err := requestreviewcompose.NewMasterDataQueue(masterDataReviews)
	require.NoError(t, err)
	excusedQueue, err := requestreviewcompose.NewExcusedQueue(svc.ExcusedRequests, func() requestreviewcompose.ReviewDate {
		return requestreviewcompose.ReviewDate(timezone.DateFromTime(clock()))
	})
	require.NoError(t, err)
	requestReview, err := requestreview.NewChecked(requestreview.Dependencies{
		Queues: requestreview.Queues{DirectCorrections: corrections, MasterData: masterQueue, CareSchedule: careQueue, Offering: offeringQueue, Excused: excusedQueue},
		Access: reviewAccess, Students: reviewStudents, FamilyProtection: requestreviewcompose.NewFamilyProtection(svc.PeopleDirectory),
		Today: func() requestreview.Date { return requestreview.Date(timezone.DateFromTime(clock())) },
	})
	require.NoError(t, err)
	resource := studentsAPI.NewResource(studentsAPI.ResourceConfig{
		PersonService:          svc.Users,
		PeopleDirectory:        svc.PeopleDirectory,
		GradeTransitionService: svc.GradeTransition,
		StudentService:         userService.NewStudentService(repoFactory.Student, repoFactory.PrivacyConsent, repoFactory.StudentCompanion, nil),
		EducationService:       svc.Education,
		UserContextService:     svc.UserContext,
		ActiveService:          svc.Active,
		IoTService:             svc.IoT,
		DeviceAuthenticator:    testutil.NewDeviceAuthenticators(svc.IoT.Fleet(), svc.Schools, nil, svc.Settings, testDevicePIN).Device(),
		PickupScheduleService:  svc.PickupSchedule,
		PartialAbsenceService:  svc.PartialAbsence,
		ArrivalScheduleService: svc.ArrivalSchedule,
		SchoolService:          svc.Schools,
		SettingsService:        svc.Settings,
		StudentHistoryService: activeSvc.NewStudentHistoryService(presence, func(ctx context.Context, ids []int64) (map[int64]string, error) {
			rooms, err := repoFactory.Room.FindByIDs(ctx, ids)
			if err != nil {
				return nil, err
			}
			names := make(map[int64]string, len(rooms))
			for _, room := range rooms {
				names[room.ID] = room.Name
			}
			return names, nil
		}, repoFactory.DataAccessLog, repoFactory.InstanceStudent),
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
		RequestReview:            requestReview,
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
