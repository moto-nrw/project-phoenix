package enrollmenthttp_test

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/documentrendering/lists"
	enrollment "github.com/moto-ogs/project-phoenix/modules/enrollment"
	compose "github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
)

func TestEnrollmentExportsPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	f := newEnrollmentGrowthFixture(t)
	env := f.env
	ctx := testpkg.Ctx(t)
	audit := owners.NewTestAuditPersistence(testpkg.NewDatabase(t, env.db), nil).DataAccessLog
	f.resource.DecisionService = newTestDecisionService(compose.DecisionDependencies{Requests: env.repos.Enrollment(), Children: env.repos.Enrollment(), Guardians: env.repos.Enrollment(), Phases: env.repos.Enrollment(), Schemas: env.repos.Enrollment(), Offerings: compose.NewCareOfferingRecords(env.reposCare.CarePlan), People: owners.EnrollmentDecisionPeopleDirectory(nil, nil, env.repos.Student, nil, nil, nil, nil), AccessLog: owners.EnrollmentDecisionAccessLog(audit)}, f.settings.Settings)
	f.resource.ReportService = owners.NewEnrollmentReports(owners.EnrollmentReportSources{Owner: env.repos.Enrollment(), Offerings: compose.NewCareOfferingRecords(env.reposCare.CarePlan), AccessLog: audit, Students: env.repos.Student, Persons: env.repos.Person, Settings: f.settings.Settings})
	f.resource.ListExportService = lists.NewRenderer().(lists.DocumentRenderer)
	token := testutil.MintTestJWT(t, sessiontoken.AppClaims{ID: int(env.creatorID), Sub: "export@example.test", Username: "export", TenantID: testpkg.Tenant(t), Roles: []string{"admin"}, Permissions: []string{"config:manage", "users:read"}})
	router := f.router(t)
	counter := testpkg.CaptureQueries(t, env.db)
	student := testpkg.CreateTestStudent(t, env.db, "ExportStudent", "Growth", "1a")
	var names []string
	add := func(n int) {
		for range n {
			i := len(names)
			name := fmt.Sprintf("ExportChild%02d", i)
			request := makeOwnerRequest(env.phaseID, uniqueToken("export-growth"), fmt.Sprintf("export%d@example.test", i))
			request.SchemaID = &env.schemaID
			require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertRequest, request))
			child := makeChild(request.ID, name, "Growth")
			child.Status = enrollment.ChildStatusApproved
			child.CreatedStudentID = &student.ID
			require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertChild, child))
			names = append(names, name)
		}
	}
	run := func() []testpkg.QueryMeasurement {
		routes := []struct{ path, body string }{
			{fmt.Sprintf("/phases/%d/export", env.phaseID), `{"format":"xlsx"}`},
			{fmt.Sprintf("/admin/students/%d/requests/export", student.ID), `{"format":"xlsx"}`},
			{"/admin/reports/care-usage/export", fmt.Sprintf(`{"format":"xlsx","filters":{"phase_id":"%d","status":"all"}}`, env.phaseID)},
		}
		results := make([]testpkg.QueryMeasurement, len(routes))
		for i, route := range routes {
			req := httptest.NewRequest(http.MethodPost, route.path, strings.NewReader(route.body))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			counter.Reset()
			testpkg.MeasuredHTTPRequest(counter, testpkg.MountedQueryBudgetRouter("/api/enrollment", router), rec, req)
			results[i] = counter.Measurement(counter.Queries())
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Contains(t, rec.Header().Get("Content-Type"), "spreadsheetml")
			require.Contains(t, rec.Header().Get("Content-Disposition"), ".xlsx")
			archive, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
			require.NoError(t, err)
			var xml strings.Builder
			for _, file := range archive.File {
				if strings.HasPrefix(file.Name, "xl/") && strings.HasSuffix(file.Name, ".xml") {
					reader, openErr := file.Open()
					require.NoError(t, openErr)
					data, readErr := io.ReadAll(reader)
					require.NoError(t, readErr)
					require.NoError(t, reader.Close())
					xml.Write(data)
				}
			}
			for _, name := range names {
				require.Contains(t, xml.String(), name, route.path)
			}
		}
		return results
	}
	add(3)
	small := run()
	add(5)
	large := run()
	for i := range large {
		t.Logf("export route %d requests N3=%d N8=%d", i, len(small[i].Statements), len(large[i].Statements))
	}
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.phase_export", large[0])
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.student_export", large[1])
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.care_usage_export", large[2])
}
