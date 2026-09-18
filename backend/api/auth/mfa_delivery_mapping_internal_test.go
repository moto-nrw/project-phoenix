package auth

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
		"{\"status\":\"error\",\"error\":\"mfa status unavailable, please retry\"}\n",
		recorder.Body.String(),
	)
}
