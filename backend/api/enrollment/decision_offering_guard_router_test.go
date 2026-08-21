package enrollment_test

import (
	"bytes"
	"context"
	"encoding/json"
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
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type offeringGuardSettings struct{}

func (offeringGuardSettings) ResolveString(_ context.Context, key string) (string, error) {
	switch key {
	case configModels.KeyEnrollmentDefaultActivationMode:
		return configModels.EnrollmentActivationModeScheduled, nil
	case configModels.KeyEnrollmentNotifyPerDecision:
		return configModels.EnrollmentNotifyPerDecisionImmediate, nil
	default:
		return "", nil
	}
}

func (offeringGuardSettings) ResolveBool(_ context.Context, key string) (bool, error) {
	return key == configModels.KeyEnrollmentCareOfferingsEnabled, nil
}

func TestDecideAdminChild_RejectsApprovalWithoutOfferingWhenPhaseRequiresOne(t *testing.T) {
	t.Parallel()

	router, requestID, childID, token := setupOfferingGuardRouterTest(
		t,
		enrollmentModels.PhaseCareOfferingSelectionAtLeastOne,
		false,
	)
	body, err := json.Marshal(map[string]string{"status": enrollmentModels.ChildStatusApproved})
	require.NoError(t, err)
	req := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/admin/requests/%d/children/%d/decide", requestID, childID),
		bytes.NewReader(body),
	)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	assert.Equal(t, "enrollment.approval_care_offering_missing", response["code"])
}

func TestDecideAdminChild_RejectsApprovalWithoutOfferingWhenPhaseRequiresExactlyOne(t *testing.T) {
	t.Parallel()

	router, requestID, childID, token := setupOfferingGuardRouterTest(
		t,
		enrollmentModels.PhaseCareOfferingSelectionExactlyOne,
		false,
	)
	body, err := json.Marshal(map[string]string{"status": enrollmentModels.ChildStatusApproved})
	require.NoError(t, err)
	req := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/admin/requests/%d/children/%d/decide", requestID, childID),
		bytes.NewReader(body),
	)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)
	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	assert.Equal(t, "enrollment.approval_care_offering_missing", response["code"])
}

func TestDecideAdminChild_AllowsApprovalWithoutOfferingWhenPhaseIsOptional(t *testing.T) {
	t.Parallel()

	router, requestID, childID, token := setupOfferingGuardRouterTest(
		t,
		enrollmentModels.PhaseCareOfferingSelectionOptional,
		false,
	)
	body, err := json.Marshal(map[string]string{"status": enrollmentModels.ChildStatusApproved})
	require.NoError(t, err)
	req := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/admin/requests/%d/children/%d/decide", requestID, childID),
		bytes.NewReader(body),
	)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"approved"`)
}

func TestDecideAdminChild_AllowsRequiredPhaseWithEffectiveOffering(t *testing.T) {
	t.Parallel()

	router, requestID, childID, token := setupOfferingGuardRouterTest(
		t,
		enrollmentModels.PhaseCareOfferingSelectionAtLeastOne,
		true,
	)
	body, err := json.Marshal(map[string]string{"status": enrollmentModels.ChildStatusApproved})
	require.NoError(t, err)
	req := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/admin/requests/%d/children/%d/decide", requestID, childID),
		bytes.NewReader(body),
	)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"approved"`)
}

func setupOfferingGuardRouterTest(
	t *testing.T,
	selectionMode string,
	withOffering bool,
) (http.Handler, int64, int64, string) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repos := repositories.NewFactory(db)

	_, reviewer := testpkg.CreateTestStaffWithAccount(t, db, "Rita", "Pruefung")
	phase := &enrollmentModels.Phase{
		Name:                      "Angebotspruefung " + t.Name(),
		Kind:                      enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate:          timezone.TodayDate().AddDays(-30),
		ServiceEndDate:            timezone.TodayDate().AddDays(300),
		CareOverflowMode:          enrollmentModels.PhaseCareOverflowWaitlist,
		CareOfferingSelectionMode: selectionMode,
		IsActive:                  true,
	}
	phase.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.Phase.Create(ctx, phase))

	request := &enrollmentModels.Request{
		PhaseID:           phase.ID,
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
	if withOffering {
		offering := &enrollmentModels.CareOffering{
			PhaseID:        phase.ID,
			Name:           "Ganztag",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
			IsActive:       true,
			CountsAsCare:   true,
		}
		require.NoError(t, repos.CareOffering.Create(ctx, offering))
		require.NoError(t, repos.RequestChildOffering.Create(ctx, &enrollmentModels.RequestChildOffering{
			RequestChildID: child.ID,
			CareOfferingID: offering.ID,
		}))
	}

	decision := enrollmentService.NewDecisionService(enrollmentService.DecisionServiceConfig{
		RequestRepo:              repos.Request,
		RequestChildRepo:         repos.RequestChild,
		RequestGuardianRepo:      repos.RequestGuardian,
		LateInviteRepo:           repos.LateInvite,
		RequestChildOfferingRepo: repos.RequestChildOffering,
		CareOfferingRepo:         repos.CareOffering,
		PhaseRepo:                repos.Phase,
		FormSchemaRepo:           repos.FormSchema,
		PersonRepo:               repos.Person,
		StaffRepo:                repos.Staff,
		StudentRepo:              repos.Student,
		StudentGuardianRepo:      repos.StudentGuardian,
		GuardianProfileRepo:      repos.GuardianProfile,
		GuardianPhoneRepo:        repos.GuardianPhoneNumber,
		PickupScheduleRepo:       repos.StudentPickupSchedule,
		ArrivalScheduleRepo:      repos.StudentArrivalSchedule,
		StudentEnrollmentRepo:    repos.StudentEnrollment,
		ActivityGroupRepo:        repos.ActivityGroup,
		ActivityScheduleRepo:     repos.ActivitySchedule,
		CalendarPeriodRepo:       repos.CalendarPeriod,
		TimeframeRepo:            repos.Timeframe,
		ActivityExceptionRepo:    repos.ActivityException,
		AccountRepo:              repos.Account,
		AccountTenantRepo:        repos.AccountTenant,
		AccountRoleRepo:          repos.AccountRole,
		RoleRepo:                 repos.Role,
		OutboxEnqueuer:           discardingOutbox{},
		Settings:                 offeringGuardSettings{},
		ParentsURL:               "http://parents.localhost:3000",
		Logger:                   slog.Default(),
	})
	resource := enrollmentAPI.NewResource(
		nil, nil, nil, nil, nil, decision, nil, nil, nil,
		nil, nil, nil, nil, db,
	)
	token := testutil.MintTestJWT(t, jwt.AppClaims{
		ID:          int(reviewer.ID),
		Sub:         reviewer.Email,
		Username:    reviewer.Email,
		FirstName:   "Rita",
		LastName:    "Pruefung",
		Roles:       []string{"admin"},
		Permissions: []string{"config:manage"},
		TenantID:    testpkg.Tenant(t),
	})
	return resource.Router(), request.ID, child.ID, token
}
