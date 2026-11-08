package enrollmenthttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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

func TestAdminFormLabelsKeepTheDisplayedQuestionsAndHistoricalConsents(t *testing.T) {
	t.Parallel()
	env := setupReviewListTest(t)
	ctx := testpkg.Ctx(t)

	validated, err := env.schemas.UpdateSchema(ctx, env.schemaID, []capability.FormField{{
		Key: "allergies", Label: "Allergien (neu)", Type: capability.FormFieldText, SortOrder: 0,
	}}, env.accountID)
	require.NoError(t, err)
	snapshot, err := json.Marshal([]capability.LegalBlocksSnapshotEntry{{
		DisplayedFormSchemaID: env.schemaID,
		Blocks: []capability.LegalBlockSnapshot{{
			Key: "photo", Title: "Historische Fotoeinwilligung",
			Translations: map[string]map[string]string{"en": {"title": "Historical photo consent"}},
		}},
	}})
	require.NoError(t, err)
	_, err = env.db.NewRaw(`UPDATE enrollment.requests
		SET schema_id = ?, legal_blocks_snapshot = ?::jsonb
		WHERE id = ?`, validated.ID, string(snapshot), env.requestID).Exec(ctx)
	require.NoError(t, err)

	labels := decodeFormLabels(t, env.getPath(t, fmt.Sprintf("/admin/requests/%d/form-labels", env.requestID), env.token))
	require.Len(t, labels.Fields, 1)
	require.Equal(t, "Allergien", labels.Fields[0].Label)
	require.Equal(t, []enrollmentAPI.FormLabelLegalBlock{{
		Key: "photo", Title: "Historische Fotoeinwilligung",
		Translations: capability.Translations{"en": {"title": {Text: "Historical photo consent"}}},
	}}, labels.LegalBlocks)
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
