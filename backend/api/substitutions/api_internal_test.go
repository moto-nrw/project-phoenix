package substitutions

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderModuleErrorUsesStableContract(t *testing.T) {
	t.Parallel()

	for _, spec := range moduleErrorSpecs {
		t.Run(spec.code, func(t *testing.T) {
			t.Parallel()
			response := renderErrorResponse(t, fmt.Errorf("wrapped: %w", spec.target))
			require.Equal(t, spec.status, response.status)
			require.Equal(t, spec.code, response.body.Code)
			require.Equal(t, spec.message, response.body.Error)
		})
	}
}

func TestRenderModuleErrorHidesInternalCause(t *testing.T) {
	t.Parallel()

	response := renderErrorResponse(t, errors.New("postgres password leaked"))
	require.Equal(t, 500, response.status)
	require.Equal(t, "internal", response.body.Code)
	require.Equal(t, "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.", response.body.Error)
	require.NotContains(t, response.rawBody, "postgres")
}

type renderedError struct {
	status  int
	rawBody string
	body    struct {
		Status string `json:"status"`
		Error  string `json:"error"`
		Code   string `json:"code"`
	}
}

func renderErrorResponse(t *testing.T, err error) renderedError {
	t.Helper()
	request := httptest.NewRequest("GET", "/api/substitutions", nil)
	recorder := httptest.NewRecorder()
	renderModuleError(recorder, request, err)
	response := renderedError{status: recorder.Code, rawBody: recorder.Body.String()}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response.body))
	return response
}
