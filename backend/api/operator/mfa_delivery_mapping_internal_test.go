package operator

import (
	"net/http"
	"net/http/httptest"
	"testing"

	identityoperator "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"

	"github.com/stretchr/testify/assert"
)

func TestMapOperatorMFAErrorDeliveryUnavailableWireContract(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/operator/mfa/resend", nil)

	mapOperatorMFAError(recorder, request, identityoperator.ErrMFAStatusUnavailable)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Equal(t,
		"{\"status\":\"Service Unavailable\",\"message\":\"MFA ist gerade nicht verfügbar. Bitte versuchen Sie es erneut.\"}\n",
		recorder.Body.String(),
	)
}
