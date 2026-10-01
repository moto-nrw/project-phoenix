package students_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/communication/communicationtest"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	reviewidentity "github.com/moto-nrw/project-phoenix/modules/identityaccess/requestreview"
	studentsAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/students"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
	requestreviewcompose "github.com/moto-nrw/project-phoenix/modules/requestreview/compose"
	reviewsettings "github.com/moto-nrw/project-phoenix/modules/settings/review"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	studentdeletioncompose "github.com/moto-nrw/project-phoenix/workflows/studentdeletion/compose"
)

// testContext holds shared test dependencies.
type testContext struct {
	careRequests carerequests.Submissions
	db           *bun.DB
	resource     *studentsAPI.Resource
	// settingsService is the Settings Platform service the resource was
	// built with; settings() hands it to fixtures that write overrides.
	settingsService studentsAPI.TenantSettings
	broadcaster     *testpkg.RecordingBroadcaster
	// clock is the fixed clock the services run on; nil means the real one.
	clock func() time.Time
	// newPickupAdjustments rebinds the pickup adjustment to other offering
	// adjustments.
	newPickupAdjustments func(careplan.DirectOfferingAdjustments) (careplan.PickupAdjustments, error)
}

// newStudentTestRepositories composes the retained repositories the suites
// arrange through, recording into the test audit store.
var newStudentTestRepositories = testutil.MustNewStudentRouteRows

// settingsWriter is the override half of the Settings Platform service the
// fixtures write through; the resource itself only reads.
type settingsWriter interface {
	SetValue(ctx context.Context, key string, value any, changedBy *int64, userPermissions []string) error
	ResetValue(ctx context.Context, key string, changedBy *int64, userPermissions []string) error
}

// settings returns the Settings Platform service the resource was built
// with, for fixtures that write tenant overrides.
func (tc *testContext) settings() settingsWriter {
	return tc.settingsService.(settingsWriter)
}

// setupStudentsRoute initializes the production students resource.
func setupStudentsRoute(t *testing.T, clocks ...func() time.Time) *testContext {
	t.Helper()
	return newStudentsRoute(t, nil, clocks...)
}

// setupStudentsRouteWithCareLifecycle initializes the resource over a native
// Care Plan lifecycle that records its care-end history and reads the booking
// authority from authoritative.
func setupStudentsRouteWithCareLifecycle(t *testing.T, authoritative bool, clocks ...func() time.Time) *testContext {
	t.Helper()
	return newStudentsRoute(t, &authoritative, clocks...)
}

func newStudentsRoute(t *testing.T, careLifecycleAuthoritative *bool, clocks ...func() time.Time) *testContext {
	t.Helper()

	db, svc := testutil.SetupStudentModule(t, clocks...)
	repoFactory, err := testutil.NewStudentRouteRows(db, svc.Audit)
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

	studentPhotos := svc.NewStudentPhotos(broadcaster, studentsAPI.NewPhotoUnlinker(slog.Default(), "public"))

	education := svc.Education
	schoolGroups := fixtureSchoolGroups{
		get: func(ctx context.Context, id int64) (*studentsAPI.SchoolGroup, error) {
			group, err := education.GetGroup(ctx, id)
			if err != nil {
				return nil, err
			}
			roomName := ""
			if group.Room != nil {
				roomName = group.Room.Name
			}
			return schoolGroupView(group.ID, group.Name, group.RoomID, roomName), nil
		},
		byIDs: func(ctx context.Context, ids []int64) (map[int64]*studentsAPI.SchoolGroup, error) {
			groups, err := education.GetGroupsByIDs(ctx, ids)
			if err != nil {
				return nil, err
			}
			views := make(map[int64]*studentsAPI.SchoolGroup, len(groups))
			for id, group := range groups {
				roomName := ""
				if group.Room != nil {
					roomName = group.Room.Name
				}
				views[id] = schoolGroupView(group.ID, group.Name, group.RoomID, roomName)
			}
			return views, nil
		},
		list: func(ctx context.Context) ([]*studentsAPI.SchoolGroup, error) {
			groups, err := education.ListGroups(ctx, nil)
			if err != nil {
				return nil, err
			}
			views := make([]*studentsAPI.SchoolGroup, 0, len(groups))
			for _, group := range groups {
				roomName := ""
				if group.Room != nil {
					roomName = group.Room.Name
				}
				views = append(views, schoolGroupView(group.ID, group.Name, group.RoomID, roomName))
			}
			return views, nil
		},
		teachers: func(ctx context.Context, groupID int64) ([]studentsAPI.GroupTeacher, error) {
			teachers, err := education.GetGroupTeachers(ctx, groupID)
			if err != nil {
				return nil, err
			}
			views := make([]studentsAPI.GroupTeacher, 0, len(teachers))
			for _, teacher := range teachers {
				if teacher == nil || teacher.Staff == nil || teacher.Staff.Person == nil {
					continue
				}
				view := studentsAPI.GroupTeacher{ID: teacher.ID, FirstName: teacher.Staff.Person.FirstName, LastName: teacher.Staff.Person.LastName}
				if teacher.Staff.Person.Account != nil {
					view.Email = teacher.Staff.Person.Account.Email
				}
				views = append(views, view)
			}
			return views, nil
		},
	}

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
		Blocks: testutil.NewStudentRoutePickupReviewBlocks(db),
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
	// The permanent-deletion routes run the owner workflow (#2710) over the
	// same capabilities the production root binds.
	studentDeletion, err := studentdeletioncompose.New(studentdeletioncompose.Dependencies{
		DB: db, Directory: svc.PeopleDirectory, CarePlan: repoFactory.CarePlan, Timetable: repoFactory.Timetable,
		Feedback: &testpkg.FeedbackEntryCounterMock{}, IsVerifiedStaff: svc.UserContext.HasCurrentStaff,
		LockCareBookingWrites: mustTimetableRecurrenceLock(t, db).LockRecurrenceWrites,
		UnlinkPhoto:           studentPhotos.ScheduleUnlinkAfterCommit, Broadcaster: broadcaster, Audit: svc.Audit,
		Now: clock,
	})
	require.NoError(t, err)
	resource := studentsAPI.NewResource(studentsAPI.ResourceConfig{
		PeopleDirectory:        svc.PeopleDirectory,
		Persons:                svc.StudentRoutePersons(),
		StudentDeletion:        studentDeletion,
		CompanionService:       testutil.NewStudentRouteCompanions(repoFactory, svc.PeopleDirectory, svc.StudentAudit),
		SchoolGroups:           schoolGroups,
		UserContextService:     svc.UserContext,
		ActiveService:          svc.Active,
		DeviceAuthenticator:    testutil.NewDeviceAuthenticators(svc.IoT.Fleet(), testutil.DeviceSchools(t, db), svc.Settings, testDevicePIN).Device(),
		AuthenticatedDevice:    testutil.AuthenticatedDeviceID,
		PickupScheduleService:  svc.PickupSchedule,
		WeekdayPickupNotes:     repoFactory.CarePlan,
		PartialAbsenceService:  svc.PartialAbsence,
		ArrivalScheduleService: svc.ArrivalSchedule,
		SchoolService:          exportSchools{schools: svc.Schools},
		SettingsService:        svc.Settings,
		StudentHistoryService: presenceservice.NewStudentHistory(presence, func(ctx context.Context, ids []int64) (map[int64]string, error) {
			rooms, err := repoFactory.Room.FindByIDs(ctx, ids)
			if err != nil {
				return nil, err
			}
			names := make(map[int64]string, len(rooms))
			for _, room := range rooms {
				names[room.ID] = room.Name
			}
			return names, nil
		}, svc.DataAccessAudit(), svc.HistorySlots(repoFactory.InstanceStudent)),
		OGSGroupLiveService:     svc.OGSGroupLive,
		InstanceService:         svc.Instance,
		CareDayService:          svc.CareDay,
		CareLifecycleService:    careLifecycleFor(t, db, svc.CareLifecycle, careLifecycleAuthoritative, clocks),
		StudentStatusDayService: presenceservice.NewStatusDays(repoFactory.StudentStatusDay, svc.ManualPartialAbsences(repoFactory.CarePlan), db, repoFactory.CarePlan.LockExceptionDay),
		AbsenceOverview:         presenceservice.NewStatusDayOverviews(repoFactory.StudentStatusDay, svc.StatusDayOverviewPeople()),
		ExcusedRequestService:   svc.ExcusedRequests,
		StudentAuditService:     svc.PeopleDirectory,
		PrivacyConsents:         presence,
		EnrollmentDecision:      svc.EnrollmentDecision,
		// The three users:update-gated review queues, wired so the combined
		// pending-count endpoint can be exercised end to end (#2232).
		MasterDataReviewService:  svc.MasterDataReview,
		CareRequestService:       svc.CareRequests,
		CareRequestReviews:       careReviews,
		OfferingChangeService:    svc.OfferingChanges,
		PickupAdjustmentService:  svc.PickupAdjustments,
		ParentRequestBulkService: svc.ParentRequests,
		FamilyProtection:         svc.PeopleDirectory,
		ChildQuota:               newChildQuotaReader(t, db),
		RequestReview:            requestReview,
		Broadcaster:              broadcaster,
		ParentEventEmitter:       parentEventEmitter,
		StudentPhotos:            svc.PeopleDirectory,
		StudentConsents:          testutil.NewStudentRouteConsents(db),
		ListExportService:        lists.NewRenderer(),
		Logger:                   slog.Default(),
		Now:                      firstClock(clocks),
	})

	return &testContext{
		careRequests:         svc.CareRequests,
		db:                   db,
		resource:             resource,
		settingsService:      svc.Settings,
		broadcaster:          broadcaster,
		clock:                firstClock(clocks),
		newPickupAdjustments: svc.NewPickupAdjustments,
	}
}

// previewStudentDeletion reads the delete-impact preview the confirmed
// deletion has to quote back (#2710): the permanent deletion is never
// reachable without it.
func previewStudentDeletion(t *testing.T, tc *testContext, claims jwt.AppClaims, studentID int64) map[string]any {
	t.Helper()
	request := testutil.NewAuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/%d/delete-impact", studentID), nil)
	response := authExec(t, tc, request, claims, []string{"admin:*"})
	require.Equal(t, http.StatusOK, response.Code, "Body: %s", response.Body.String())
	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	return body.Data
}

// confirmStudentDeletion sends the confirmed DELETE for a preview.
func confirmStudentDeletion(t *testing.T, tc *testContext, claims jwt.AppClaims, studentID int64, preview map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	request := testutil.NewAuthenticatedRequest(t, http.MethodDelete, fmt.Sprintf("/%d", studentID), map[string]any{
		"expected_fingerprint": preview["fingerprint"],
		"confirmation_name":    preview["confirmation_name"],
		"reason":               "test_data",
		"acknowledged":         true,
	})
	return authExec(t, tc, request, claims, []string{"admin:*"})
}

// deleteStudentConfirmed previews and confirms in one step for tests whose
// subject is the deletion's side effect, not the confirmation itself.
func deleteStudentConfirmed(t *testing.T, tc *testContext, claims jwt.AppClaims, studentID int64) *httptest.ResponseRecorder {
	t.Helper()
	return confirmStudentDeletion(t, tc, claims, studentID, previewStudentDeletion(t, tc, claims, studentID))
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

// exportSchools reads the export title through the Organisation & Tenancy
// capability, as the serving root does.
type exportSchools struct{ schools organizationtenancy.Query }

func (s exportSchools) GetSchoolByID(ctx context.Context, id int64) (*studentsAPI.ExportSchool, error) {
	school, err := s.schools.FindSchool(ctx, id)
	if errors.Is(err, organizationtenancy.ErrSchoolNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &studentsAPI.ExportSchool{Name: school.Name}, nil
}

// mustTimetableRecurrenceLock is the Timetable owner's tenant recurrence gate
// over the test database.
func mustTimetableRecurrenceLock(t *testing.T, db *bun.DB) interface {
	LockRecurrenceWrites(context.Context) error
} {
	t.Helper()
	lock, err := testutil.NewTimetableRecurrenceLock(db)
	require.NoError(t, err)
	return lock
}

// readStudentRecord reads the child's owned row with its departure plan in
// the effective form the routes hydrate it to.
func readStudentRecord(t *testing.T, tc *testContext, studentID int64) (peopleModule.StudentRecord, error) {
	t.Helper()
	record, err := tc.resource.PeopleDirectory.FindStudentRecord(testpkg.Ctx(t), studentID)
	if err != nil {
		return record, err
	}
	effective := departure.Plan{
		AllowedDepartureModes: record.AllowedDepartureModes,
		DepartureDays:         record.DepartureDays,
		BusDays:               record.BusDays,
		PickupDays:            record.PickupDays,
	}.Effective()
	record.AllowedDepartureModes = effective.AllowedDepartureModes
	record.DepartureDays = effective.DepartureDays
	record.BusDays = effective.BusDays
	record.PickupDays = effective.PickupDays
	return record, nil
}

// careLifecycleFor keeps the module's lifecycle, or composes the native one
// over the booking authority a suite asks for.
func careLifecycleFor(t *testing.T, db *bun.DB, module careplan.CareLifecycle, authoritative *bool, clocks []func() time.Time) careplan.CareLifecycle {
	t.Helper()
	if authoritative == nil {
		return module
	}
	now := firstClock(clocks)
	if now == nil {
		now = time.Now
	}
	bookings := *authoritative
	lifecycle, err := testutil.NewStudentRouteCareLifecycle(db, testutil.StudentRouteCareLifecycleConfig{
		AuditActor: testpkg.RequestAuditActor,
		BookingsAuthoritative: func(context.Context) (bool, error) {
			return bookings, nil
		},
		Today: func() timezone.Date { return timezone.DateFromTime(now()) },
	})
	require.NoError(t, err)
	return lifecycle
}
