package checkin_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	checkinAPI "github.com/moto-nrw/project-phoenix/api/iot/checkin"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/device"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/models/users"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/services/users/userstest"
	"github.com/stretchr/testify/assert"
)

type attendanceScanRecorder struct {
	tags []string
}

func (s *attendanceScanRecorder) Record(_ context.Context, tag string, _ *int64) error {
	s.tags = append(s.tags, tag)
	return nil
}

func TestAttendanceRFIDLookupDistinguishesMissingTagFromFailure(t *testing.T) {
	t.Parallel()
	for _, route := range []struct{ name, method, path, body string }{
		{"status", http.MethodGet, "/status/a1:b2:c3:d4", ""},
		{"toggle", http.MethodPost, "/toggle", `{"rfid":"a1:b2:c3:d4","action":"confirm"}`},
		{"daily checkout", http.MethodPost, "/toggle", `{"rfid":"a1:b2:c3:d4","action":"confirm_daily_checkout","destination":"zuhause"}`},
	} {
		for _, tc := range []struct {
			name            string
			err             error
			status, records int
		}{
			{"missing", &usersSvc.UsersError{Op: "find person by tag ID", Err: usersSvc.ErrPersonNotFound}, http.StatusNotFound, 1},
			{"unavailable", errors.New("database unavailable"), http.StatusInternalServerError, 0},
		} {
			t.Run(route.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				scans := &attendanceScanRecorder{}
				resource := &checkinAPI.AttendanceResource{
					UsersService: &userstest.PersonServiceMock{FindByTagIDFn: func(_ context.Context, tag string) (*users.Person, error) {
						assert.Equal(t, "A1B2C3D4", tag)
						return nil, tc.err
					}},
					UnregisteredTagScans: scans,
				}
				req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
				req.Header.Set("Content-Type", "application/json")
				req = req.WithContext(context.WithValue(req.Context(), device.CtxDevice, testutil.DevicePrincipal(&iotModels.Device{DeviceID: "test-reader"})))
				rr := httptest.NewRecorder()
				resource.Router().ServeHTTP(rr, req)
				assert.Equal(t, tc.status, rr.Code, rr.Body.String())
				assert.Len(t, scans.tags, tc.records)
				if tc.records > 0 {
					assert.Equal(t, []string{"A1B2C3D4"}, scans.tags)
					assert.Contains(t, rr.Body.String(), "RFID tag not found")
				} else {
					assert.NotContains(t, rr.Body.String(), "RFID tag not found")
					assert.NotContains(t, rr.Body.String(), "database unavailable")
				}
			})
		}
	}
}
