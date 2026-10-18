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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
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

func (f failingAtDateOfferingReader) RequestChildOfferingsAtDate(context.Context, int64, capability.Date) ([]*capability.RequestChildOffering, error) {
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
		{"at least one missing", enrollmentModels.PhaseCareOfferingSelectionAtLeastOne, noOffering, true, http.StatusConflict, "enrollment.approval_care_offering_missing"},
		{"at least one required only", enrollmentModels.PhaseCareOfferingSelectionAtLeastOne, requiredOnly, true, http.StatusConflict, "enrollment.approval_care_offering_missing"},
		{"exactly one missing", enrollmentModels.PhaseCareOfferingSelectionExactlyOne, noOffering, true, http.StatusConflict, "enrollment.approval_care_offering_exactly_one"},
		{"exactly one required only", enrollmentModels.PhaseCareOfferingSelectionExactlyOne, requiredOnly, true, http.StatusConflict, "enrollment.approval_care_offering_exactly_one"},
		{"exactly one multiple", enrollmentModels.PhaseCareOfferingSelectionExactlyOne, twoOfferings, true, http.StatusConflict, "enrollment.approval_care_offering_exactly_one"},
		{"required while feature disabled", enrollmentModels.PhaseCareOfferingSelectionAtLeastOne, noOffering, false, http.StatusOK, ""},
		{"expired link", enrollmentModels.PhaseCareOfferingSelectionAtLeastOne, expiredOffering, true, http.StatusConflict, "enrollment.approval_care_offering_missing"},
		{"future link", enrollmentModels.PhaseCareOfferingSelectionAtLeastOne, futureOffering, true, http.StatusConflict, "enrollment.approval_care_offering_missing"},
		{"historical link active at service start", enrollmentModels.PhaseCareOfferingSelectionAtLeastOne, historicalOffering, true, http.StatusOK, ""},
		{"cross-phase link", enrollmentModels.PhaseCareOfferingSelectionAtLeastOne, crossPhaseOffering, true, http.StatusConflict, "enrollment.approval_care_offering_missing"},
		{"optional missing", enrollmentModels.PhaseCareOfferingSelectionOptional, noOffering, true, http.StatusOK, ""},
		{"required effective", enrollmentModels.PhaseCareOfferingSelectionAtLeastOne, effectiveOffering, true, http.StatusOK, ""},
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
	harness := setupOfferingGuardRouterTest(t, enrollmentModels.PhaseCareOfferingSelectionAtLeastOne, effectiveOffering, true, true)
	rec := executeApprovalDecision(t, harness)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	child, err := harness.repos.Enrollment().ChildByID(harness.ctx, harness.childID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.ChildStatusSubmitted, child.Status)
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
	repos, repoErr := api.NewEnrollmentTestRepositories(testpkg.NewDatabase(t, db), api.NewTestAuditStore(testpkg.NewDatabase(t, db)))
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
	studentEnrollment, err := api.NewPeopleDirectory(testpkg.MustDatabase(db))
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

func createOfferingGuardPhase(t *testing.T, repos api.EnrollmentTestRepositories, ctx context.Context, mode string) *capability.Phase {
	t.Helper()
	phase := &capability.Phase{
		Name:                      "Angebotspruefung " + t.Name(),
		Kind:                      enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate:          capability.Date(calendar.TodayDate().AddDays(-30)),
		ServiceEndDate:            capability.Date(calendar.TodayDate().AddDays(300)),
		CareOverflowMode:          enrollmentModels.PhaseCareOverflowWaitlist,
		CareOfferingSelectionMode: mode,
		IsActive:                  true,
	}
	phase.TenantID = testpkg.Tenant(t)
	require.NoError(t, repos.Enrollment().InsertPhase(ctx, phase))
	return phase
}

func createOfferingGuardChild(t *testing.T, repos api.EnrollmentTestRepositories, ctx context.Context, phaseID int64) (*capability.Request, *capability.RequestChild) {
	t.Helper()
	request := &capability.Request{
		PhaseID:           phaseID,
		GuardianFirstName: "Anna",
		GuardianLastName:  "Beispiel",
		GuardianEmail:     fmt.Sprintf("offering-guard-%d@example.test", testpkg.Tenant(t)),
		ConsentFlags:      []byte("{}"),
		CustomData:        []byte("{}"),
		SubmissionSource:  enrollmentModels.RequestSourcePublic,
		SourceMetadata:    []byte("{}"),
		StatusToken:       fmt.Sprintf("offering-guard-%d-%d", testpkg.Tenant(t), time.Now().UnixNano()),
		SubmittedAt:       time.Now(),
	}
	require.NoError(t, repos.Enrollment().InsertRequest(ctx, request))
	grade := int16(1)
	child := &capability.RequestChild{
		RequestID:        request.ID,
		FirstName:        "Lina",
		LastName:         "Beispiel",
		DateOfBirth:      capability.Date("2018-04-15"),
		TargetGradeLevel: &grade,
		CustomData:       []byte("{}"),
		Status:           enrollmentModels.ChildStatusSubmitted,
		ActivationMode:   enrollmentModels.ChildActivationScheduled,
	}
	require.NoError(t, repos.Enrollment().InsertChild(ctx, child))
	return request, child
}

func attachOfferingGuardLink(t *testing.T, repos api.EnrollmentTestRepositories, reposCare api.TimetableCarePlanRows, ctx context.Context, phase *capability.Phase, childID int64, fixture offeringLinkFixture) {
	t.Helper()
	if fixture == noOffering {
		return
	}
	createLink := func(name string, required bool) *capability.RequestChildOffering {
		phaseID := phase.ID
		if fixture == crossPhaseOffering {
			otherPhase := &capability.Phase{
				Name:                      phase.Name + " Fremd",
				Kind:                      phase.Kind,
				ServiceStartDate:          phase.ServiceStartDate,
				ServiceEndDate:            phase.ServiceEndDate,
				CareOverflowMode:          phase.CareOverflowMode,
				CareOfferingSelectionMode: enrollmentModels.PhaseCareOfferingSelectionOptional,
				IsActive:                  true,
			}
			otherPhase.TenantID = testpkg.Tenant(t)
			require.NoError(t, repos.Enrollment().InsertPhase(ctx, otherPhase))
			phaseID = otherPhase.ID
		}
		offering := &enrollmentModels.CareOffering{
			PhaseID: phaseID, Name: name, DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
			AvailableDays: []string{"mon", "tue", "wed", "thu", "fri"}, IsActive: true,
			IsRequired:   required,
			CountsAsCare: true,
		}
		require.NoError(t, newCareOfferingFixtures(reposCare.CarePlan).Create(ctx, offering))
		return &capability.RequestChildOffering{RequestChildID: childID, CareOfferingID: offering.ID}
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

func datePointers(from, until calendar.Date) (*capability.Date, *capability.Date) {
	start, end := capability.Date(from), capability.Date(until)
	return &start, &end
}

func newOfferingGuardDecisionService(repos api.EnrollmentTestRepositories, reposCare api.TimetableCarePlanRows, reposTimetable api.TimetableTestRepositories, offeringsEnabled bool, children TestDecisionChildren, guardianAccess TestDecisionGuardianAccess, studentEnrollment api.EnrollmentDecisionPeople, bookings TestDecisionBookings) enrollmentAPI.DecisionService {
	return enrollmentAPI.NewDecisionService(enrollmentCompose.PublicDecisions(newOfferingGuardDecisions(repos, reposCare, reposTimetable, offeringsEnabled, children, guardianAccess, studentEnrollment, bookings)))
}

// newOfferingGuardDecisions composes the decision flow the guard suites
// drive, the way the root binds it.
func newOfferingGuardDecisions(repos api.EnrollmentTestRepositories, reposCare api.TimetableCarePlanRows, reposTimetable api.TimetableTestRepositories, offeringsEnabled bool, children TestDecisionChildren, guardianAccess TestDecisionGuardianAccess, studentEnrollment api.EnrollmentDecisionPeople, bookings TestDecisionBookings) *TestDecisions {
	return api.NewEnrollmentDecisions(enrollmentCompose.DecisionDependencies{
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
		People: api.EnrollmentDecisionPeopleDirectory(
			repos.Person, repos.Staff, repos.Student, repos.StudentGuardian, repos.GuardianProfile, repos.GuardianPhoneNumber, studentEnrollment,
		),
		StudentEnrollment: studentEnrollment,
		PickupSchedules:   reposTimetable.EnrollmentPickupSchedules(),
		ArrivalSchedules:  reposTimetable.EnrollmentArrivalSchedules(),
		ParentsURL:        api.EnrollmentDecisionParentsURL("http://parents.localhost:3000", ""),
		Settings:          api.EnrollmentDecisionSettingValues(offeringGuardSettings(offeringsEnabled)),
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
	body, err := json.Marshal(map[string]string{"status": enrollmentModels.ChildStatusApproved})
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
