package enrollmenthttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	enrollment "github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentHTTP "github.com/moto-ogs/project-phoenix/modules/enrollment/http"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

// The request collection linked to this child grows, not the background school
// population. Both decision summaries and schema versions use their real owners.
func TestStudents20ExtraFieldsGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	f := newEnrollmentGrowthFixture(t)
	tc := setupStudentsRoute(t)
	feedback, err := api.NewFeedbackTestModule(testpkg.NewDatabase(t, tc.db), testpkg.TenantRuntime(t, tc.db))
	require.NoError(t, err)
	_, _, _, _, decisions, _, err := api.NewStudentTestModule(testpkg.NewDatabase(t, tc.db), testpkg.TenantRuntime(t, tc.db), feedback.Feedback)
	require.NoError(t, err)
	tc.resource.EnrollmentDecision = decisions
	tc.resource.EnrollmentFormSchema = f.resource.FormSchemaService
	schema, err := f.resource.FormSchemaService.CreateSchema(testpkg.Ctx(t), "S20 extras", []enrollment.FormField{{Key: "child_note", Label: "Child note", Type: enrollment.FormFieldText, AppliesToCh: true}, {Key: "guardian_note", Label: "Guardian note", Type: enrollment.FormFieldText}}, f.env.creatorID)
	require.NoError(t, err)
	child := testpkg.CreateTestStudent(t, tc.db, "Extras", "Subject", "S20")
	other := testpkg.CreateTestStudent(t, tc.db, "Extras", "Excluded", "S20")
	expected := make([]enrollmentHTTP.StudentEnrollmentExtraFieldGroup, 0, 8)
	counter := testpkg.CaptureQueriesForContext(t, tc.db)
	for _, n := range []int{3, 8} {
		for len(expected) < n {
			i := len(expected)
			request := makeOwnerRequest(f.env.phaseID, uniqueToken("s20-extras"), fmt.Sprintf("extras%d@example.test", i))
			request.SchemaID = &schema.ID
			request.SubmittedAt = time.Date(2026, 8, 24, 12, i, 0, 0, time.UTC)
			request.CustomData = testJSONFormValues(json.RawMessage(`{"guardian_note":"private guardian value"}`))
			require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), tc.db, f.env.repos.Enrollment().InsertRequest, request))
			linked := makeChild(request.ID, "Extras", "Subject")
			linked.Status = enrollment.ChildStatusApproved
			linked.CreatedStudentID = &child.ID
			linked.CustomData = testFormValues(map[string]any{"child_note": fmt.Sprintf("value%02d", i), "guardian_note": "must not leak"})
			require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), tc.db, f.env.repos.Enrollment().InsertChild, linked))
			sibling := makeChild(request.ID, "Extras", "Excluded")
			sibling.Status = enrollment.ChildStatusApproved
			sibling.CreatedStudentID = &other.ID
			sibling.CustomData = testFormValues(map[string]any{"child_note": "sibling only"})
			require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), tc.db, f.env.repos.Enrollment().InsertChild, sibling))
			expected = append([]enrollmentHTTP.StudentEnrollmentExtraFieldGroup{{RequestID: strconv.FormatInt(request.ID, 10), PhaseName: f.env.phase.Name, SubmittedAt: request.SubmittedAt, Fields: []enrollmentHTTP.StudentEnrollmentExtraField{{Key: "child_note", Label: "Child note", Type: "text", Value: fmt.Sprintf("value%02d", i)}}}}, expected...)
		}
		counter.Reset()
		req := testutil.NewRequest(http.MethodGet, fmt.Sprintf("/api/students/%d/enrollment-extra-fields", child.ID), nil)
		req = req.WithContext(counter.Context(req.Context()))
		router := api.NewOwnerHTTPTestRouter(tc.resource.Register, "/api/students")
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, testutil.AdminTestClaims(int(f.env.creatorID)), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var body struct {
			Data []enrollmentHTTP.StudentEnrollmentExtraFieldGroup `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		require.Len(t, body.Data, n)
		for i := range expected {
			require.True(t, expected[i].SubmittedAt.Equal(body.Data[i].SubmittedAt))
			body.Data[i].SubmittedAt = expected[i].SubmittedAt
		}
		require.Equal(t, expected, body.Data)
		require.Zero(t, counter.WriteRows())
		assertStudents20EnrollmentBudget(t, "enrollment_extra_fields", n, counter.Measurement(counter.Queries()))
		t.Logf("students20 enrollment_extra_fields N%d SQL=%d", n, counter.Total())
	}
	foreignTenant, _ := testpkg.CreateTestTenant(t, tc.db)
	foreign := testpkg.CreateTestStudentForTenant(t, tc.db, foreignTenant, "Foreign", "Private", "S20")
	denied := authExec(t, tc, testutil.NewRequest(http.MethodGet, fmt.Sprintf("/%d/enrollment-extra-fields", foreign.ID), nil), testutil.AdminTestClaims(int(f.env.creatorID)), []string{"admin:*"})
	require.Equal(t, http.StatusNotFound, denied.Code, denied.Body.String())

}

func assertStudents20EnrollmentBudget(t *testing.T, route string, n int, queries querybudgets.Measurement) {
	t.Helper()
	switch fmt.Sprintf("%s.n%d", route, n) {
	case "enrollment_extra_fields.n3":
		querybudgets.AssertQueryBudget(t, "api.students.students20.enrollment_extra_fields.n3", queries)
	case "enrollment_extra_fields.n8":
		querybudgets.AssertQueryBudget(t, "api.students.students20.enrollment_extra_fields.n8", queries)
	default:
		t.Fatalf("unregistered students20 route %s N%d", route, n)
	}
}
