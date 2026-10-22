package enrollmenthttp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type offeringLinkFixture string

const (
	noOffering         offeringLinkFixture = "none"
	effectiveOffering  offeringLinkFixture = "effective"
	expiredOffering    offeringLinkFixture = "expired"
	futureOffering     offeringLinkFixture = "future"
	historicalOffering offeringLinkFixture = "historical"
	requiredOnly       offeringLinkFixture = "required_only"
	twoOfferings       offeringLinkFixture = "two_offerings"
	crossPhaseOffering offeringLinkFixture = "cross_phase"
	// Two variants of a per-day group (#4514), on a shared or on separate days.
	perDayShared   offeringLinkFixture = "per_day_shared"
	perDaySeparate offeringLinkFixture = "per_day_separate"
)

type offeringGuardHarness struct {
	router             http.Handler
	requestID, childID int64
	token              string
	repos              api.EnrollmentTestRepositories
	ctx                context.Context
}

type failingAtDateOfferingReader struct {
	TestDecisionChildren
}

func (f failingAtDateOfferingReader) RequestChildOfferingsAtDate(context.Context, int64, enrollmentNative.Date) ([]*enrollmentNative.RequestChildOffering, error) {
	return nil, errors.New("offering lookup failed")
}

func TestDecideAdminChild_ApprovalOfferingGuard(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, mode       string
		link             offeringLinkFixture
		offeringsEnabled bool
		wantStatus       int
		wantCode         string
	}{
		{"at least one missing", enrollmentNative.PhaseCareOfferingSelectionAtLeastOne, noOffering, true, http.StatusConflict, "enrollment.approval_care_offering_missing"},
		{"at least one required only", enrollmentNative.PhaseCareOfferingSelectionAtLeastOne, requiredOnly, true, http.StatusConflict, "enrollment.approval_care_offering_missing"},
		{"exactly one missing", enrollmentNative.PhaseCareOfferingSelectionExactlyOne, noOffering, true, http.StatusConflict, "enrollment.approval_care_offering_exactly_one"},
		{"exactly one required only", enrollmentNative.PhaseCareOfferingSelectionExactlyOne, requiredOnly, true, http.StatusConflict, "enrollment.approval_care_offering_exactly_one"},
		{"exactly one multiple", enrollmentNative.PhaseCareOfferingSelectionExactlyOne, twoOfferings, true, http.StatusConflict, "enrollment.approval_care_offering_exactly_one"},
		{"required while feature disabled", enrollmentNative.PhaseCareOfferingSelectionAtLeastOne, noOffering, false, http.StatusOK, ""},
		{"expired link", enrollmentNative.PhaseCareOfferingSelectionAtLeastOne, expiredOffering, true, http.StatusConflict, "enrollment.approval_care_offering_missing"},
		{"future link", enrollmentNative.PhaseCareOfferingSelectionAtLeastOne, futureOffering, true, http.StatusConflict, "enrollment.approval_care_offering_missing"},
		{"historical link active at service start", enrollmentNative.PhaseCareOfferingSelectionAtLeastOne, historicalOffering, true, http.StatusOK, ""},
		{"cross-phase link", enrollmentNative.PhaseCareOfferingSelectionAtLeastOne, crossPhaseOffering, true, http.StatusConflict, "enrollment.approval_care_offering_missing"},
		{"optional missing", enrollmentNative.PhaseCareOfferingSelectionOptional, noOffering, true, http.StatusOK, ""},
		{"required effective", enrollmentNative.PhaseCareOfferingSelectionAtLeastOne, effectiveOffering, true, http.StatusOK, ""},
		{"per-day variants share a day", enrollmentNative.PhaseCareOfferingSelectionOptional, perDayShared, true, http.StatusConflict, "enrollment.approval_care_offering_day_conflict"},
		{"per-day variants on separate days", enrollmentNative.PhaseCareOfferingSelectionAtLeastOne, perDaySeparate, true, http.StatusOK, ""},
		{"per-day conflict while feature disabled", enrollmentNative.PhaseCareOfferingSelectionOptional, perDayShared, false, http.StatusOK, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			harness := setupOfferingGuardRouterTest(t, tt.mode, tt.link, tt.offeringsEnabled, false)
			rec := executeApprovalDecision(t, harness)
			require.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantStatus == http.StatusConflict {
				assertDecisionErrorCode(t, rec, tt.wantCode)
			} else {
				assert.Contains(t, rec.Body.String(), `"status":"approved"`)
			}
		})
	}
}

func TestDecideAdminChild_OfferingLookupFailureDoesNotApprove(t *testing.T) {
	t.Parallel()
	harness := setupOfferingGuardRouterTest(t, enrollmentNative.PhaseCareOfferingSelectionAtLeastOne, effectiveOffering, true, true)
	rec := executeApprovalDecision(t, harness)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	child, err := testpkg.TenantQuery1(t, harness.ctx, testpkg.SetupTestDB(t), harness.repos.Enrollment().ChildByID, harness.childID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentNative.ChildStatusSubmitted, child.Status)
}

func setupOfferingGuardRouterTest(
	t *testing.T,
	selectionMode string,
	linkFixture offeringLinkFixture,
	offeringsEnabled bool,
	failOfferingLookup bool,
) offeringGuardHarness {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repos, repoErr := api.NewEnrollmentTestRepositories(testpkg.NewDatabase(t, db), owners.NewTestAuditStore(testpkg.NewDatabase(t, db)))
	reposCare := api.NewEnrollmentCarePlanRows(testpkg.NewDatabase(t, db))
	reposTimetable := api.NewEnrollmentTimetableRows(testpkg.NewDatabase(t, db))
	require.NoError(t, repoErr)
	_, reviewer := testpkg.CreateTestStaffWithAccount(t, db, "Rita", "Pruefung")
	phase := createOfferingGuardPhase(t, repos, ctx, selectionMode)
	request, child := createOfferingGuardChild(t, repos, ctx, phase.ID)
	attachOfferingGuardLink(t, repos, reposCare, ctx, phase, child.ID, linkFixture)
	children := TestDecisionChildren(repos.Enrollment())
	if failOfferingLookup {
		children = failingAtDateOfferingReader{children}
	}
	guardianAccess, err := identityaccessCompose.New(identityaccessCompose.Dependencies{DB: db, Observe: func(identityaccessCompose.Observation) {}})
	require.NoError(t, err)
	studentEnrollment, err := owners.NewPeopleDirectory(testpkg.MustDatabase(db))
	require.NoError(t, err)
	bookings := newBookingMaterialization(t, db,
		api.WithBookingCatalog(newCareOfferingCatalog(t, db).Catalog),
		api.WithBookingSettings(offeringGuardSettings(offeringsEnabled)),
	).Bookings
	decision := newOfferingGuardDecisionService(repos, reposCare, reposTimetable, offeringsEnabled, children, guardianAccess, studentEnrollment, bookings)
	resource := enrollmentAPI.NewResource(
		nil, nil, nil, nil, nil, decision, nil, nil, nil,
		nil, enrollmentAPI.GuardianInvitationRuntime{}, nil, nil,
		nativeUploads.UploadFiles{}, enrollmentAPI.DatabaseFailures{Transient: transactionFailures.IsTransientDatabaseError, Constraint: transactionFailures.IsConstraintViolation})
	token := mintOfferingGuardReviewerToken(t, reviewer.ID, reviewer.Email)
	return offeringGuardHarness{testpkg.TenantRuntimeMiddleware(t, db)(resource.Router(api.TenantSessionChain)), request.ID, child.ID, token, repos, ctx}
}

func createOfferingGuardPhase(t *testing.T, repos api.EnrollmentTestRepositories, ctx context.Context, mode string) *enrollmentNative.Phase {
	t.Helper()
	phase := &enrollmentNative.Phase{
		Name:                      "Angebotspruefung " + t.Name(),
		Kind:                      enrollmentNative.PhaseKindSchoolYear,
		ServiceStartDate:          enrollmentNative.Date(calendar.TodayDate().AddDays(-30)),
		ServiceEndDate:            enrollmentNative.Date(calendar.TodayDate().AddDays(300)),
		CareOverflowMode:          enrollmentNative.PhaseCareOverflowWaitlist,
		CareOfferingSelectionMode: mode,
		IsActive:                  true,
	}
	phase.TenantID = testpkg.Tenant(t)
	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), repos.Enrollment().InsertPhase, phase))
	return phase
}

func createOfferingGuardChild(t *testing.T, repos api.EnrollmentTestRepositories, ctx context.Context, phaseID int64) (*enrollmentNative.Request, *enrollmentNative.RequestChild) {
	t.Helper()
	request := &enrollmentNative.Request{
		PhaseID:           phaseID,
		GuardianFirstName: "Anna",
		GuardianLastName:  "Beispiel",
		GuardianEmail:     fmt.Sprintf("offering-guard-%d@example.test", testpkg.Tenant(t)),
		ConsentFlags:      []byte("{}"),
		CustomData:        []byte("{}"),
		SubmissionSource:  enrollmentNative.RequestSourcePublic,
		SourceMetadata:    []byte("{}"),
		StatusToken:       fmt.Sprintf("offering-guard-%d-%d", testpkg.Tenant(t), time.Now().UnixNano()),
		SubmittedAt:       time.Now(),
	}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), repos.Enrollment().InsertRequest, request))
	grade := int16(1)
	child := &enrollmentNative.RequestChild{
		RequestID:        request.ID,
		FirstName:        "Lina",
		LastName:         "Beispiel",
		DateOfBirth:      enrollmentNative.Date("2018-04-15"),
		TargetGradeLevel: &grade,
		CustomData:       []byte("{}"),
		Status:           enrollmentNative.ChildStatusSubmitted,
		ActivationMode:   enrollmentNative.ChildActivationScheduled,
	}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), repos.Enrollment().InsertChild, child))
	return request, child
}

func attachOfferingGuardLink(t *testing.T, repos api.EnrollmentTestRepositories, reposCare owners.TimetableCarePlanRows, ctx context.Context, phase *enrollmentNative.Phase, childID int64, fixture offeringLinkFixture) {
	t.Helper()
	if fixture == noOffering {
		return
	}
	if fixture == perDayShared || fixture == perDaySeparate {
		attachPerDayLinks(t, reposCare, ctx, phase.ID, childID, fixture == perDayShared)
		return
	}
	createLink := func(name string, required bool) *enrollmentNative.RequestChildOffering {
		phaseID := phase.ID
		if fixture == crossPhaseOffering {
			otherPhase := &enrollmentNative.Phase{
				Name:                      phase.Name + " Fremd",
				Kind:                      phase.Kind,
				ServiceStartDate:          phase.ServiceStartDate,
				ServiceEndDate:            phase.ServiceEndDate,
				CareOverflowMode:          phase.CareOverflowMode,
				CareOfferingSelectionMode: enrollmentNative.PhaseCareOfferingSelectionOptional,
				IsActive:                  true,
			}
			otherPhase.TenantID = testpkg.Tenant(t)
			require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), repos.Enrollment().InsertPhase, otherPhase))
			phaseID = otherPhase.ID
		}
		offering := &offeringContract.CareOfferingRecord{
			PhaseID: phaseID, Name: name, DaysOfWeekMode: offeringContract.DaysOfWeekModeFixed,
			AvailableDays: []string{"mon", "tue", "wed", "thu", "fri"}, IsActive: true,
			IsRequired:   required,
			CountsAsCare: true,
		}
		require.NoError(t, newCareOfferingFixtures(reposCare.CarePlan).Create(ctx, offering))
		return &enrollmentNative.RequestChildOffering{RequestChildID: childID, CareOfferingID: offering.ID}
	}
	link := createLink("Ganztag", fixture == requiredOnly)
	if fixture == twoOfferings {
		require.NoError(t, api.NewEnrollmentBookingFixture().InsertRequestChildOffering(ctx, link))
		link = createLink("Spätbetreuung", false)
	}
	today := calendar.TodayDate()
	if fixture == expiredOffering {
		link.ValidFrom, link.ValidUntil = datePointers(today.AddDays(-10), today)
	}
	if fixture == futureOffering {
		link.ValidFrom, link.ValidUntil = datePointers(today.AddDays(1), today.AddDays(10))
	}
	if fixture == historicalOffering {
		link.ValidFrom, link.ValidUntil = datePointers(calendar.Date(phase.ServiceStartDate), today)
	}
	require.NoError(t, api.NewEnrollmentBookingFixture().InsertRequestChildOffering(ctx, link))
}

// attachPerDayLinks books two fixed variants of one per-day group, written
// directly as an application from before the rule would hold them.
func attachPerDayLinks(t *testing.T, reposCare owners.TimetableCarePlanRows, ctx context.Context, phaseID, childID int64, shared bool) {
	t.Helper()
	secondDays := []string{"tue"}
	if shared {
		secondDays = []string{"mon", "tue"}
	}
	for _, variant := range []struct {
		name string
		days []string
	}{{"bis 13 Uhr", []string{"mon"}}, {"bis 17 Uhr", secondDays}} {
		offering := &offeringContract.CareOfferingRecord{
			PhaseID: phaseID, Name: variant.name, DaysOfWeekMode: offeringContract.DaysOfWeekModeFixed,
			AvailableDays: variant.days, IsActive: true, CountsAsCare: true,
			SelectionGroup: "Betreuungszeit", SelectionRule: offeringContract.SelectionRuleAtMostOnePerDay,
		}
		require.NoError(t, newCareOfferingFixtures(reposCare.CarePlan).Create(ctx, offering))
		require.NoError(t, api.NewEnrollmentBookingFixture().InsertRequestChildOffering(ctx,
			&enrollmentNative.RequestChildOffering{RequestChildID: childID, CareOfferingID: offering.ID}))
	}
}

func datePointers(from, until calendar.Date) (*enrollmentNative.Date, *enrollmentNative.Date) {
	start, end := enrollmentNative.Date(from), enrollmentNative.Date(until)
	return &start, &end
}

func newOfferingGuardDecisionService(repos api.EnrollmentTestRepositories, reposCare owners.TimetableCarePlanRows, reposTimetable owners.TimetableTestRepositories, offeringsEnabled bool, children TestDecisionChildren, guardianAccess TestDecisionGuardianAccess, studentEnrollment owners.EnrollmentDecisionPeople, bookings TestDecisionBookings) enrollmentAPI.DecisionService {
	return enrollmentAPI.NewDecisionService(enrollmentCompose.PublicDecisions(newOfferingGuardDecisions(repos, reposCare, reposTimetable, offeringsEnabled, children, guardianAccess, studentEnrollment, bookings)))
}

// newOfferingGuardDecisions composes the decision flow the guard suites
// drive, the way the root binds it.
func newOfferingGuardDecisions(repos api.EnrollmentTestRepositories, reposCare owners.TimetableCarePlanRows, reposTimetable owners.TimetableTestRepositories, offeringsEnabled bool, children TestDecisionChildren, guardianAccess TestDecisionGuardianAccess, studentEnrollment owners.EnrollmentDecisionPeople, bookings TestDecisionBookings) *TestDecisions {
	return owners.NewEnrollmentDecisions(enrollmentCompose.DecisionDependencies{
		Requests:       repos.Enrollment(),
		Children:       children,
		Guardians:      repos.Enrollment(),
		LateInvites:    repos.Enrollment(),
		Phases:         repos.Enrollment(),
		Schemas:        repos.Enrollment(),
		Offerings:      enrollmentCompose.NewCareOfferingRecords(reposCare.CarePlan),
		Notifications:  NewTestNotifications(repos.Enrollment(), notifyModeSettings{settings: offeringGuardSettings(offeringsEnabled)}, discardingOutbox{}, nil),
		Bookings:       bookings,
		GuardianAccess: guardianAccess,
		People: owners.EnrollmentDecisionPeopleDirectory(
			repos.Person, repos.Staff, repos.Student, repos.StudentGuardian, repos.GuardianProfile, repos.GuardianPhoneNumber, studentEnrollment,
		),
		StudentEnrollment: studentEnrollment,
		PickupSchedules:   reposTimetable.EnrollmentPickupSchedules(),
		ArrivalSchedules:  reposTimetable.EnrollmentArrivalSchedules(),
		ParentsURL:        owners.EnrollmentDecisionParentsURL("http://parents.localhost:3000", ""),
		Settings:          owners.EnrollmentDecisionSettingValues(offeringGuardSettings(offeringsEnabled)),
		Logger:            slog.Default(),
	})
}

func offeringGuardSettings(offeringsEnabled bool) *settingsDouble {
	return &settingsDouble{
		ResolveStringFn: func(_ context.Context, key string) (string, error) {
			if key == settingKeyDefaultActivationMode {
				return settingValueActivationScheduled, nil
			}
			return settingValueNotifyImmediate, nil
		},
		ResolveBoolFn: func(_ context.Context, key string) (bool, error) {
			return key == settingKeyCareOfferingsEnabled && offeringsEnabled, nil
		},
	}
}

func executeApprovalDecision(t *testing.T, harness offeringGuardHarness) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(map[string]string{"status": enrollmentNative.ChildStatusApproved})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/requests/%d/children/%d/decide", harness.requestID, harness.childID), bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+harness.token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	harness.router.ServeHTTP(rec, req)
	return rec
}

func assertDecisionErrorCode(t *testing.T, rec *httptest.ResponseRecorder, code string) {
	t.Helper()
	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	assert.Equal(t, code, response["code"])
}

func mintOfferingGuardReviewerToken(t *testing.T, reviewerID int64, reviewerEmail string) string {
	t.Helper()
	return testutil.MintTestJWT(t, sessiontoken.AppClaims{
		ID:          int(reviewerID),
		Sub:         reviewerEmail,
		Username:    reviewerEmail,
		FirstName:   "Rita",
		LastName:    "Pruefung",
		Roles:       []string{"admin"},
		Permissions: []string{"config:manage"},
		TenantID:    testpkg.Tenant(t),
	})
}
