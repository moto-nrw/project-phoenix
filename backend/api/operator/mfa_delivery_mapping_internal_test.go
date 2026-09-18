package operator

import (
	"net/http"
	"net/http/httptest"
	"testing"

	authSvc "github.com/moto-nrw/project-phoenix/services/auth"

	"github.com/stretchr/testify/assert"
)

func TestMapOperatorMFAErrorDeliveryUnavailableWireContract(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/operator/mfa/resend", nil)

	mapOperatorMFAError(recorder, request, authSvc.ErrMFAStatusUnavailable)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Equal(t,
		"{\"status\":\"Service Unavailable\",\"message\":\"MFA ist gerade nicht verfügbar. Bitte versuchen Sie es erneut.\"}\n",
		recorder.Body.String(),
	)
}
