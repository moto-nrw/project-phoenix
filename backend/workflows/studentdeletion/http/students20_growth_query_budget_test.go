package studentdeletionhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

// Reuses the typed root and fixture resources, but supplies the real Feedback
// owner rather than the ordinary suite's zero-count double. Production live
// session verification remains outside this resource HTTP seam.
func students20DeletionRoute(t *testing.T, authoritative *bool, clocks ...func() time.Time) *testContext {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	feedback, err := api.NewFeedbackTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc, live, people, audit, decisions, groupLive, err := api.NewStudentHTTPTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), feedback.Feedback, broadcaster, clocks...)
	require.NoError(t, err)
	liveEducation := api.NewActiveTestEducation(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	livePeople := api.BindStudentTestPeople(api.NewActiveTestPeople(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...), svc)
	livePresence := api.NewActiveTestPresence(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	liveTimetable, _ := api.NewActiveTestTimetable(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	liveCaller := api.NewActiveTestCaller(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	liveSettings := api.NewActiveTestSettings(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	liveDevices := api.NewActiveTestDevices(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	_, _, _, _, resource, _, mount, err := api.NewStudentHTTPTestResources(
		testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), svc, live, liveSettings, liveEducation, liveCaller, livePeople, livePresence, liveTimetable, people, audit, groupLive, broadcaster, tenantScopedCaller{liveCaller},
		testpkg.StudentDirectory{}, feedback.Feedback, testpkg.RequestAuditActor,
		deviceAuthenticator(t, db, liveDevices, liveSettings), testutil.AuthenticatedDeviceID, authoritative, firstClock(clocks),
	)
	require.NoError(t, err)
	router := func() chi.Router { return mount(nil) }
	return &testContext{careRequests: svc.CareRequests, enrollmentDecision: decisions, db: db, resource: resource,
		settingsService: liveSettings.Settings, broadcaster: broadcaster, clock: firstClock(clocks), newPickupAdjustments: svc.NewPickupAdjustments, module: svc, router: router}
}

func TestStudents20DeletionImpactGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	for _, withdrawal := range []bool{false, true} {
		t.Run(fmt.Sprintf("withdrawal_%t", withdrawal), func(t *testing.T) {
			testpkg.OwnTenant(t)
			testpkg.SetupIsolatedTestDB(t)
			authoritative := false
			today := calendar.NewDate(2026, 8, 24)
			tc := students20DeletionRoute(t, &authoritative, func() time.Time { return today.BerlinMidnight().Add(12 * time.Hour) })
			child := testpkg.CreateTestStudent(t, tc.db, "Deletion", "Subject", "S20")
			other := testpkg.CreateTestStudent(t, tc.db, "Deletion", "Excluded", "S20")
			staff := testpkg.CreateTestStaff(t, tc.db, "Deletion", "Author")
			_, actor := testpkg.CreateTestStaffWithAccount(t, tc.db, "Deletion", "Reviewer")
			path := fmt.Sprintf("/api/students/%d/delete-impact", child.ID)
			if withdrawal {
				completion := testpkg.CreateTestCareWithdrawalCompletion(t, tc.db, child.ID, actor.ID, today)
				path = fmt.Sprintf("/api/students/care-withdrawals/%d/deletion-impact", completion.ID)
			}
			testpkg.InsertStudentArrivalNoteRow(t, tc.db, testpkg.StudentDatedNoteRow{TenantID: testpkg.Tenant(t), StudentID: other.ID, NoteDate: today, Content: "Excluded note", CreatedBy: staff.ID})
			created := 0
			counter := testpkg.CaptureQueriesForContext(t, tc.db)
			for _, n := range []int{3, 8} {
				for created < n {
					testpkg.InsertStudentArrivalNoteRow(t, tc.db, testpkg.StudentDatedNoteRow{TenantID: testpkg.Tenant(t), StudentID: child.ID, NoteDate: today.AddDays(created), Content: fmt.Sprintf("Dependent note %d", created), CreatedBy: staff.ID})
					created++
				}
				counter.Reset()
				router := testutil.NewRouter()
				router.Mount("/api/students", tc.router())
				req := testutil.NewRequest(http.MethodGet, path, nil)
				req = req.WithContext(counter.Context(req.Context()))
				rr := testutil.ExecuteWithAuthPermissions(t, router, req, testutil.AdminTestClaims(int(actor.ID)), []string{"users:delete"})
				require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
				var body struct {
					Data map[string]any `json:"data"`
				}
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
				fingerprint, ok := body.Data["fingerprint"].(string)
				require.True(t, ok)
				require.Len(t, fingerprint, 64)
				delete(body.Data, "fingerprint")
				expectedCounts := map[string]any{"timetable_assignments": float64(0), "activity_enrollments": float64(0), "attendance_records": float64(0), "care_schedules": float64(n), "guardian_links": float64(0), "companion_links": float64(0), "communications": float64(0), "consents": float64(0), "enrollment_references": float64(0), "other_records": float64(0)}
				require.Equal(t, map[string]any{"confirmation_name": "Deletion Subject", "counts": expectedCounts, "total": float64(n), "preserved": map[string]any{"guardian_profiles": true, "parent_accounts": true, "other_students": true, "shared_instances": true}}, body.Data)
				require.Zero(t, counter.WriteRows())
				require.NotEmpty(t, counter.Selects("feedback.entries"), "real Feedback count must execute")
				if withdrawal {
					assertStudents20DeletionBudget(t, "withdrawal_deletion_impact", n, counter.Measurement(counter.Queries()))
				} else {
					assertStudents20DeletionBudget(t, "delete_impact", n, counter.Measurement(counter.Queries()))
				}
				t.Logf("students20 deletion_impact withdrawal=%t N%d SQL=%d", withdrawal, n, counter.Total())
			}
			foreignTenant, _ := testpkg.CreateTestTenant(t, tc.db)
			foreign := testpkg.CreateTestStudentForTenant(t, tc.db, foreignTenant, "Foreign", "Private", "S20")
			denied := authExec(t, tc, testutil.NewRequest(http.MethodGet, fmt.Sprintf("/%d/delete-impact", foreign.ID), nil), testutil.AdminTestClaims(int(actor.ID)), []string{"users:delete"})
			require.Equal(t, http.StatusNotFound, denied.Code, denied.Body.String())
			require.NotContains(t, denied.Body.String(), "Private")

		})
	}
}

func assertStudents20DeletionBudget(t *testing.T, route string, n int, queries querybudgets.Measurement) {
	t.Helper()
	switch fmt.Sprintf("%s.n%d", route, n) {
	case "delete_impact.n3":
		querybudgets.AssertQueryBudget(t, "api.students.students20.delete_impact.n3", queries)
	case "delete_impact.n8":
		querybudgets.AssertQueryBudget(t, "api.students.students20.delete_impact.n8", queries)
	case "withdrawal_deletion_impact.n3":
		querybudgets.AssertQueryBudget(t, "api.students.students20.withdrawal_deletion_impact.n3", queries)
	case "withdrawal_deletion_impact.n8":
		querybudgets.AssertQueryBudget(t, "api.students.students20.withdrawal_deletion_impact.n8", queries)
	default:
		t.Fatalf("unregistered students20 route %s N%d", route, n)
	}
}
