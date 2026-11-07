package enrollmenthttp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	enrollment "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentHTTP "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type cancelAfterExtraFieldSchema struct {
	enrollment.FormSchemaAdministration
	cancel context.CancelFunc
	fired  bool
}

func (s *cancelAfterExtraFieldSchema) StudentExtraFieldSchemas(ctx context.Context, ids []int64) ([]enrollment.SchemaVersionResult, error) {
	results, err := s.FormSchemaAdministration.StudentExtraFieldSchemas(ctx, ids)
	if !s.fired {
		s.fired = true
		s.cancel()
	}
	return results, err
}

// Cancellation after the real schema batch must fail the tenant transaction,
// never release its buffered success. Internal 500 details may change.
func TestStudentExtraFieldsKeepsCanceledSchemaFailure(t *testing.T) {
	t.Parallel()
	f := newEnrollmentGrowthFixture(t)
	schema, err := f.resource.FormSchemaService.CreateSchema(testpkg.Ctx(t), "Canceled extra fields", []enrollment.FormField{{Key: "note", Label: "Note", Type: enrollment.FormFieldText, AppliesToCh: true}}, f.env.creatorID)
	require.NoError(t, err)
	student := testpkg.CreateTestStudent(t, f.env.db, "Canceled", "Schemas", "S20")
	var logs bytes.Buffer
	forms := &cancelAfterExtraFieldSchema{FormSchemaAdministration: f.resource.FormSchemaService}
	summaries := make([]*enrollment.DecisionSummary, 0, 2)
	for range 2 {
		summaries = append(summaries, &enrollment.DecisionSummary{Request: &enrollment.Request{ID: schema.ID, SchemaID: &schema.ID}, Children: []*enrollment.RequestChild{{CreatedStudentID: &student.ID, CustomData: json.RawMessage(`{"note":"value"}`)}}})
	}
	resource := &enrollmentHTTP.StudentResource{EnrollmentDecision: &fakeEnrollmentDecisionService{summaries: summaries}, EnrollmentFormSchema: forms, StudentReadAccess: func(http.ResponseWriter, *http.Request) (int64, bool, bool) { return student.ID, true, true }, Logger: slog.New(slog.NewJSONHandler(&logs, nil))}
	router := api.NewOwnerHTTPTestRouter(resource.Register, "/api/students")
	req := testutil.NewRequest(http.MethodGet, fmt.Sprintf("/api/students/%d/enrollment-extra-fields", student.ID), nil)
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	forms.cancel = cancel
	rr := testutil.ExecuteWithAuthPermissions(t, router, req.WithContext(ctx), testutil.AdminTestClaims(int(f.env.creatorID)), []string{"admin:*"})
	t.Logf("wire status=%d headers=%v body=%s schema_log=%s", rr.Code, rr.Header(), rr.Body.String(), logs.String())
	require.True(t, forms.fired)
	require.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.NotEmpty(t, body["detail"])
	require.Equal(t, "general.server", body["code"])
	delete(body, "detail")
	require.Equal(t, map[string]any{"type": "https://moto-app.de/help/fehlermeldungen#anleitung-unerwarteter-fehler", "title": "Internal Server Error", "status": float64(500), "instance": "", "code": "general.server"}, body)
	require.Equal(t, "application/json", rr.Header().Get("Content-Type"))
}
