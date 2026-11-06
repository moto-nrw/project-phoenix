package enrollmenthttp_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	enrollment "github.com/moto-nrw/project-phoenix/modules/enrollment"
	compose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentHTTP "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// A malformed pinned version must hide only its own display metadata. Rows
// without a schema keep their offerings and all later phase/child guards run.
func TestEnrollmentHistorySchemaMetadataKeepsRowFailures(t *testing.T) {
	t.Parallel()
	f := newEnrollmentGrowthFixture(t)
	env := f.env
	ctx := testpkg.Ctx(t)
	valid, err := f.resource.FormSchemaService.CreateSchema(ctx, "History metadata", []enrollment.FormField{{Key: "note", Label: "Note", Type: enrollment.FormFieldText}}, env.creatorID)
	require.NoError(t, err)
	broken, err := f.resource.FormSchemaService.CreateSchema(ctx, "Broken history metadata", []enrollment.FormField{{Key: "other", Label: "Other", Type: enrollment.FormFieldText}}, env.creatorID)
	require.NoError(t, err)
	student := testpkg.CreateTestStudent(t, env.db, "History", "Metadata", "2a")
	f.resource.DecisionService = newTestDecisionService(compose.DecisionDependencies{Requests: env.repos.Enrollment(), Children: env.repos.Enrollment(), Guardians: env.repos.Enrollment(), Phases: env.repos.Enrollment(), Schemas: env.repos.Enrollment(), Offerings: compose.NewCareOfferingRecords(env.reposCare.CarePlan)}, f.settings.Settings)
	schemaIDs := []*int64{&valid.ID, &broken.ID, nil}
	requestIDs := make([]string, 0, len(schemaIDs))
	for i, schemaID := range schemaIDs {
		request := makeOwnerRequest(env.phaseID, uniqueToken("history-metadata"), fmt.Sprintf("metadata%d@example.test", i))
		request.SchemaID = schemaID
		request.SubmittedAt = time.Now().Add(time.Duration(i) * time.Minute)
		request.CustomData = json.RawMessage(`{"answer":"retained"}`)
		require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertRequest, request))
		child := makeChild(request.ID, "History", "Metadata")
		child.Status = enrollment.ChildStatusApproved
		child.CreatedStudentID = &student.ID
		require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().InsertChild, child))
		requestIDs = append(requestIDs, fmt.Sprint(request.ID))
	}
	_, err = env.db.NewRaw("UPDATE enrollment.form_schemas SET fields = '{}'::jsonb WHERE id = ? AND tenant_id = ?", broken.ID, testpkg.Tenant(t)).Exec(ctx)
	require.NoError(t, err)
	raw, _ := growthReadOnlyGET(t, f, f.router(t), testpkg.CaptureQueries(t, env.db), f.token, fmt.Sprintf("/admin/students/%d/requests/", student.ID))
	var rows []enrollmentHTTP.AdminRequestSummary
	require.NoError(t, json.Unmarshal(raw, &rows))
	require.Len(t, rows, len(schemaIDs))
	for i, row := range rows {
		require.Equal(t, requestIDs[len(rows)-1-i], row.ID)
		require.Equal(t, map[string]any{"answer": "retained"}, row.CustomData)
		require.Len(t, row.Children, 1)
		require.False(t, row.Children[0].OfferingsUnavailable)
	}
	require.Empty(t, rows[0].SchemaFields)
	require.Empty(t, rows[1].SchemaFields)
	require.Len(t, rows[2].SchemaFields, 1)
	require.Equal(t, "note", rows[2].SchemaFields[0].Key)
	require.Equal(t, "Note", rows[2].SchemaFields[0].Label)
}
