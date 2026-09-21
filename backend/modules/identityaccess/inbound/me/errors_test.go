package me_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/me"
	"github.com/stretchr/testify/assert"
)

func TestErrorRenderer_Unauthorized(t *testing.T) {
	t.Parallel()

	ucErr := &identityaccess.CallerError{Err: identityaccess.ErrCallerNotAuthenticated}
	renderer := me.ErrorRenderer(ucErr)
	// ErrorUnauthorized returns common.ErrResponse from api/common
	resp, ok := renderer.(interface {
		Render(http.ResponseWriter, *http.Request) error
	})
	assert.True(t, ok)
	assert.NotNil(t, resp)
}

func TestErrorRenderer_Forbidden(t *testing.T) {
	t.Parallel()

	ucErr := &identityaccess.CallerError{Err: identityaccess.ErrCallerNotAuthorized}
	renderer := me.ErrorRenderer(ucErr)
	resp, ok := renderer.(interface {
		Render(http.ResponseWriter, *http.Request) error
	})
	assert.True(t, ok)
	assert.NotNil(t, resp)
}

func TestErrorRenderer_NotFoundErrors(t *testing.T) {
	t.Parallel()

	// The old ErrNoActiveGroups row is gone: no production path returned that
	// sentinel and the caller context has no counterpart.
	tests := []struct {
		name    string
		baseErr error
	}{
		{"ErrCallerNotFound", identityaccess.ErrCallerNotFound},
		{"ErrCallerNotLinkedToPerson", identityaccess.ErrCallerNotLinkedToPerson},
		{"ErrCallerNotLinkedToStaff", identityaccess.ErrCallerNotLinkedToStaff},
		{"ErrCallerNotLinkedToTeacher", identityaccess.ErrCallerNotLinkedToTeacher},
		{"ErrCallerGroupNotFound", identityaccess.ErrCallerGroupNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ucErr := &identityaccess.CallerError{Err: tt.baseErr}
			renderer := me.ErrorRenderer(ucErr)
			resp, ok := renderer.(interface {
				Render(http.ResponseWriter, *http.Request) error
			})
			assert.True(t, ok)
			assert.NotNil(t, resp)
		})
	}
}

// The old TestErrorRenderer_BadRequest covered ErrInvalidOperation, which no
// production path returned; the caller context has no counterpart.

func TestErrorRenderer_UnknownUserContextError(t *testing.T) {
	t.Parallel()

	ucErr := &identityaccess.CallerError{Err: errors.New("unknown error")}
	renderer := me.ErrorRenderer(ucErr)
	resp, ok := renderer.(interface {
		Render(http.ResponseWriter, *http.Request) error
	})
	assert.True(t, ok)
	assert.NotNil(t, resp)
}

func TestErrorRenderer_NonUserContextError(t *testing.T) {
	t.Parallel()

	plainErr := errors.New("generic error")
	renderer := me.ErrorRenderer(plainErr)
	resp, ok := renderer.(interface {
		Render(http.ResponseWriter, *http.Request) error
	})
	assert.True(t, ok)
	assert.NotNil(t, resp)
}
