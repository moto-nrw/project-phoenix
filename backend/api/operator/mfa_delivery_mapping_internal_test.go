package operator

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

func TestMapOperatorMFAErrorDeliveryUnavailableWireContract(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/operator/mfa/resend", nil)

	mapOperatorMFAError(recorder, request, authService.ErrMFAStatusUnavailable)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Equal(t,
		"{\"status\":\"Service Unavailable\",\"message\":\"MFA status temporarily unavailable, please retry\"}\n",
		recorder.Body.String(),
	)
}
