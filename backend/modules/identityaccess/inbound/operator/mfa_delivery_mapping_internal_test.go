package operator

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapOperatorMFAErrorDeliveryUnavailableWireContract(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/operator/mfa/resend", nil)

	mapOperatorMFAError(recorder, request, ErrMFAStatusUnavailable)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Equal(t,
		"{\"status\":\"Service Unavailable\",\"message\":\"MFA ist gerade nicht verfügbar. Bitte versuchen Sie es erneut.\",\"type\":\"https://moto-app.de/help/fehlermeldungen#anleitung-gerade-nicht-erreichbar\",\"title\":\"Service Unavailable\",\"detail\":\"MFA ist gerade nicht verfügbar. Bitte versuchen Sie es erneut.\",\"instance\":\"\",\"code\":\"general.unavailable\"}\n",
		recorder.Body.String(),
	)
}
