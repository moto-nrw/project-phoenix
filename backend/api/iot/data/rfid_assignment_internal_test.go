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

type assignmentScanRecorder struct {
	calls int
}

func (s *assignmentScanRecorder) Record(context.Context, string, *int64) error {
	s.calls++
	return nil
}

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
			scans := &assignmentScanRecorder{}
			resource := &Resource{
				UsersService: &userstest.PersonServiceMock{FindByTagIDFn: func(_ context.Context, tag string) (*users.Person, error) {
					assert.Equal(t, "A1B2C3D4", tag)
					return nil, tc.err
				}},
				UnregisteredTagScans: scans,
			}
			req := httptest.NewRequest(http.MethodGet, "/rfid/a1:b2:c3:d4", nil).WithContext(requestWithDeviceContext().Context())
			rr := httptest.NewRecorder()
			resource.Router().ServeHTTP(rr, req)
			assert.Equal(t, tc.status, rr.Code, rr.Body.String())
			assert.Zero(t, scans.calls, "checking a free bracelet during assignment is not a failed attendance scan")
			if tc.status == http.StatusOK {
				assert.Contains(t, rr.Body.String(), `"assigned":false`)
			} else {
				assert.NotContains(t, rr.Body.String(), "database unavailable")
				assert.NotContains(t, rr.Body.String(), `"assigned":false`)
			}
		})
	}
}
