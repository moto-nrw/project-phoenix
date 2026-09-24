// The Kinderkontingent on the approval paths (#3570): approving an
// enrollment that would raise the Kontingentzahl answers 409 with
// students.child_quota_reached and leaves the enrollment open; renewing a
// child that already counts goes through.
package enrollment_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentAPI "github.com/moto-nrw/project-phoenix/api/enrollment"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func fillChildQuota(t *testing.T, db *bun.DB, bundles, bundleSize int) {
	t.Helper()
	_, err := db.NewUpdate().TableExpr("platform.schools").
		Set("child_quota_bundles = ?", bundles).Set("child_quota_bundle_size = ?", bundleSize).
		Where("id = ?", testpkg.Tenant(t)).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
}

func requireChildQuotaRefusal(t *testing.T, rec *httptest.ResponseRecorder, booked, occupied int) {
	t.Helper()
	require.Equal(t, http.StatusConflict, rec.Code, "Body: %s", rec.Body.String())
	var body struct {
		Code    string         `json:"code"`
		Details map[string]int `json:"details"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "students.child_quota_reached", body.Code)
	assert.Equal(t, map[string]int{"booked_places": booked, "occupied_places": occupied, "requested_places": 1}, body.Details)
}

func requireChildStillOpen(t *testing.T, harness offeringGuardHarness) {
	t.Helper()
	child, err := harness.repos.Enrollment().ChildByID(harness.ctx, harness.childID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.ChildStatusSubmitted, child.Status, "the refused enrollment stays open")
	assert.Nil(t, child.CreatedStudentID, "the refused enrollment links no student")
	assert.Nil(t, child.ReviewedAt, "the refused decision left no trace")
}

// matchExistingStudent turns the harness child into the re-enrollment of an
// existing student in the given status.
func matchExistingStudent(t *testing.T, db *bun.DB, harness offeringGuardHarness, status usersModels.StudentStatus) *usersModels.Student {
	t.Helper()
	student := testpkg.CreateTestStudent(t, db, "Wieder", "Angemeldet", "1a")
	_, err := db.NewUpdate().TableExpr("users.student_school_memberships").
		Set("status = ?", string(status)).
		Where("student_profile_id = ? AND deleted_at IS NULL", student.ID).Exec(harness.ctx)
	require.NoError(t, err)
	require.NoError(t, harness.repos.Enrollment().UpdateMatchedStudent(harness.ctx, harness.childID, &student.ID))
	return student
}

func studentStatus(t *testing.T, db *bun.DB, studentID int64) string {
	t.Helper()
	var status string
	require.NoError(t, db.NewSelect().TableExpr("users.student_school_memberships").Column("status").
		Where("student_profile_id = ? AND deleted_at IS NULL", studentID).Scan(testpkg.Ctx(t), &status))
	return status
}

func TestDecideAdminChild_NewChildRefusedByAFullKinderkontingent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	harness := setupOfferingGuardRouterTest(t, enrollmentModels.PhaseCareOfferingSelectionOptional, noOffering, false, false)
	testpkg.CreateTestStudent(t, db, "Schon", "Da", "1a")
	fillChildQuota(t, db, 1, 1)

	rec := executeApprovalDecision(t, harness)

	requireChildQuotaRefusal(t, rec, 1, 1)
	requireChildStillOpen(t, harness)
	var created int
	require.NoError(t, db.NewSelect().TableExpr("users.persons").ColumnExpr("COUNT(*)").
		Where("tenant_id = ?", testpkg.Tenant(t)).Where("first_name = ? AND last_name = ?", "Lina", "Beispiel").
		Scan(harness.ctx, &created))
	assert.Zero(t, created, "the refused approval created no child")
}

func TestDecideAdminChild_RenewalOfAnUncountedChildRefusedByAFullKinderkontingent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	harness := setupOfferingGuardRouterTest(t, enrollmentModels.PhaseCareOfferingSelectionOptional, noOffering, false, false)
	testpkg.CreateTestStudent(t, db, "Schon", "Da", "1a")
	returning := matchExistingStudent(t, db, harness, usersModels.StudentStatusInactive)
	fillChildQuota(t, db, 1, 1)

	rec := executeApprovalDecision(t, harness)

	requireChildQuotaRefusal(t, rec, 1, 1)
	requireChildStillOpen(t, harness)
	assert.Equal(t, string(usersModels.StudentStatusInactive), studentStatus(t, db, returning.ID), "the child is not brought back")
}

func TestDecideAdminChild_RenewalOfADepartedChildRefusedByAFullKinderkontingent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	harness := setupOfferingGuardRouterTest(t, enrollmentModels.PhaseCareOfferingSelectionOptional, noOffering, false, false)
	testpkg.CreateTestStudent(t, db, "Schon", "Da", "1a")
	departed := matchExistingStudent(t, db, harness, usersModels.StudentStatusActive)
	_, err := db.NewUpdate().TableExpr("users.student_school_memberships").
		Set("enrolled_until = ?", timezone.TodayDate().AddDays(-1)).
		Where("student_profile_id = ? AND deleted_at IS NULL", departed.ID).Exec(harness.ctx)
	require.NoError(t, err)
	fillChildQuota(t, db, 1, 1)

	rec := executeApprovalDecision(t, harness)

	requireChildQuotaRefusal(t, rec, 1, 1)
	requireChildStillOpen(t, harness)
}

func TestDecideAdminChild_RenewalOfACountedChildPassesAFullKinderkontingent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	harness := setupOfferingGuardRouterTest(t, enrollmentModels.PhaseCareOfferingSelectionOptional, noOffering, false, false)
	counted := matchExistingStudent(t, db, harness, usersModels.StudentStatusActive)
	fillChildQuota(t, db, 1, 1)

	rec := executeApprovalDecision(t, harness)

	require.Equal(t, http.StatusOK, rec.Code, "Body: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"status":"approved"`)
	assert.Equal(t, string(usersModels.StudentStatusActive), studentStatus(t, db, counted.ID))
}

// The admin area reads why an open enrollment was left to the school.
func TestGetAdminRequest_ShowsARenewalHeldForTheKinderkontingent(t *testing.T) {
	t.Parallel()
	harness := setupOfferingGuardRouterTest(t, enrollmentModels.PhaseCareOfferingSelectionOptional, noOffering, false, false)
	_, err := testpkg.SetupTestDB(t).NewUpdate().TableExpr("enrollment.request_children").
		Set("status = ?", enrollmentModels.ChildStatusAutoRenewed).Where("id = ?", harness.childID).Exec(harness.ctx)
	require.NoError(t, err)
	held, err := harness.repos.Enrollment().HoldAutoRenewedChild(harness.ctx, harness.childID, capability.ReviewReasonChildQuotaReached)
	require.NoError(t, err)
	require.True(t, held)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/requests/%d", harness.requestID), nil)
	req.Header.Set("Authorization", "Bearer "+harness.token)
	rec := httptest.NewRecorder()
	harness.router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "Body: %s", rec.Body.String())
	var body struct {
		Data struct {
			Children []struct {
				Status       string  `json:"status"`
				ReviewReason *string `json:"review_reason"`
			} `json:"children"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Data.Children, 1)
	assert.Equal(t, enrollmentModels.ChildStatusSubmitted, body.Data.Children[0].Status)
	require.NotNil(t, body.Data.Children[0].ReviewReason)
	assert.Equal(t, "child_quota_reached", *body.Data.Children[0].ReviewReason)
}

func TestCreateManualApprovedEnrollment_RefusedByAFullKinderkontingent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repos, err := repositories.NewEnrollmentTestRepositories(db, repositories.NewTestAuditStore(db))
	require.NoError(t, err)
	_, reviewer := testpkg.CreateTestStaffWithAccount(t, db, "Rita", "Pruefung")
	schema, err := enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Owner: repos.Enrollment(), Logger: slog.Default(),
	}).CreateSchema(ctx, "Testformular "+t.Name(), []capability.FormField{
		{Key: "allergies", Label: "Allergien", Type: capability.FormFieldText, SortOrder: 0},
	}, reviewer.ID)
	require.NoError(t, err)
	phase := &capability.Phase{
		Name:                      "Manuell " + t.Name(),
		Kind:                      enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate:          capability.Date(timezone.NewDate(2026, 9, 1)),
		ServiceEndDate:            capability.Date(timezone.NewDate(2027, 7, 31)),
		IsActive:                  true,
		FormSchemaID:              &schema.ID,
		CareOverflowMode:          enrollmentModels.PhaseCareOverflowWaitlist,
		CareOfferingSelectionMode: enrollmentModels.PhaseCareOfferingSelectionOptional,
	}
	phase.TenantID = testpkg.Tenant(t)
	require.NoError(t, repos.Enrollment().InsertPhase(ctx, phase))

	approvedOfferings, err := testutil.NewApprovedOfferingProjection(db, repos.Enrollment())
	require.NoError(t, err)
	guardianAccess, err := identityaccessCompose.New(identityaccessCompose.Dependencies{DB: db, Observe: func(identityaccessCompose.Observation) {}})
	require.NoError(t, err)
	studentEnrollment, err := repositories.NewPeopleDirectory(db)
	require.NoError(t, err)
	decision := newOfferingGuardDecisionService(repos, false, repos.Enrollment(), approvedOfferings, guardianAccess, studentEnrollment)
	requests := enrollmentService.NewRequestService(enrollmentService.RequestServiceConfig{
		Requests: repos.Enrollment(), Children: repos.Enrollment(), Guardians: repos.Enrollment(),
		CareOfferingRepo: enrollmentService.NewCareOfferingRepository(repos.CarePlan), Catalog: repos.Enrollment(),
		SchoolRepo: capabilitySchools{schools: repos.School}, RateLimitRepo: repos.Enrollment(),
		LateInviteRepo: repos.Enrollment(), OutboxEnqueuer: discardingOutbox{}, Settings: stubTakeoverSettings{},
		ManualDecider: decision, FrontendURL: "http://localhost:3000", ParentsURL: "http://parents.localhost:3000",
		DB: db, Logger: slog.Default(),
	})
	resource := enrollmentAPI.NewResource(
		nil, nil, requests, nil, nil, decision, nil, nil, nil,
		nil, enrollmentAPI.GuardianInvitationRuntime{}, nil, nil, db,
	)
	router := testpkg.TenantRuntimeMiddleware(t, db)(resource.Router())

	testpkg.CreateTestStudent(t, db, "Schon", "Da", "1a")
	fillChildQuota(t, db, 1, 1)

	grade := int16(1)
	payload, err := json.Marshal(enrollmentAPI.AdminManualApprovedEnrollmentRequest{
		SubmitEnrollmentRequest: enrollmentAPI.SubmitEnrollmentRequest{
			GuardianFirstName: "Anna", GuardianLastName: "Beispiel",
			GuardianEmail: fmt.Sprintf("manuell-%d@example.test", testpkg.Tenant(t)),
			Children: []enrollmentAPI.SubmitChildRequest{{
				FirstName: "Zu", LastName: "Viel", DateOfBirth: "2019-03-01", TargetGradeLevel: &grade,
			}},
		},
		ExternalConsentConfirmed: true,
		Reason:                   "Anmeldung auf Papier",
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/phases/%d/manual-approved-enrollments", phase.ID), bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+mintOfferingGuardReviewerToken(t, reviewer.ID, reviewer.Email))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	requireChildQuotaRefusal(t, rec, 1, 1)
	leftRequests, err := db.NewSelect().TableExpr("enrollment.requests").
		Where("tenant_id = ?", testpkg.Tenant(t)).Where("phase_id = ?", phase.ID).Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, leftRequests, "the refused manual enrollment left nothing behind")
}
