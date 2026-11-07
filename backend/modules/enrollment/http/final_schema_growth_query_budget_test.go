package enrollmenthttp_test

import (
	"encoding/json"
	"fmt"
	"testing"

	enrollment "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmenthttp "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestEnrollmentSchemaPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	f := newEnrollmentGrowthFixture(t)
	ctx := testpkg.Ctx(t)
	router := f.router(t)
	counter := testpkg.CaptureQueries(t, f.env.db)
	run := func(n int) [][]string {
		fields := make([]enrollment.FormField, n)
		for i := range fields {
			fields[i] = enrollment.FormField{Key: fmt.Sprintf("field%d", i), Label: fmt.Sprintf("Field %d", i), Type: enrollment.FormFieldText, SortOrder: i}
		}
		schema, err := f.resource.FormSchemaService.CreateSchema(ctx, fmt.Sprintf("Positive schema %d", n), fields, f.env.creatorID)
		require.NoError(t, err)
		phase := *f.env.phase
		phase.ID = 0
		phase.Name = fmt.Sprintf("Positive schema phase %d", n)
		phase.FormSchemaID = &schema.ID
		require.NoError(t, testpkg.TenantCommand1(t, ctx, f.env.db, f.env.repos.Enrollment().InsertPhase, &phase))
		raw, previewQ := growthGET(t, router, counter, f.token, fmt.Sprintf("/schema/preview?schemaId=%d", schema.ID))
		var preview enrollmenthttp.SchemaPreviewBootstrapResponse
		require.NoError(t, json.Unmarshal(raw, &preview))
		require.NotNil(t, preview.Schema)
		require.Equal(t, fmt.Sprint(schema.ID), preview.Schema.ID)
		require.Equal(t, 1, preview.AssignedPhaseCount)
		require.Equal(t, 1, preview.ActiveAssignedPhaseCount)
		require.Equal(t, fields, preview.Schema.Fields)
		raw, publicQ := growthGET(t, router, counter, "", fmt.Sprintf("/schema/public/%s/%d", f.slug, phase.ID))
		var public enrollmenthttp.PublicFormSchemaResponse
		require.NoError(t, json.Unmarshal(raw, &public))
		require.Equal(t, fmt.Sprint(schema.ID), public.ID)
		require.Equal(t, fields, public.Fields)
		require.Equal(t, schema.Version, public.Version)
		return [][]string{previewQ, publicQ}
	}
	small := run(3)
	large := run(8)
	for i := range large {
		require.Equal(t, len(small[i]), len(large[i]))
		t.Logf("schema route %d fields N3=%d N8=%d", i, len(small[i]), len(large[i]))
	}
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.schema_preview", large[0])
	querybudgets.AssertQueryBudget(t, "api.enrollment.growth.public_schema", large[1])
}
