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

func (s *cancelAfterExtraFieldSchema) SchemaVersion(ctx context.Context, id int64) (*enrollment.FormSchema, error) {
	schema, err := s.FormSchemaAdministration.SchemaVersion(ctx, id)
	if !s.fired {
		s.fired = true
		s.cancel()
	}
	return schema, err
}

// Cancellation after the first real schema query still fails the later schema
// lookup. The production tenant response must keep the handler's error rather
// than replacing its detail with a failed-commit response.
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
	require.Equal(t, "failed to load enrollment extra fields", body["detail"])
	require.Equal(t, "general.server", body["code"])
	require.JSONEq(t, `{"type":"https://moto-app.de/help/fehlermeldungen#anleitung-unerwarteter-fehler","title":"Internal Server Error","status":500,"detail":"failed to load enrollment extra fields","instance":"","code":"general.server"}`, rr.Body.String())
	require.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	require.Contains(t, logs.String(), "context canceled")
}
