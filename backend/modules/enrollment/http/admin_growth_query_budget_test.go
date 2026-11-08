package enrollmenthttp_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"

	"github.com/moto-nrw/project-phoenix/api"
	enrollment "github.com/moto-nrw/project-phoenix/modules/enrollment"
	compose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentHTTP "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/stretchr/testify/require"
)

func TestEnrollmentAdminRouteGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	f := newEnrollmentGrowthFixture(t)
	env := f.env
	ctx := testpkg.Ctx(t)
	_, _, audit, _, _, _ := flowRepositories(t, env.db)
	f.resource.DecisionService = newTestDecisionService(compose.DecisionDependencies{
		Requests: env.repos.Enrollment(), Children: env.repos.Enrollment(), Guardians: env.repos.Enrollment(), Phases: env.repos.Enrollment(), Schemas: env.repos.Enrollment(), Offerings: compose.NewCareOfferingRecords(env.reposCare.CarePlan), Adjustments: owners.EnrollmentDecisionAdjustments(audit.EnrollmentOfferingAdjustment),
	}, f.settings.Settings)
	f.resource.ReportService = owners.NewEnrollmentReports(owners.EnrollmentReportSources{Owner: env.repos.Enrollment(), Offerings: compose.NewCareOfferingRecords(env.reposCare.CarePlan), Settings: f.settings.Settings})
	f.resource.UploadList = owners.NewEnrollmentRequestUploads(env.repos.Enrollment(), nil, f.settings.Settings, nil)
	router := f.router(t)
	counter := testpkg.CaptureQueries(t, env.db)
	// The detail and report grow sibling children in one real submitted request.
	runDetail := func(n int) (testpkg.QueryMeasurement, testpkg.QueryMeasurement) {
		schema, schemaErr := f.resource.FormSchemaService.CreateSchema(ctx, fmt.Sprintf("Upload growth %d", n), []enrollment.FormField{{Key: "proof", Label: "Proof", Type: enrollment.FormFieldFile, FileOptions: &enrollment.FormFileOptions{AllowedTypes: []string{"pdf"}, MaxSizeMB: 1, Multiple: true}}}, env.creatorID)
		require.NoError(t, schemaErr)
		phase := *env.phase
		phase.ID = 0
		phase.FormSchemaID = &schema.ID
		phase.Name = fmt.Sprintf("Admin growth %d", n)
		require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertPhase, &phase))
		input := validSubmission(t, phase.ID)
		input.GuardianEmail = fmt.Sprintf("admin%d@example.test", n)
		input.Children = nil
		for i := range n {
			child := validSubmission(t, env.phaseID).Children[0]
			child.FirstName = fmt.Sprintf("Child%d", i)
			input.Children = append(input.Children, child)
		}
		result, err := f.resource.RequestService.Submit(ctx, input)
		require.NoError(t, err)
		uploadIDs := []string{}
		for i := range n {
			now := time.Now()
			hash := testpkg.TokenHash(t, env.db, uniqueToken("upload-growth"))
			upload := &enrollment.RequestUpload{PhaseID: phase.ID, FieldKey: "proof", Filename: fmt.Sprintf("proof%d.pdf", i), StoredName: uniqueToken("stored-upload"), SizeBytes: 12, ContentType: "application/pdf", TokenHash: hash}
			require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertRequestUpload, upload))
			require.NoError(t, testpkg.TenantCommand3(t, ctx, env.db, env.repos.Enrollment().ClaimRequestUploads, []int64{upload.ID}, result.Request.ID, now))
			uploadIDs = append(uploadIDs, fmt.Sprint(upload.ID))
		}
		raw, q := growthGET(t, router, counter, f.token, fmt.Sprintf("/admin/requests/%d/", result.Request.ID))
		var detail enrollmentHTTP.AdminRequestDetail
		require.NoError(t, json.Unmarshal(raw, &detail))
		require.Len(t, detail.Children, n)
		require.Len(t, detail.Uploads, n)
		for i, upload := range detail.Uploads {
			require.Equal(t, uploadIDs[i], upload.ID)
			require.Equal(t, fmt.Sprintf("proof%d.pdf", i), upload.Filename)
			require.Equal(t, "proof", upload.FieldKey)
			require.Equal(t, int64(12), upload.SizeBytes)
			require.Equal(t, "application/pdf", upload.ContentType)
		}
		for i, child := range detail.Children {
			require.Equal(t, fmt.Sprintf("Child%d", i), child.FirstName)
			require.Equal(t, fmt.Sprint(result.Children[i].ID), child.ID)
			require.False(t, child.OfferingsUnavailable)
		}
		raw, reportQ := growthGET(t, router, counter, f.token, fmt.Sprintf("/admin/reports/care-usage?phase_id=%d&status=all", phase.ID))
		var report struct {
			Rows []struct {
				RequestID string `json:"request_id"`
				ChildID   string `json:"child_id"`
				FirstName string `json:"child_first_name"`
				Status    string `json:"status"`
			} `json:"rows"`
		}
		require.NoError(t, json.Unmarshal(raw, &report))
		require.Len(t, report.Rows, n)
		for i, row := range report.Rows {
			require.Equal(t, fmt.Sprint(result.Request.ID), row.RequestID)
			require.Equal(t, fmt.Sprint(result.Children[i].ID), row.ChildID)
			require.Equal(t, fmt.Sprintf("Child%d", i), row.FirstName)
			require.Equal(t, enrollment.ChildStatusSubmitted, row.Status)
		}
		return q, reportQ
	}
	small, reportSmall := runDetail(3)
	large, reportLarge := runDetail(8)
	require.Equal(t, len(small.Statements), len(large.Statements))
	t.Logf("admin detail N3=%d N8=%d", len(small.Statements), len(large.Statements))
	require.Equal(t, len(reportSmall.Statements), len(reportLarge.Statements))
	t.Logf("care usage children N3=%d N8=%d", len(reportSmall.Statements), len(reportLarge.Statements))
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.complete_admin_detail", large)
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.care_usage", reportLarge)

	student := testpkg.CreateTestStudent(t, env.db, "History", "Student", "2a")
	var requestIDs []string
	add := func(n int) {
		for range n {
			i := len(requestIDs)
			request := makeOwnerRequest(env.phaseID, uniqueToken("history-growth"), fmt.Sprintf("history%d@example.test", i))
			request.SchemaID = &env.schemaID
			request.SubmittedAt = time.Now().Add(time.Duration(i) * time.Minute)
			require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertRequest, request))
			child := makeChild(request.ID, "History", "Student")
			child.Status = enrollment.ChildStatusApproved
			child.CreatedStudentID = &student.ID
			require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertChild, child))
			requestIDs = append(requestIDs, fmt.Sprint(request.ID))
		}
	}
	runHistory := func(n int) testpkg.QueryMeasurement {
		raw, q := growthReadOnlyGET(t, f, router, counter, f.token, fmt.Sprintf("/admin/students/%d/requests/", student.ID))
		var rows []enrollmentHTTP.AdminRequestSummary
		require.NoError(t, json.Unmarshal(raw, &rows))
		require.Len(t, rows, n)
		for i, row := range rows {
			require.Equal(t, requestIDs[n-1-i], row.ID)
			require.Len(t, row.Children, 1)
			require.Equal(t, fmt.Sprint(student.ID), row.Children[0].CreatedStudentID)
		}
		return q
	}
	add(3)
	historySmall := runHistory(3)
	add(5)
	historyLarge := runHistory(8)
	t.Logf("student history N3=%d N8=%d", len(historySmall.Statements), len(historyLarge.Statements))
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.student_history", historyLarge)

	// Audit history uses the actual audit owner store, not a service double.
	request := makeOwnerRequest(env.phaseID, uniqueToken("adjustment-growth"), "adjustment@example.test")
	require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertRequest, request))
	child := makeChild(request.ID, "History", "Student")
	child.Status = enrollment.ChildStatusApproved
	child.CreatedStudentID = &student.ID
	require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertChild, child))
	adjustmentIDs := []string{}
	addAdjustments := func(n int) {
		for range n {
			i := len(adjustmentIDs)
			row := &api.EnrollmentFlowOfferingAdjustmentAudit{RequestID: request.ID, RequestChildID: child.ID, StudentID: student.ID, ActorAccountID: env.creatorID, ActorRole: "admin", Reason: fmt.Sprintf("Correction %d", i), Before: json.RawMessage(`{}`), After: json.RawMessage(`{}`), ChangedAt: time.Now().Add(time.Duration(i) * time.Minute)}
			require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, audit.EnrollmentOfferingAdjustment.Create, row))
			adjustmentIDs = append(adjustmentIDs, fmt.Sprint(row.ID))
		}
	}
	runAdjustments := func(n int) testpkg.QueryMeasurement {
		raw, q := growthGET(t, router, counter, f.token, fmt.Sprintf("/admin/requests/%d/children/%d/offering-adjustments", request.ID, child.ID))
		var rows []struct {
			ID     string `json:"id"`
			Reason string `json:"reason"`
		}
		require.NoError(t, json.Unmarshal(raw, &rows))
		require.Len(t, rows, n)
		for i, row := range rows {
			require.Equal(t, adjustmentIDs[n-1-i], row.ID)
			require.Equal(t, fmt.Sprintf("Correction %d", n-1-i), row.Reason)
		}
		return q
	}
	addAdjustments(3)
	adjustmentSmall := runAdjustments(3)
	addAdjustments(5)
	adjustmentLarge := runAdjustments(8)
	require.Equal(t, len(adjustmentSmall.Statements), len(adjustmentLarge.Statements))
	t.Logf("offering adjustments N3=%d N8=%d", len(adjustmentSmall.Statements), len(adjustmentLarge.Statements))
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.offering_adjustments", adjustmentLarge)
}
