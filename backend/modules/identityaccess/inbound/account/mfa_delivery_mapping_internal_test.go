package account

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"

	"github.com/stretchr/testify/assert"
)

func TestMapMFAErrorDeliveryUnavailableWireContract(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/auth/mfa/resend", nil)

	mapMFAError(recorder, request, identityaccess.ErrMFAStatusUnavailable)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Equal(t,
		"{\"type\":\"https://moto-app.de/help/fehlermeldungen#anleitung-gerade-nicht-erreichbar\",\"title\":\"Service Unavailable\",\"status\":503,\"detail\":\"mfa status unavailable, please retry\",\"instance\":\"\",\"code\":\"general.unavailable\"}\n",
		recorder.Body.String(),
	)
}
