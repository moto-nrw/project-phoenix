package enrollmenthttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
)

// The change request view names its answers with the questions of the
// request's pinned form version (#6142), for the school and for the family.

func decodeFormLabels(t *testing.T, rec *httptest.ResponseRecorder) enrollmentAPI.FormLabelsResponse {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Data enrollmentAPI.FormLabelsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.Data
}

func requireAllergiesLabel(t *testing.T, labels enrollmentAPI.FormLabelsResponse) {
	t.Helper()
	require.Len(t, labels.Fields, 1)
	require.Equal(t, "allergies", labels.Fields[0].Key)
	require.Equal(t, "Allergien", labels.Fields[0].Label)
	require.NotNil(t, labels.LegalBlocks)
}

func TestAdminFormLabelsNameAnswersWithPinnedQuestions(t *testing.T) {
	t.Parallel()
	env := setupReviewListTest(t)

	rec := env.getPath(t, fmt.Sprintf("/admin/requests/%d/form-labels", env.requestID), env.token)

	requireAllergiesLabel(t, decodeFormLabels(t, rec))
}

func TestAdminFormLabelsUnknownRequestIsNotFound(t *testing.T) {
	t.Parallel()
	env := setupReviewListTest(t)

	rec := env.getPath(t, fmt.Sprintf("/admin/requests/%d/form-labels", env.requestID+1000), env.token)

	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "enrollment.request_not_found")
}

func TestPublicFormLabelsNameAnswersBehindStatusToken(t *testing.T) {
	t.Parallel()
	env := setupReviewListTest(t)

	rec := env.getPath(t, "/requests/"+env.statusToken+"/form-labels", "")

	requireAllergiesLabel(t, decodeFormLabels(t, rec))
}

func TestPublicFormLabelsRejectUnknownStatusToken(t *testing.T) {
	t.Parallel()
	env := setupReviewListTest(t)

	rec := env.getPath(t, "/requests/not-a-valid-token/form-labels", "")

	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "enrollment.status_link_invalid")
}
