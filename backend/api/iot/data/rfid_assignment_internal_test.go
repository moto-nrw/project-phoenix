package data

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/users"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/services/users/userstest"
	"github.com/stretchr/testify/assert"
)

// The assignment check answers a free bracelet without a failed-scan record;
// the data resource has no scan recorder at all since #2698.
func TestRFIDAssignmentCheckDoesNotReportFailedScan(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"free tag", &usersSvc.UsersError{Op: "find person by tag ID", Err: usersSvc.ErrPersonNotFound}, http.StatusOK},
		{"lookup unavailable", errors.New("database unavailable"), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resource := &Resource{
				UsersService: &userstest.PersonServiceMock{FindByTagIDFn: func(_ context.Context, tag string) (*users.Person, error) {
					assert.Equal(t, "A1B2C3D4", tag)
					return nil, tc.err
				}},
			}
			req := httptest.NewRequest(http.MethodGet, "/rfid/a1:b2:c3:d4", nil).WithContext(requestWithDeviceContext().Context())
			rr := httptest.NewRecorder()
			resource.Router().ServeHTTP(rr, req)
			assert.Equal(t, tc.status, rr.Code, rr.Body.String())
			if tc.status == http.StatusOK {
				assert.Contains(t, rr.Body.String(), `"assigned":false`)
			} else {
				assert.NotContains(t, rr.Body.String(), "database unavailable")
				assert.NotContains(t, rr.Body.String(), `"assigned":false`)
			}
		})
	}
}
