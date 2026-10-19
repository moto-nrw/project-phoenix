package devices

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/stretchr/testify/assert"
)

func TestRenderErrorPreservesOuterWrapperDispatch(t *testing.T) {
	t.Parallel()
	native := &devicefleet.AdministrationError{Op: "find", Err: devicefleet.ErrAdministrationDeviceNotFound}
	for _, tc := range []struct {
		name    string
		failure error
		status  int
	}{
		{"owner failure", native, http.StatusNotFound},
		{"wrapped failure", fmt.Errorf("lookup: %w", native), http.StatusInternalServerError},
		{"joined failure", errors.Join(native, errors.New("transaction failed")), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rs := &Resource{runtime: Runtime{Failure: func(w http.ResponseWriter, _ *http.Request, status int, err error, _ string) {
				assert.ErrorIs(t, err, tc.failure)
				w.WriteHeader(status)
			}}}
			w := httptest.NewRecorder()
			rs.renderError(w, httptest.NewRequest(http.MethodGet, "/", nil), tc.failure)
			assert.Equal(t, tc.status, w.Code)
		})
	}
}
