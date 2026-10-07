package inbound

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	substitutionsHTTP "github.com/moto-nrw/project-phoenix/api/substitutions"
	"github.com/stretchr/testify/require"
)

// TestRenderSubstitutionsFailureCarriesDetailsAndField pins that a refusal
// naming its values and field reaches the client with both (#2516).
func TestRenderSubstitutionsFailureCarriesDetailsAndField(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/substitutions", nil)
	renderSubstitutionsFailure(recorder, request, substitutionsHTTP.Failure{
		Status: http.StatusBadRequest, Code: "timetable.substitute_absent_on_date",
		Message: "Die Angaben für die Vertretung sind ungültig.", Err: errors.New("cause"),
		Details: map[string]any{"date": "2026-10-06"}, Field: "substitute_staff_id",
	})

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var body struct {
		Code    string         `json:"code"`
		Details map[string]any `json:"details"`
		Errors  []struct {
			Field string `json:"field"`
		} `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, "timetable.substitute_absent_on_date", body.Code)
	require.Equal(t, map[string]any{"date": "2026-10-06"}, body.Details)
	require.Len(t, body.Errors, 1)
	require.Equal(t, "substitute_staff_id", body.Errors[0].Field)
}
