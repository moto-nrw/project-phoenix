package users

import (
	"errors"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorRenderer_PersonNotFound(t *testing.T) {
	t.Parallel()

	err := &usersSvc.UsersError{
		Op:  "GetPerson",
		Err: usersSvc.ErrPersonNotFound,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "person not found")
}

func TestErrorRenderer_AccountNotFound(t *testing.T) {
	t.Parallel()

	err := &usersSvc.UsersError{
		Op:  "GetAccount",
		Err: usersSvc.ErrAccountNotFound,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "account not found")
}

func TestErrorRenderer_RFIDCardNotFound(t *testing.T) {
	t.Parallel()

	err := &usersSvc.UsersError{
		Op:  "GetRFIDCard",
		Err: usersSvc.ErrRFIDCardNotFound,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "RFID card not found")
}

func TestErrorRenderer_AccountAlreadyLinked(t *testing.T) {
	t.Parallel()

	err := &usersSvc.UsersError{
		Op:  "LinkAccount",
		Err: usersSvc.ErrAccountAlreadyLinked,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusConflict, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "account is already linked")
}

func TestErrorRenderer_RFIDCardAlreadyLinked(t *testing.T) {
	t.Parallel()

	err := &usersSvc.UsersError{
		Op:  "LinkRFIDCard",
		Err: usersSvc.ErrRFIDCardAlreadyLinked,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusConflict, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "RFID card is already linked")
}

func TestErrorRenderer_PersonIdentifierRequired(t *testing.T) {
	t.Parallel()

	err := &usersSvc.UsersError{
		Op:  "FindPerson",
		Err: usersSvc.ErrPersonIdentifierRequired,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "either tag ID or account ID is required")
}

func TestErrorRenderer_GuardianNotFound(t *testing.T) {
	t.Parallel()

	// GuardianNotFound is not explicitly mapped, should fall to default case
	err := &usersSvc.UsersError{
		Op:  "GetGuardian",
		Err: errors.New("guardian not found"),
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "guardian not found")
}

func TestErrorRenderer_StaffNotFound(t *testing.T) {
	t.Parallel()

	// StaffNotFound is not explicitly mapped, should fall to default case
	err := &usersSvc.UsersError{
		Op:  "GetStaff",
		Err: errors.New("staff member not found"),
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "staff member not found")
}

func TestErrorRenderer_TeacherNotFound(t *testing.T) {
	t.Parallel()

	// TeacherNotFound is not explicitly mapped, should fall to default case
	err := &usersSvc.UsersError{
		Op:  "GetTeacher",
		Err: usersSvc.ErrTeacherNotFound,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "teacher not found")
}

func TestErrorRenderer_StaffAlreadyExists(t *testing.T) {
	t.Parallel()

	// StaffAlreadyExists is not explicitly mapped, should fall to default case
	err := &usersSvc.UsersError{
		Op:  "CreateStaff",
		Err: errors.New("staff member already exists for this person"),
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "staff member already exists")
}

func TestErrorRenderer_TeacherAlreadyExists(t *testing.T) {
	t.Parallel()

	// TeacherAlreadyExists is not explicitly mapped, should fall to default case
	err := &usersSvc.UsersError{
		Op:  "CreateTeacher",
		Err: errors.New("teacher already exists for this staff member"),
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "teacher already exists")
}

func TestErrorRenderer_InvalidPIN(t *testing.T) {
	t.Parallel()

	// InvalidPIN is not explicitly mapped, should fall to default case
	err := &usersSvc.UsersError{
		Op:  "ValidatePIN",
		Err: errors.New("invalid staff PIN"),
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "invalid staff PIN")
}

func TestErrorRenderer_NonUsersError(t *testing.T) {
	t.Parallel()

	// Non-UsersError should be treated as internal server error
	err := errors.New("some random error")

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "some random error")
}

func TestErrorRenderer_UnknownUsersError(t *testing.T) {
	t.Parallel()

	// UsersError with unknown underlying error should fall to default case
	unknownErr := errors.New("unknown users error")
	err := &usersSvc.UsersError{
		Op:  "UnknownOperation",
		Err: unknownErr,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "unknown users error")
}
