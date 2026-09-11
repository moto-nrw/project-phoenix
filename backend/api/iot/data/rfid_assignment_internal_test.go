package data

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type tagAssignmentStub func(context.Context, string) (RFIDTagAssignmentResponse, error)

func (q tagAssignmentStub) LookupTagAssignment(ctx context.Context, tag string) (RFIDTagAssignmentResponse, error) {
	return q(ctx, tag)
}

// The assignment check answers a free bracelet without a failed-scan record;
// the data resource has no scan recorder at all since #2698.
func TestRFIDAssignmentCheckDoesNotReportFailedScan(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"free tag", nil, 200},
		{"lookup unavailable", errors.New("database unavailable"), 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resource := &Resource{
				runtime: testRuntime(),
				TagAssignments: tagAssignmentStub(func(_ context.Context, tag string) (RFIDTagAssignmentResponse, error) {
					assert.Equal(t, "a1:b2:c3:d4", tag)
					return RFIDTagAssignmentResponse{Assigned: false}, tc.err
				}),
			}
			req := httptest.NewRequest("GET", "/rfid/a1:b2:c3:d4", nil).WithContext(requestWithDeviceContext().Context())
			rr := httptest.NewRecorder()
			resource.Router().ServeHTTP(rr, req)
			assert.Equal(t, tc.status, rr.Code, rr.Body.String())
			if tc.status == 200 {
				assert.Contains(t, rr.Body.String(), `"assigned":false`)
			} else {
				assert.NotContains(t, rr.Body.String(), "database unavailable")
				assert.NotContains(t, rr.Body.String(), `"assigned":false`)
			}
		})
	}
}
