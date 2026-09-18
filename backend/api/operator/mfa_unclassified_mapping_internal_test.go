package operator

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	identityoperator "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/operator"
)

// The operator second factor reports the operator not-found outcome for an
// unknown operator, and the surface deliberately does not classify it: the
// enrollment endpoints are reached with a signed enrollment token, so a
// missing operator is a server-side inconsistency, not a client error. It
// keeps rendering the stable 500 the retained port produced (#3364).
func TestMapOperatorMFAErrorUnclassifiedOutcomeWireContract(t *testing.T) {
	t.Parallel()

	for name, err := range map[string]error{
		"operator not found": identityoperator.ErrOperatorNotFound,
		"unknown failure":    errors.New("boom"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/operator/auth/mfa/enroll/start", nil)

			mapOperatorMFAError(recorder, request, err)

			assert.Equal(t, http.StatusInternalServerError, recorder.Code)
			assert.Equal(t,
				"{\"status\":\"error\",\"message\":\"MFA operation failed\"}\n",
				recorder.Body.String(),
			)
		})
	}
}
