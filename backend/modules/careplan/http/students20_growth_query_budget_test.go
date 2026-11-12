package careplanhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/careplan"
	studentsAPI "github.com/moto-ogs/project-phoenix/modules/careplan/http"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/modules/settings"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

// These are resource HTTP tests with real owner bindings. The test router
// supplies claims, not the production live-session/Serve authentication frame.
func TestStudents20ArrivalGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRoute(t, fixedCalendarClock)
	child := testpkg.CreateTestStudent(t, tc.db, "Arrival", "Subject", "S20")
	other := testpkg.CreateTestStudent(t, tc.db, "Arrival", "Excluded", "S20")
	staff := testpkg.CreateTestStaff(t, tc.db, "Arrival", "Author")

	reason := "S20 exception"
	testpkg.InsertStudentArrivalExceptionRow(t, tc.db, testpkg.StudentArrivalExceptionRow{TenantID: testpkg.Tenant(t), StudentID: other.ID, ExceptionDate: studentsTestToday, Reason: &reason, CreatedBy: staff.ID})
	testpkg.InsertStudentArrivalNoteRow(t, tc.db, testpkg.StudentDatedNoteRow{TenantID: testpkg.Tenant(t), StudentID: other.ID, NoteDate: studentsTestToday, Content: "must not leak", CreatedBy: staff.ID})
	expectedSchedules := make([]studentsAPI.ArrivalScheduleResponse, 0, 3)
	for i, weekdayName := range []string{"Montag", "Dienstag", "Mittwoch"} {
		row := testpkg.CreateTestArrivalSchedule(t, tc.db, child.ID, i+1, staff.ID, "08:15")
		require.NoError(t, tc.db.NewSelect().Model(row).WherePK().Scan(testpkg.Ctx(t)))
		expectedSchedules = append(expectedSchedules, studentsAPI.ArrivalScheduleResponse{ID: row.ID, StudentID: child.ID, Weekday: i + 1, WeekdayName: weekdayName, ExpectedArrival: "08:15", Source: "staff", CreatedBy: staff.ID, CreatedAt: row.CreatedAt.Format(time.RFC3339), UpdatedAt: row.UpdatedAt.Format(time.RFC3339)})
	}
	var expectedExceptions []studentsAPI.ArrivalExceptionResponse
	var expectedNotes []studentsAPI.ArrivalNoteResponse
	counter := testpkg.CaptureQueriesForContext(t, tc.db)
	for _, n := range []int{3, 8} {
		for len(expectedExceptions) < n {
			date := studentsTestToday.AddDays(len(expectedExceptions))
			exception := testpkg.InsertStudentArrivalExceptionRow(t, tc.db, testpkg.StudentArrivalExceptionRow{TenantID: testpkg.Tenant(t), StudentID: child.ID, ExceptionDate: date, Reason: &reason, CreatedBy: staff.ID})
			note := testpkg.InsertStudentArrivalNoteRow(t, tc.db, testpkg.StudentDatedNoteRow{TenantID: testpkg.Tenant(t), StudentID: child.ID, NoteDate: date, Content: fmt.Sprintf("Note %d", len(expectedNotes)), CreatedBy: staff.ID})
			require.NoError(t, tc.db.NewSelect().Model(exception).WherePK().Scan(testpkg.Ctx(t)))
			require.NoError(t, tc.db.NewSelect().Model(note).WherePK().Scan(testpkg.Ctx(t)))

			expectedExceptions = append(expectedExceptions, studentsAPI.ArrivalExceptionResponse{ID: exception.ID, StudentID: child.ID, ExceptionDate: date.String(), Reason: &reason, Source: "staff", CreatedBy: staff.ID, CreatedAt: exception.CreatedAt.Format(time.RFC3339), UpdatedAt: exception.UpdatedAt.Format(time.RFC3339)})
			expectedNotes = append(expectedNotes, studentsAPI.ArrivalNoteResponse{ID: note.ID, StudentID: child.ID, NoteDate: date.String(), Content: note.Content, CreatedBy: staff.ID, CreatedAt: note.CreatedAt.Format(time.RFC3339), UpdatedAt: note.UpdatedAt.Format(time.RFC3339)})
		}
		counter.Reset()
		req := testutil.NewRequest(http.MethodGet, fmt.Sprintf("/%d/arrival-schedules", child.ID), nil)
		req = req.WithContext(counter.Context(req.Context()))
		rr := students20Exec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var body struct {
			Data studentsAPI.ArrivalDataResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		require.Equal(t, expectedExceptions, body.Data.Exceptions)
		require.Equal(t, expectedNotes, body.Data.Notes)
		require.Equal(t, expectedSchedules, body.Data.Schedules)
		require.Zero(t, counter.WriteRows())
		assertStudents20CareplanBudget(t, "arrival", n, counter.Measurement(counter.Queries()))
		t.Logf("students20 arrival N%d SQL=%d", n, counter.Total())
	}
	guest := testpkg.CreateTestAccount(t, tc.db, "s20-arrival-guest@example.com")
	denied := students20Exec(t, tc, testutil.NewRequest(http.MethodGet, fmt.Sprintf("/%d/arrival-schedules", child.ID), nil), testutil.TeacherTestClaims(int(guest.ID)), []string{"users:read"})
	require.Equal(t, http.StatusForbidden, denied.Code, denied.Body.String())
}

func TestStudents20ClassExceptionsGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRoute(t)
	testpkg.CreateTestStudent(t, tc.db, "Class", "Subject", "S20")
	actor := testpkg.CreateTestAccount(t, tc.db, "s20-class@example.com")
	claims := testutil.AdminTestClaims(int(actor.ID))
	start := calendar.NewDate(2099, time.January, 5)
	var expected []studentsAPI.ClassArrivalExceptionResponse
	counter := testpkg.CaptureQueriesForContext(t, tc.db)
	date := start
	for _, n := range []int{3, 8} {
		for len(expected) < n {
			for date.Weekday() == 0 || date.Weekday() == 6 {
				date = date.AddDays(1)
			}
			reason := fmt.Sprintf("Class exception %d", len(expected))
			rr := students20Exec(t, tc, testutil.NewAuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/class-arrival-exceptions/S20/%s", date), map[string]any{"arrival_time": "12:45", "reason": reason}), claims, []string{"admin:*"})
			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			var created struct {
				Data studentsAPI.ClassArrivalExceptionResponse `json:"data"`
			}
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &created))
			require.Equal(t, "S20", created.Data.SchoolClass)
			require.Equal(t, date.String(), created.Data.Date)
			require.Equal(t, "12:45", created.Data.ArrivalTime)
			require.Equal(t, &reason, created.Data.Reason)
			require.Equal(t, "ogs", created.Data.Origin)
			require.NotEmpty(t, created.Data.CreatedAt)
			expected = append(expected, created.Data)
			date = date.AddDays(1)
		}
		counter.Reset()
		req := testutil.NewRequest(http.MethodGet, fmt.Sprintf("/class-arrival-exceptions/S20?from=%s&to=%s", start, date), nil)
		req = req.WithContext(counter.Context(req.Context()))
		rr := students20Exec(t, tc, req, claims, []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var body struct {
			Data studentsAPI.ClassArrivalExceptionListResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		require.Equal(t, studentsAPI.ClassArrivalExceptionListResponse{SchoolClass: "S20", CanEdit: true, Exceptions: expected}, body.Data)
		require.Zero(t, counter.WriteRows())
		assertStudents20CareplanBudget(t, "class_exceptions", n, counter.Measurement(counter.Queries()))
		t.Logf("students20 class_exceptions N%d SQL=%d", n, counter.Total())
	}
}

func students20Exec(t *testing.T, tc *testContext, req *http.Request, claims sessiontoken.AppClaims, perms []string) *httptest.ResponseRecorder {
	t.Helper()
	router := testutil.NewRouter()
	router.Mount("/api/students", tc.router())
	req.URL.Path = "/api/students" + req.URL.Path
	return testutil.ExecuteWithAuthPermissions(t, router, req, claims, perms)
}

func TestStudents20CompanionsGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRoute(t, fixedCalendarClock)
	child := testpkg.CreateTestStudent(t, tc.db, "Companion", "Subject", "S20")
	_ = testpkg.CreateTestStudent(t, tc.db, "Companion", "Unrelated", "S20")
	putStudent(t, tc, child.ID, accompaniedPlanBody(nil))
	type companion struct {
		ID       int64    `json:"companion_student_id"`
		First    string   `json:"first_name"`
		Last     string   `json:"last_name"`
		Weekdays []string `json:"weekdays"`
	}
	expected := make([]companion, 0, 8)
	ids := make([]int64, 0, 8)
	counter := testpkg.CaptureQueriesForContext(t, tc.db)
	for _, n := range []int{3, 8} {
		previous := append([]int64(nil), ids...)
		for len(expected) < n {
			name := fmt.Sprintf("Partner%02d", len(expected))
			partner := testpkg.CreateTestStudent(t, tc.db, "Companion", name, "OtherClass")
			putStudent(t, tc, partner.ID, accompaniedPlanBody(nil))
			expected = append(expected, companion{partner.ID, "Companion", name, []string{"mon"}})
			ids = append(ids, partner.ID)
		}
		entries := make([]map[string]any, 0, n)
		for _, id := range ids {
			entries = append(entries, map[string]any{"companion_student_id": id, "weekdays": []string{"mon"}})
		}
		putStudent(t, tc, child.ID, accompaniedPlanBody(map[string]any{"companions": entries, "companions_fingerprint": mondayLinkFingerprint(previous...)}))
		counter.Reset()
		req := testutil.NewRequest(http.MethodGet, fmt.Sprintf("/%d/companions", child.ID), nil)
		req = req.WithContext(counter.Context(req.Context()))
		rr := students20Exec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var body struct {
			Data []companion `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		require.Equal(t, expected, body.Data)
		require.NotContains(t, rr.Body.String(), "Unrelated")
		require.Zero(t, counter.WriteRows())
		assertStudents20CareplanBudget(t, "companions", n, counter.Measurement(counter.Queries()))
		t.Logf("students20 companions N%d SQL=%d", n, counter.Total())
	}
	guest := testpkg.CreateTestAccount(t, tc.db, "s20-companion-guest@example.com")
	rr := students20Exec(t, tc, testutil.NewRequest(http.MethodGet, fmt.Sprintf("/%d/companions", child.ID), nil), testutil.TeacherTestClaims(int(guest.ID)), []string{"users:read"})
	require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
}

func TestStudents20CareEndPreviewGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRouteWithCareLifecycle(t, false, fixedCalendarClock)
	actor := testpkg.CreateTestAccount(t, tc.db, "s20-care-exit@example.com")
	staff := testpkg.CreateTestStaff(t, tc.db, "Care", "Planner")
	claims := testutil.AdminTestClaims(int(actor.ID))
	ids := make([]string, 0, 8)
	expected := make([]map[string]any, 0, 8)
	counter := testpkg.CaptureQueriesForContext(t, tc.db)
	for _, n := range []int{3, 8} {
		for len(ids) < n {
			name := fmt.Sprintf("Child%02d", len(ids))
			child := testpkg.CreateTestStudent(t, tc.db, "Care", name, "S20")
			testpkg.CreateTestArrivalSchedule(t, tc.db, child.ID, 1, staff.ID, "08:15")
			ids = append(ids, strconv.FormatInt(child.ID, 10))
			expected = append(expected, map[string]any{"student_id": strconv.FormatInt(child.ID, 10), "first_name": "Care", "last_name": name, "school_class": "S20", "planned_roster_rows": float64(0), "activity_bookings": float64(0), "open_parent_requests": float64(0), "has_rfid_tag": false, "currently_present": false, "source_offerings": []any{}, "weekly_plans": []any{"Ankunft am Montag: 08:15"}})
		}
		input := map[string]any{"student_ids": ids, "last_care_day": studentsTestToday.String(), "reason": careplan.CareExitReasonMovedAway}
		counter.Reset()
		req := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/care-end/preview", input)
		req = req.WithContext(counter.Context(req.Context()))
		rr := students20Exec(t, tc, req, claims, []string{"users:delete"})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var body struct {
			Data map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		token, ok := body.Data["token"].(string)
		require.True(t, ok)
		require.Len(t, token, 64)
		delete(body.Data, "token")
		require.Equal(t, map[string]any{"last_care_day": studentsTestToday.String(), "reason": careplan.CareExitReasonMovedAway, "blocked": false, "students": toStudents20Any(expected)}, body.Data)
		require.Zero(t, counter.WriteRows())
		assertStudents20CareplanBudget(t, "care_end_preview", n, counter.Measurement(counter.Queries()))
		t.Logf("students20 care_end_preview N%d SQL=%d", n, counter.Total())
	}
}

func toStudents20Any(rows []map[string]any) []any {
	result := make([]any, 0, len(rows))
	for _, row := range rows {
		result = append(result, row)
	}
	return result
}

func TestStudents20ExportGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRoute(t, fixedCalendarClock)
	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Export", "Teacher")
	foreignTenant, _ := testpkg.CreateTestTenant(t, tc.db)
	testpkg.CreateTestStudentForTenant(t, tc.db, foreignTenant, "Foreign", "Excluded", "S20")
	expected := make([][]string, 0, 8)
	counter := testpkg.CaptureQueries(t, tc.db)
	for _, n := range []int{3, 8} {
		for len(expected) < n {
			name := fmt.Sprintf("Child%02d", len(expected))
			child := testpkg.CreateTestStudent(t, tc.db, name, "Export", "S20")
			testpkg.CreateTestArrivalSchedule(t, tc.db, child.ID, 1, staff.ID, "08:15")
			testpkg.CreateTestPickupSchedule(t, tc.db, child.ID, 1, staff.ID, "16:15")
			expected = append(expected, []string{name + " Export", "16:15"})
		}
		counter.Reset()
		rows := pickupExportRows(t, tc, testutil.AdminTestClaims(int(account.ID)), map[string]string{"school_class": "S20", "date": studentsTestToday.String(), "sort": "name"})
		queries := counter.Queries()
		require.Equal(t, expected, rows)
		require.Zero(t, counter.WriteRows())
		assertStudents20CareplanBudget(t, "export", n, counter.Measurement(counter.Queries()))
		t.Logf("students20 export N%d SQL=%d", n, len(queries))
	}
}

func TestStudents20EndedCareGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRouteWithCareLifecycle(t, false, fixedCalendarClock)
	testpkg.CreateTestStudent(t, tc.db, "Still", "Active", "S20")
	expected := make([]any, 0, 8)
	counter := testpkg.CaptureQueriesForContext(t, tc.db)
	for _, n := range []int{3, 8} {
		for len(expected) < n {
			name := fmt.Sprintf("Child%02d", len(expected))
			child := testpkg.CreateTestStudent(t, tc.db, "Ended", name, "S20")
			_, err := tc.db.NewUpdate().TableExpr("users.student_school_memberships").Set("enrolled_until = ?", studentsTestToday.AddDays(-1)).Where("student_profile_id = ? AND tenant_id = ?", child.ID, testpkg.Tenant(t)).Exec(testpkg.Ctx(t))
			require.NoError(t, err)
			expected = append(expected, map[string]any{"student_id": strconv.FormatInt(child.ID, 10), "first_name": "Ended", "last_name": name, "school_class": "S20", "last_care_day": studentsTestToday.AddDays(-1).String()})
		}
		counter.Reset()
		req := testutil.NewRequest(http.MethodGet, "/ended-care", nil)
		req = req.WithContext(counter.Context(req.Context()))
		rr := students20Exec(t, tc, req, testutil.AdminTestClaims(1), []string{"users:delete"})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var body struct {
			Data map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		require.Equal(t, map[string]any{"items": expected, "total": float64(n), "page": float64(1), "page_size": float64(50)}, body.Data)
		require.Zero(t, counter.WriteRows())
		assertStudents20CareplanBudget(t, "ended_care", n, counter.Measurement(counter.Queries()))
		t.Logf("students20 ended_care N%d SQL=%d", n, counter.Total())
	}
}

func TestStudents20WithdrawalPreviewGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRouteWithCareLifecycle(t, false, fixedCalendarClock)
	child := testpkg.CreateTestStudent(t, tc.db, "Withdrawal", "Subject", "S20")
	actor := testpkg.CreateTestAccount(t, tc.db, "s20-withdrawal@example.com")
	staff := testpkg.CreateTestStaff(t, tc.db, "Withdrawal", "Planner")
	completion := testpkg.CreateTestCareWithdrawalCompletion(t, tc.db, child.ID, actor.ID, studentsTestToday.AddDays(1))
	counter := testpkg.CaptureQueriesForContext(t, tc.db)
	patterns := make([]any, 0, 8)
	for _, n := range []int{3, 8} {
		for len(patterns) < n {
			i := len(patterns)
			weekday := i/2 + 1
			day := []string{"Montag", "Dienstag", "Mittwoch", "Donnerstag"}[weekday-1]
			if i%2 == 0 {
				testpkg.CreateTestArrivalSchedule(t, tc.db, child.ID, weekday, staff.ID, "08:15")
				patterns = append(patterns, "Ankunft am "+day+": 08:15")
			} else {
				testpkg.CreateTestPickupSchedule(t, tc.db, child.ID, weekday, staff.ID, "16:15")
				patterns = append(patterns, "Abholung am "+day+": 16:15")
			}
		}
		sorted := append([]any(nil), patterns...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].(string) < sorted[j].(string) })
		counter.Reset()
		req := testutil.NewAuthenticatedRequest(t, http.MethodPost, fmt.Sprintf("/care-withdrawals/%d/care-end/preview", completion.ID), map[string]any{"last_care_day": studentsTestToday.String(), "reason": careplan.CareExitReasonNoCareNeed})
		req = req.WithContext(counter.Context(req.Context()))
		rr := students20Exec(t, tc, req, testutil.AdminTestClaims(int(actor.ID)), []string{"users:delete"})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var body struct {
			Data map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		token, ok := body.Data["token"].(string)
		require.True(t, ok)
		require.Len(t, token, 64)
		delete(body.Data, "token")
		require.Equal(t, map[string]any{"last_care_day": studentsTestToday.String(), "reason": careplan.CareExitReasonNoCareNeed, "blocked": false, "students": []any{map[string]any{"student_id": strconv.FormatInt(child.ID, 10), "first_name": "Withdrawal", "last_name": "Subject", "school_class": "S20", "planned_roster_rows": float64(0), "activity_bookings": float64(0), "open_parent_requests": float64(0), "has_rfid_tag": false, "currently_present": false, "source_offerings": []any{}, "weekly_plans": sorted}}}, body.Data)
		require.Zero(t, counter.WriteRows())
		stored, err := newStudentTestPeopleRows(testpkg.NewDatabase(t, tc.db)).CareWithdrawal.FindByID(testpkg.Ctx(t), completion.ID)
		require.NoError(t, err)
		require.Equal(t, careplan.WithdrawalStatePending, stored.State)
		require.Equal(t, child.ID, *stored.StudentID)
		assertStudents20CareplanBudget(t, "withdrawal_preview", n, counter.Measurement(counter.Queries()))
		t.Logf("students20 withdrawal_preview N%d SQL=%d", n, counter.Total())
	}
}

func TestStudents20DocumentsGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRoute(t, fixedCalendarClock)
	store := testpkg.NewDatabase(t, tc.db)
	people := api.NewPeopleRepositorySuiteFactory(store)
	audit := owners.NewTestAuditPersistence(store, nil)
	caller := api.NewActiveTestCaller(store, testpkg.TenantRuntime(t, tc.db))
	documents, err := api.NewStudentDocumentsTestModule(store, newStudentTestRepositories(store).CarePlan, people.Student, caller, audit.StudentFieldEdit, audit.DataAccessLog)
	require.NoError(t, err)
	tc.resource.StudentDocumentService = documents
	child := testpkg.CreateTestStudent(t, tc.db, "Documents", "Subject", "S20")
	other := testpkg.CreateTestStudent(t, tc.db, "Documents", "Excluded", "S20")
	actor := testpkg.CreateTestAccount(t, tc.db, "s20-documents@example.com")
	writer := careplan.StudentDocumentActor{AccountID: actor.ID, Name: "Document author", Role: "admin", Permissions: []string{"admin:*"}}
	_, err = documents.CreateStudentDocument(testpkg.Ctx(t), careplan.CreateStudentDocumentInput{StudentID: other.ID, Category: careplan.StudentDocumentCategorySonstiges, FilenameDisplay: "Excluded.pdf", FilenameStored: "s20-excluded.pdf", SizeBytes: 42, ContentType: "application/pdf"}, writer)
	require.NoError(t, err)
	expected := make([]studentsAPI.StudentDocumentResponse, 0, 8)
	counter := testpkg.CaptureQueriesForContext(t, tc.db)
	for _, n := range []int{3, 8} {
		for len(expected) < n {
			filename := fmt.Sprintf("Document%02d.pdf", len(expected))
			doc, createErr := documents.CreateStudentDocument(testpkg.Ctx(t), careplan.CreateStudentDocumentInput{StudentID: child.ID, Category: careplan.StudentDocumentCategorySonstiges, FilenameDisplay: filename, FilenameStored: "s20-" + filename, SizeBytes: 42, ContentType: "application/pdf"}, writer)
			require.NoError(t, createErr)
			expected = append([]studentsAPI.StudentDocumentResponse{{ID: doc.ID, StudentID: child.ID, Category: "sonstiges", CategoryLabel: "Sonstiges", Filename: filename, SizeBytes: 42, ContentType: "application/pdf", UploadedAt: doc.CreatedAt, Sensitive: false, HasFile: true}}, expected...)
		}
		counter.Reset()
		req := testutil.NewRequest(http.MethodGet, fmt.Sprintf("/%d/documents", child.ID), nil)
		req = req.WithContext(counter.Context(req.Context()))
		rr := students20Exec(t, tc, req, testutil.AdminTestClaims(int(actor.ID)), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var body struct {
			Data studentsAPI.StudentDocumentListResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		require.Equal(t, child.ID, body.Data.StudentID)
		require.Equal(t, expected, body.Data.Documents)
		require.Equal(t, []studentsAPI.StudentDocumentCategory{{Value: "betreuungsvertrag", Label: "Betreuungsvertrag"}, {Value: "abholvollmacht", Label: "Abholvollmacht"}, {Value: "schwimmerlaubnis", Label: "Schwimmerlaubnis"}, {Value: "attest", Label: "Ärztliches Attest", Sensitive: true}, {Value: "impfnachweis", Label: "Impfnachweis", Sensitive: true}, {Value: "medikamentenplan", Label: "Medikamentenplan", Sensitive: true}, {Value: "sorgerecht", Label: "Sorgerechtsnachweis", Sensitive: true}, {Value: "unfallanzeige", Label: "Unfallanzeige", Sensitive: true}, {Value: "sonstiges", Label: "Sonstiges"}}, body.Data.VisibleCategories)
		require.Zero(t, counter.WriteRows())
		assertStudents20CareplanBudget(t, "documents", n, counter.Measurement(counter.Queries()))
		t.Logf("students20 documents N%d SQL=%d", n, counter.Total())
	}
	foreignTenant, _ := testpkg.CreateTestTenant(t, tc.db)
	foreign := testpkg.CreateTestStudentForTenant(t, tc.db, foreignTenant, "Foreign", "Private", "S20")
	denied := students20Exec(t, tc, testutil.NewRequest(http.MethodGet, fmt.Sprintf("/%d/documents", foreign.ID), nil), testutil.AdminTestClaims(int(actor.ID)), []string{"admin:*"})
	require.Equal(t, http.StatusNotFound, denied.Code, denied.Body.String())
	require.NotContains(t, denied.Body.String(), "Private")

}

func TestStudents20OfferingPreviewsGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"pickup", "offering_request"} {
		t.Run(route, func(t *testing.T) {
			testpkg.OwnTenant(t)
			testpkg.SetupIsolatedTestDB(t)
			tc := setupStudentsRoute(t, fixedCalendarClock)
			child := testpkg.CreateTestStudent(t, tc.db, "Offering", "Subject", "S20")
			_, actor := testpkg.CreateTestStaffWithAccount(t, tc.db, "Offering", "Reviewer")
			fixture := setupCorrectionFixture(t, tc, child.ID, testpkg.Tenant(t), "Subject")
			require.NoError(t, tc.settings().SetValue(testpkg.Ctx(t), settings.KeyRequirePickupOfferingReview, true, nil, nil))
			offerings := []*testpkg.CareOffering{fixture.ganztag, fixture.mittag}
			pending := insertPendingOfferingChangeRequest(t, tc, fixture, child.ID, actor.ID)
			counter := testpkg.CaptureQueriesForContext(t, tc.db)
			for _, n := range []int{3, 8} {
				for len(offerings) < n {
					offerings = append(offerings, testpkg.CreateTestCareOffering(t, tc.db, fixture.phase.ID, fmt.Sprintf("Choice%02d", len(offerings))))
				}
				payload := make([]map[string]any, 0, n)
				for i, offering := range offerings {
					offering.Name = fmt.Sprintf("Choice%02d", i)
					_, err := tc.db.NewUpdate().TableExpr("enrollment.care_offerings").Set("name = ?", offering.Name).Set("pickup_times = ?", map[string]string{"mon": "14:30", "tue": "14:30", "wed": "14:30", "thu": "14:30", "fri": "14:30"}).Where("id = ? AND tenant_id = ?", offering.ID, testpkg.Tenant(t)).Exec(testpkg.Ctx(t))
					require.NoError(t, err)
					payload = append(payload, map[string]any{"offering_id": offering.ID})
				}
				encoded, err := json.Marshal(map[string]any{"offerings": payload})
				require.NoError(t, err)
				_, err = tc.db.NewUpdate().TableExpr("enrollment.offering_change_requests").Set("payload = ?::jsonb", string(encoded)).Where("id = ? AND tenant_id = ?", pending.ID, testpkg.Tenant(t)).Exec(testpkg.Ctx(t))
				require.NoError(t, err)
				path := fmt.Sprintf("/%d/pickup-schedules/preview", child.ID)
				input := pickupAdjustmentFiveDayBody(fixture.ganztag.ID)
				delete(input, "selections")
				if route == "offering_request" {
					path = fmt.Sprintf("/offering-change-requests/%d/preview", pending.ID)
					input = map[string]any{"effective_from": studentsTestToday.String(), "excluded_offering_ids": []string{}}
				}
				counter.Reset()
				req := testutil.NewAuthenticatedRequest(t, http.MethodPost, path, input)
				req = req.WithContext(counter.Context(req.Context()))
				rr := students20Exec(t, tc, req, testutil.AdminTestClaims(int(actor.ID)), []string{"admin:*"})
				require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
				var body struct {
					Data map[string]any `json:"data"`
				}
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
				if route == "offering_request" {
					selections := make([]any, 0, n)
					for _, offering := range offerings {
						selections = append(selections, map[string]any{"offering_id": strconv.FormatInt(offering.ID, 10), "new": "alle Betreuungstage"})
					}
					require.Equal(t, map[string]any{"selections": selections, "manual_planning_conflicts": []any{}, "arrival_expectations_follow_bookings": false}, body.Data)
				} else {
					token, ok := body.Data["preview_token"].(string)
					require.True(t, ok)
					require.Len(t, token, 64)
					delete(body.Data, "preview_token")
					items := make([]any, 0, n)
					matches := make([]any, 0, n-2)
					for i, offering := range offerings {
						id := strconv.FormatInt(offering.ID, 10)
						items = append(items, map[string]any{"offering_id": id, "name": offering.Name, "days_of_week_mode": "fixed", "available_days": []any{"mon", "tue", "wed", "thu", "fri"}, "selection_rule": "optional", "is_required": false, "includes_lunch": false, "includes_holiday_care": false, "selected": i < 2, "selected_days": nil, "automatic": false, "is_active": true, "pickup_times": map[string]any{"mon": "14:30", "tue": "14:30", "wed": "14:30", "thu": "14:30", "fri": "14:30"}, "counts_as_care": true})
						if i >= 2 {
							matches = append(matches, map[string]any{"offering_id": id, "name": offering.Name, "selected_days": nil, "selections": []any{map[string]any{"offering_id": id, "selected_days": nil}}})
						}
					}
					plan := "Mo 14:30 Uhr, Di 14:30 Uhr, Mi 14:30 Uhr, Do 14:30 Uhr, Fr 14:30 Uhr"
					require.Equal(t, map[string]any{"effective_from": studentsTestToday.String(), "current_plan": plan, "proposed_plan": plan, "deviates_from_offering": false, "resolution_required": false, "matching_offerings": matches, "offering_catalog": map[string]any{"phase_id": strconv.FormatInt(fixture.phase.ID, 10), "phase_name": fixture.phase.Name, "selection_mode": "optional", "earliest_effective_from": studentsTestToday.String(), "latest_effective_from": string(fixture.phase.ServiceEndDate), "items": items}}, body.Data)
				}
				require.Zero(t, counter.WriteRows())
				assertStudents20CareplanBudget(t, route, n, counter.Measurement(counter.Queries()))
				t.Logf("students20 %s N%d SQL=%d", route, n, counter.Total())
			}
		})
	}
}

func assertStudents20CareplanBudget(t *testing.T, route string, n int, queries querybudgets.Measurement) {
	t.Helper()
	switch fmt.Sprintf("%s.n%d", route, n) {
	case "arrival.n3":
		querybudgets.AssertQueryBudget(t, "api.students.students20.arrival.n3", queries)
	case "arrival.n8":
		querybudgets.AssertQueryBudget(t, "api.students.students20.arrival.n8", queries)
	case "class_exceptions.n3":
		querybudgets.AssertQueryBudget(t, "api.students.students20.class_exceptions.n3", queries)
	case "class_exceptions.n8":
		querybudgets.AssertQueryBudget(t, "api.students.students20.class_exceptions.n8", queries)
	case "companions.n3":
		querybudgets.AssertQueryBudget(t, "api.students.students20.companions.n3", queries)
	case "companions.n8":
		querybudgets.AssertQueryBudget(t, "api.students.students20.companions.n8", queries)
	case "care_end_preview.n3":
		querybudgets.AssertQueryBudget(t, "api.students.students20.care_end_preview.n3", queries)
	case "care_end_preview.n8":
		querybudgets.AssertQueryBudget(t, "api.students.students20.care_end_preview.n8", queries)
	case "export.n3":
		querybudgets.AssertQueryBudget(t, "api.students.students20.export.n3", queries)
	case "export.n8":
		querybudgets.AssertQueryBudget(t, "api.students.students20.export.n8", queries)
	case "ended_care.n3":
		querybudgets.AssertQueryBudget(t, "api.students.students20.ended_care.n3", queries)
	case "ended_care.n8":
		querybudgets.AssertQueryBudget(t, "api.students.students20.ended_care.n8", queries)
	case "withdrawal_preview.n3":
		querybudgets.AssertQueryBudget(t, "api.students.students20.withdrawal_preview.n3", queries)
	case "withdrawal_preview.n8":
		querybudgets.AssertQueryBudget(t, "api.students.students20.withdrawal_preview.n8", queries)
	case "documents.n3":
		querybudgets.AssertQueryBudget(t, "api.students.students20.documents.n3", queries)
	case "documents.n8":
		querybudgets.AssertQueryBudget(t, "api.students.students20.documents.n8", queries)
	case "pickup.n3":
		querybudgets.AssertQueryBudget(t, "api.students.students20.pickup.n3", queries)
	case "pickup.n8":
		querybudgets.AssertQueryBudget(t, "api.students.students20.pickup.n8", queries)
	case "offering_request.n3":
		querybudgets.AssertQueryBudget(t, "api.students.students20.offering_request.n3", queries)
	case "offering_request.n8":
		querybudgets.AssertQueryBudget(t, "api.students.students20.offering_request.n8", queries)
	default:
		t.Fatalf("unregistered students20 route %s N%d", route, n)
	}
}

func TestStudents20ForeignChildReadIsolation(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	foreignTenant, _ := testpkg.CreateTestTenant(t, tc.db)
	foreign := testpkg.CreateTestStudentForTenant(t, tc.db, foreignTenant, "Foreign", "Private", "S20")
	for _, suffix := range []string{"arrival-schedules", "companions"} {
		rr := students20Exec(t, tc, testutil.NewRequest(http.MethodGet, fmt.Sprintf("/%d/%s", foreign.ID, suffix), nil), testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
		require.NotContains(t, rr.Body.String(), "Private")
	}
}
