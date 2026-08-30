package enrollment_test

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

	enrollmentAPI "github.com/moto-nrw/project-phoenix/api/enrollment"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
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
	repos              *repositories.Factory
	ctx                context.Context
}

type failingAtDateOfferingRepo struct {
	enrollmentModels.RequestChildOfferingRepository
}

func (f failingAtDateOfferingRepo) ListByRequestChildIDAtDate(context.Context, int64, timezone.Date) ([]*enrollmentModels.RequestChildOffering, error) {
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
	child, err := harness.repos.RequestChild.FindByID(harness.ctx, harness.childID)
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
	repos := repositories.NewFactory(db)
	_, reviewer := testpkg.CreateTestStaffWithAccount(t, db, "Rita", "Pruefung")
	phase := createOfferingGuardPhase(t, repos, ctx, selectionMode)
	request, child := createOfferingGuardChild(t, repos, ctx, phase.ID)
	attachOfferingGuardLink(t, repos, ctx, phase, child.ID, linkFixture)
	offeringRepo := enrollmentModels.RequestChildOfferingRepository(repos.RequestChildOffering)
	if failOfferingLookup {
		offeringRepo = failingAtDateOfferingRepo{offeringRepo}
	}
	decision := newOfferingGuardDecisionService(repos, offeringsEnabled, offeringRepo)
	resource := enrollmentAPI.NewResource(
		nil, nil, nil, nil, nil, decision, nil, nil, nil,
		nil, nil, nil, nil, db,
	)
	token := mintOfferingGuardReviewerToken(t, reviewer.ID, reviewer.Email)
	return offeringGuardHarness{testpkg.TenantRuntimeMiddleware(t, db)(resource.Router()), request.ID, child.ID, token, repos, ctx}
}

func createOfferingGuardPhase(t *testing.T, repos *repositories.Factory, ctx context.Context, mode string) *enrollmentModels.Phase {
	t.Helper()
	phase := &enrollmentModels.Phase{
		Name:                      "Angebotspruefung " + t.Name(),
		Kind:                      enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate:          timezone.TodayDate().AddDays(-30),
		ServiceEndDate:            timezone.TodayDate().AddDays(300),
		CareOverflowMode:          enrollmentModels.PhaseCareOverflowWaitlist,
		CareOfferingSelectionMode: mode,
		IsActive:                  true,
	}
	phase.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.Phase.Create(ctx, phase))
	return phase
}

func createOfferingGuardChild(t *testing.T, repos *repositories.Factory, ctx context.Context, phaseID int64) (*enrollmentModels.Request, *enrollmentModels.RequestChild) {
	t.Helper()
	request := &enrollmentModels.Request{
		PhaseID:           phaseID,
		GuardianFirstName: "Anna",
		GuardianLastName:  "Beispiel",
		GuardianEmail:     fmt.Sprintf("offering-guard-%d@example.test", testpkg.Tenant(t)),
		ConsentFlags:      map[string]any{},
		CustomData:        map[string]any{},
		SubmissionSource:  enrollmentModels.RequestSourcePublic,
		SourceMetadata:    map[string]any{},
		StatusToken:       fmt.Sprintf("offering-guard-%d-%d", testpkg.Tenant(t), time.Now().UnixNano()),
		SubmittedAt:       time.Now(),
	}
	require.NoError(t, repos.Request.Create(ctx, request))
	grade := int16(1)
	child := &enrollmentModels.RequestChild{
		RequestID:        request.ID,
		FirstName:        "Lina",
		LastName:         "Beispiel",
		DateOfBirth:      timezone.NewDate(2018, 4, 15),
		TargetGradeLevel: &grade,
		CustomData:       map[string]any{},
		Status:           enrollmentModels.ChildStatusSubmitted,
		ActivationMode:   enrollmentModels.ChildActivationScheduled,
	}
	require.NoError(t, repos.RequestChild.Create(ctx, child))
	return request, child
}

func attachOfferingGuardLink(t *testing.T, repos *repositories.Factory, ctx context.Context, phase *enrollmentModels.Phase, childID int64, fixture offeringLinkFixture) {
	t.Helper()
	if fixture == noOffering {
		return
	}
	createLink := func(name string, required bool) *enrollmentModels.RequestChildOffering {
		phaseID := phase.ID
		if fixture == crossPhaseOffering {
			otherPhase := &enrollmentModels.Phase{
				Name:                      phase.Name + " Fremd",
				Kind:                      phase.Kind,
				ServiceStartDate:          phase.ServiceStartDate,
				ServiceEndDate:            phase.ServiceEndDate,
				CareOverflowMode:          phase.CareOverflowMode,
				CareOfferingSelectionMode: enrollmentModels.PhaseCareOfferingSelectionOptional,
				IsActive:                  true,
			}
			otherPhase.SetTenantID(testpkg.Tenant(t))
			require.NoError(t, repos.Phase.Create(ctx, otherPhase))
			phaseID = otherPhase.ID
		}
		offering := &enrollmentModels.CareOffering{
			PhaseID: phaseID, Name: name, DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
			AvailableDays: []string{"mon", "tue", "wed", "thu", "fri"}, IsActive: true,
			IsRequired: required, CountsAsCare: true,
		}
		require.NoError(t, repos.CareOffering.Create(ctx, offering))
		return &enrollmentModels.RequestChildOffering{RequestChildID: childID, CareOfferingID: offering.ID}
	}
	link := createLink("Ganztag", fixture == requiredOnly)
	if fixture == twoOfferings {
		require.NoError(t, repos.RequestChildOffering.Create(ctx, link))
		link = createLink("Spätbetreuung", false)
	}
	today := timezone.TodayDate()
	if fixture == expiredOffering {
		link.ValidFrom, link.ValidUntil = datePointers(today.AddDays(-10), today)
	}
	if fixture == futureOffering {
		link.ValidFrom, link.ValidUntil = datePointers(today.AddDays(1), today.AddDays(10))
	}
	if fixture == historicalOffering {
		link.ValidFrom, link.ValidUntil = datePointers(phase.ServiceStartDate, today)
	}
	require.NoError(t, repos.RequestChildOffering.Create(ctx, link))
}

func datePointers(from, until timezone.Date) (*timezone.Date, *timezone.Date) {
	return &from, &until
}

func newOfferingGuardDecisionService(repos *repositories.Factory, offeringsEnabled bool, offeringRepo enrollmentModels.RequestChildOfferingRepository) enrollmentService.DecisionService {
	return enrollmentService.NewDecisionService(enrollmentService.DecisionServiceConfig{
		RequestRepo: repos.Request, RequestChildRepo: repos.RequestChild, RequestGuardianRepo: repos.RequestGuardian,
		LateInviteRepo: repos.LateInvite, RequestChildOfferingRepo: offeringRepo, CareOfferingRepo: repos.CareOffering,
		PhaseRepo: repos.Phase, FormSchemaRepo: repos.FormSchema, PersonRepo: repos.Person, StaffRepo: repos.Staff,
		StudentRepo: repos.Student, StudentGuardianRepo: repos.StudentGuardian, GuardianProfileRepo: repos.GuardianProfile,
		GuardianPhoneRepo: repos.GuardianPhoneNumber, PickupScheduleRepo: repos.StudentPickupSchedule,
		ArrivalScheduleRepo: repos.StudentArrivalSchedule, StudentEnrollmentRepo: repos.StudentEnrollment,
		ActivityGroupRepo: repos.ActivityGroup, ActivityScheduleRepo: repos.ActivitySchedule,
		CalendarPeriodRepo: repos.CalendarPeriod, TimeframeRepo: repos.Timeframe, ActivityExceptionRepo: repos.ActivityException,
		AccountRepo: repos.Account, AccountTenantRepo: repos.AccountTenant, AccountRoleRepo: repos.AccountRole, RoleRepo: repos.Role,
		OutboxEnqueuer: discardingOutbox{}, Settings: offeringGuardSettings(offeringsEnabled),
		ParentsURL: "http://parents.localhost:3000", Logger: slog.Default(),
	})
}

func offeringGuardSettings(offeringsEnabled bool) *configtest.Mock {
	return &configtest.Mock{
		ResolveStringFn: func(_ context.Context, key string) (string, error) {
			if key == configModels.KeyEnrollmentDefaultActivationMode {
				return configModels.EnrollmentActivationModeScheduled, nil
			}
			return configModels.EnrollmentNotifyPerDecisionImmediate, nil
		},
		ResolveBoolFn: func(_ context.Context, key string) (bool, error) {
			return key == configModels.KeyEnrollmentCareOfferingsEnabled && offeringsEnabled, nil
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
	return testutil.MintTestJWT(t, jwt.AppClaims{
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
