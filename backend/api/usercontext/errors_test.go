package usercontext_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/usercontext"
	usercontextSvc "github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/stretchr/testify/assert"
)

func TestErrorRenderer_Unauthorized(t *testing.T) {
	ucErr := &usercontextSvc.UserContextError{Err: usercontextSvc.ErrUserNotAuthenticated}
	renderer := usercontext.ErrorRenderer(ucErr)
	// ErrorUnauthorized returns common.ErrResponse from api/common
	resp, ok := renderer.(interface {
		Render(http.ResponseWriter, *http.Request) error
	})
	assert.True(t, ok)
	assert.NotNil(t, resp)
}

func TestErrorRenderer_Forbidden(t *testing.T) {
	ucErr := &usercontextSvc.UserContextError{Err: usercontextSvc.ErrUserNotAuthorized}
	renderer := usercontext.ErrorRenderer(ucErr)
	resp, ok := renderer.(interface {
		Render(http.ResponseWriter, *http.Request) error
	})
	assert.True(t, ok)
	assert.NotNil(t, resp)
}

func TestErrorRenderer_NotFoundErrors(t *testing.T) {
	tests := []struct {
		name    string
		baseErr error
	}{
		{"ErrUserNotFound", usercontextSvc.ErrUserNotFound},
		{"ErrUserNotLinkedToPerson", usercontextSvc.ErrUserNotLinkedToPerson},
		{"ErrUserNotLinkedToStaff", usercontextSvc.ErrUserNotLinkedToStaff},
		{"ErrUserNotLinkedToTeacher", usercontextSvc.ErrUserNotLinkedToTeacher},
		{"ErrGroupNotFound", usercontextSvc.ErrGroupNotFound},
		{"ErrNoActiveGroups", usercontextSvc.ErrNoActiveGroups},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ucErr := &usercontextSvc.UserContextError{Err: tt.baseErr}
			renderer := usercontext.ErrorRenderer(ucErr)
			resp, ok := renderer.(interface {
				Render(http.ResponseWriter, *http.Request) error
			})
			assert.True(t, ok)
			assert.NotNil(t, resp)
		})
	}
}

func TestErrorRenderer_BadRequest(t *testing.T) {
	ucErr := &usercontextSvc.UserContextError{Err: usercontextSvc.ErrInvalidOperation}
	renderer := usercontext.ErrorRenderer(ucErr)
	resp, ok := renderer.(interface {
		Render(http.ResponseWriter, *http.Request) error
	})
	assert.True(t, ok)
	assert.NotNil(t, resp)
}

func TestErrorRenderer_UnknownUserContextError(t *testing.T) {
	ucErr := &usercontextSvc.UserContextError{Err: errors.New("unknown error")}
	renderer := usercontext.ErrorRenderer(ucErr)
	resp, ok := renderer.(interface {
		Render(http.ResponseWriter, *http.Request) error
	})
	assert.True(t, ok)
	assert.NotNil(t, resp)
}

func TestErrorRenderer_NonUserContextError(t *testing.T) {
	plainErr := errors.New("generic error")
	renderer := usercontext.ErrorRenderer(plainErr)
	resp, ok := renderer.(interface {
		Render(http.ResponseWriter, *http.Request) error
	})
	assert.True(t, ok)
	assert.NotNil(t, resp)
}
