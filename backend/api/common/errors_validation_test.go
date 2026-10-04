package common_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
)

func renderedFieldErrors(t *testing.T, renderer render.Renderer) []common.FieldError {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/test", nil)
	require.NoError(t, render.Render(w, r, renderer))

	var body struct {
		Code   string              `json:"code"`
		Errors []common.FieldError `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, common.CodeGeneralInput, body.Code)
	return body.Errors
}

func TestErrorInvalidRequest_ValidationErrorsBecomeFieldErrors(t *testing.T) {
	t.Parallel()

	err := validation.Errors{
		"role_id": errors.New("cannot be blank"),
		"email":   errors.New("cannot be blank"),
		"qualifikationen": validation.Errors{
			"0": validation.Errors{"name": errors.New("cannot be blank")},
		},
	}

	fields := renderedFieldErrors(t, common.ErrorInvalidRequest(err))

	assert.Equal(t, []common.FieldError{
		{Field: "email", Reason: "cannot be blank"},
		{Field: "qualifikationen.0.name", Reason: "cannot be blank"},
		{Field: "role_id", Reason: "cannot be blank"},
	}, fields)
}

func TestErrorInvalidRequest_WrappedValidationErrorsBecomeFieldErrors(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("bind: %w", validation.Errors{"first_name": errors.New("cannot be blank")})

	fields := renderedFieldErrors(t, common.ErrorInvalidRequest(err))

	assert.Equal(t, []common.FieldError{{Field: "first_name", Reason: "cannot be blank"}}, fields)
}

func TestErrorInvalidRequest_PlainErrorHasNoFieldErrors(t *testing.T) {
	t.Parallel()

	fields := renderedFieldErrors(t, common.ErrorInvalidRequest(errors.New("malformed body")))

	assert.Empty(t, fields)
}

func TestErrorConflictOnField_NamesTheConflictingField(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/test", nil)
	renderer := common.ErrorConflictOnField(errors.New("email already exists"), common.CodeIdentityEmailAlreadyExists, "email")
	require.NoError(t, render.Render(w, r, renderer))

	var body struct {
		Code   string              `json:"code"`
		Errors []common.FieldError `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, 409, w.Code)
	assert.Equal(t, common.CodeIdentityEmailAlreadyExists, body.Code)
	assert.Equal(t, []common.FieldError{{Field: "email", Reason: "email already exists"}}, body.Errors)
}
