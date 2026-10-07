package common_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
)

type rejectedValue struct{ code, field string }

func (r rejectedValue) Error() string      { return "value rejected" }
func (r rejectedValue) ErrorCode() string  { return r.code }
func (r rejectedValue) ErrorField() string { return r.field }

func renderedBody(t *testing.T, _ any, w *httptest.ResponseRecorder) common.ErrResponse {
	t.Helper()
	var body common.ErrResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	return body
}

func TestErrorInvalidRequest_TakesCodeAndFieldFromRejection(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("context: %w", rejectedValue{common.CodeGeneralInput, "children.0.first_name"})
	w := httptest.NewRecorder()
	common.RenderError(w, httptest.NewRequest(http.MethodPost, "/x", nil), common.ErrorInvalidRequest(err))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := renderedBody(t, nil, w)
	assert.Equal(t, common.CodeGeneralInput, body.Code)
	assert.Equal(t, []common.FieldError{{Field: "children.0.first_name", Reason: "value rejected"}}, body.Errors)
}

func TestErrorInvalidRequestWithCode_ExplicitCodeWins(t *testing.T) {
	t.Parallel()

	err := rejectedValue{common.CodeGeneralInput, "name"}
	w := httptest.NewRecorder()
	common.RenderError(w, httptest.NewRequest(http.MethodPost, "/x", nil), common.ErrorInvalidRequestWithCode(err, common.CodeGeneralBusinessRejection))

	body := renderedBody(t, nil, w)
	assert.Equal(t, common.CodeGeneralBusinessRejection, body.Code)
	assert.Equal(t, "name", body.Errors[0].Field)
}

func TestRenderError_RejectedValueIsNeverAServerError(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("parse: %w", rejectedValue{common.CodeGeneralInput, "date_of_birth"})
	w := httptest.NewRecorder()
	common.RenderError(w, httptest.NewRequest(http.MethodPost, "/x", nil), common.ErrorInternalServer(err))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := renderedBody(t, nil, w)
	assert.Equal(t, common.CodeGeneralInput, body.Code)
	assert.Equal(t, "date_of_birth", body.Errors[0].Field)
	assert.True(t, common.HasInputRejection(err))
	assert.False(t, common.HasInputRejection(errors.New("plain")))
}
