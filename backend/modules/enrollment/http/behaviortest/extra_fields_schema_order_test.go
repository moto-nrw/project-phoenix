package enrollmenthttp_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"testing"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	enrollment "github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentHTTP "github.com/moto-ogs/project-phoenix/modules/enrollment/http"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Historical schema decoding must not overtake an earlier child's JSON error,
// or let a later broken schema hide the first schema's error.
func TestStudentExtraFieldsKeepsSchemaErrorOrder(t *testing.T) {
	t.Parallel()
	f := newEnrollmentGrowthFixture(t)
	ctx := testpkg.Ctx(t)
	forms := f.resource.FormSchemaService
	first, err := forms.CreateSchema(ctx, "Extra field order", []enrollment.FormField{{Key: "note", Label: "Historical label", Type: enrollment.FormFieldText, AppliesToCh: true}}, f.env.creatorID)
	require.NoError(t, err)
	latest, err := forms.UpdateSchema(ctx, first.ID, []enrollment.FormField{{Key: "note", Label: "New label", Type: enrollment.FormFieldText, AppliesToCh: true}}, f.env.creatorID)
	require.NoError(t, err)
	broken, err := forms.CreateSchema(ctx, "Broken extra field order", []enrollment.FormField{{Key: "note", Label: "Broken", Type: enrollment.FormFieldText, AppliesToCh: true}}, f.env.creatorID)
	require.NoError(t, err)
	_, err = f.env.db.NewUpdate().TableExpr("enrollment.form_schemas").Set("fields = ?::jsonb", `[{"key":"note","options":{}}]`).Where("id = ?", broken.ID).Exec(ctx)
	require.NoError(t, err)
	_, brokenErr := testpkg.TenantQuery1(t, ctx, f.env.db, forms.SchemaVersion, broken.ID)
	require.Error(t, brokenErr)
	var missing int64
	require.NoError(t, f.env.db.NewSelect().ColumnExpr("nextval(pg_get_serial_sequence('enrollment.form_schemas', 'id'))").Scan(ctx, &missing))
	_, missingErr := testpkg.TenantQuery1(t, ctx, f.env.db, forms.SchemaVersion, missing)
	require.Error(t, missingErr)
	student := testpkg.CreateTestStudent(t, f.env.db, "Schema", "Order", "S20")
	summary := func(id int64, raw string) *enrollment.DecisionSummary {
		return &enrollment.DecisionSummary{Request: &enrollment.Request{ID: id, SchemaID: &id}, Children: []*enrollment.RequestChild{{CreatedStudentID: &student.ID, CustomData: testJSONFormValues(json.RawMessage(raw))}}}
	}
	for _, tc := range []struct {
		name        string
		summaries   []*enrollment.DecisionSummary
		schemaError error
		status      int
		label       string
	}{
		{"historical IDs", []*enrollment.DecisionSummary{summary(first.ID, `{"note":"old"}`), summary(latest.ID, `{"note":"new"}`)}, nil, http.StatusOK, "Historical label"},
		{"first schema before later schema", []*enrollment.DecisionSummary{summary(missing, `{"note":"x"}`), summary(broken.ID, `{"note":"x"}`)}, missingErr, http.StatusInternalServerError, ""},
		{"schema decode error", []*enrollment.DecisionSummary{summary(broken.ID, `{"note":"x"}`)}, brokenErr, http.StatusInternalServerError, ""},
		{"empty answers skip missing schemas", []*enrollment.DecisionSummary{nil, {Request: &enrollment.Request{}}, summary(missing, `{}`), summary(broken.ID, `null`)}, nil, http.StatusOK, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			resource := &enrollmentHTTP.StudentResource{EnrollmentDecision: &fakeEnrollmentDecisionService{summaries: tc.summaries}, EnrollmentFormSchema: forms, StudentReadAccess: func(http.ResponseWriter, *http.Request) (int64, bool, bool) { return student.ID, true, true }, Logger: slog.New(slog.NewJSONHandler(&logs, nil))}
			router := api.NewOwnerHTTPTestRouter(resource.Register, "/api/students")
			req := testutil.NewRequest(http.MethodGet, fmt.Sprintf("/api/students/%d/enrollment-extra-fields", student.ID), nil)
			rr := testutil.ExecuteWithAuthPermissions(t, router, req, testutil.AdminTestClaims(int(f.env.creatorID)), []string{"admin:*"})
			require.Equal(t, tc.status, rr.Code, rr.Body.String())
			if tc.schemaError == nil {
				require.Empty(t, logs.String())
			} else {
				var log map[string]any
				require.NoError(t, json.Unmarshal(logs.Bytes(), &log))
				require.Equal(t, tc.schemaError.Error(), log["error"])
			}
			if tc.label != "" {
				require.Contains(t, rr.Body.String(), tc.label)
				require.Contains(t, rr.Body.String(), "New label")
			}
		})
	}
}
