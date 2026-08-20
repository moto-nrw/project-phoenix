package operator

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// TestMapOperatorMFAError_RemainingCases fills the two branches the
// existing TestMapOperatorMFAError_StatusCodes table did not exercise:
// the permission-denied 403 path and the unknown-error 500 fallback.
func TestMapOperatorMFAError_RemainingCases(t *testing.T) {
	t.Parallel()

	t.Run("permission_denied_maps_to_403", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		mapOperatorMFAError(rr, req, authService.ErrMFAPermissionDenied)
		assert.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("unrecognised_error_maps_to_500", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		mapOperatorMFAError(rr, req, errors.New("brand-new failure mode"))
		assert.Equal(t, http.StatusInternalServerError, rr.Code,
			"unknown errors must fall through to a 500 — never leak internals to the client as 4xx")
	})
}
