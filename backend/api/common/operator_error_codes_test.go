package common_test

import (
	"errors"
	"net/http"
	"testing"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type operatorTestRejection struct{ error }

func (operatorTestRejection) ErrorCode() string  { return common.CodeIdentityPasswordTooWeak }
func (operatorTestRejection) ErrorField() string { return "password" }

// #2519: an operator input error keeps the code of an InputRejection and the
// fields of a validation error, so the operator portal marks the field.
func TestOperatorInvalidRequestCarriesRejectionCodeAndFields(t *testing.T) {
	t.Parallel()

	rejected, ok := common.OperatorInvalidRequest(operatorTestRejection{errors.New("weak")}).(*common.ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, rejected.HTTPStatusCode)
	assert.Equal(t, common.CodeIdentityPasswordTooWeak, rejected.Code)
	assert.Equal(t, []common.FieldError{{Field: "password", Reason: "weak"}}, rejected.Errors)

	invalid, ok := common.OperatorInvalidRequest(validation.Errors{"name": errors.New("cannot be blank")}).(*common.ErrResponse)
	require.True(t, ok)
	assert.Equal(t, []common.FieldError{{Field: "name", Reason: "cannot be blank"}}, invalid.Errors)
}

// OperatorInvalidInput keeps a cause's adapter text out of the answer.
func TestOperatorInvalidInputHidesCauseText(t *testing.T) {
	t.Parallel()

	resp, ok := common.OperatorInvalidInput(errors.New("pq: secret"), "invalid input data").(*common.ErrResponse)
	require.True(t, ok)
	assert.Equal(t, "invalid input data", resp.ErrorText)
	assert.Empty(t, resp.Errors)
}
