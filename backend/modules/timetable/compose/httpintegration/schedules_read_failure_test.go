package httpintegration_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	schedulesAPI "github.com/moto-nrw/project-phoenix/modules/timetable/compose/httpadapter"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
)

type failedScheduleRead struct {
	scheduleSvc.Service
	err error
}

func (s failedScheduleRead) GetDateframe(context.Context, int64) (*schedule.Dateframe, error) {
	return nil, &scheduleSvc.ScheduleError{Op: "get dateframe", Err: s.err}
}

func TestSchedulesReadFailureIsNotNotFound(t *testing.T) {
	t.Parallel()
	db, services := testutil.SetupScheduleModule(t)
	dateframe := testpkg.CreateTestDateframe(t, db, "read error contract", time.Now(), time.Now().AddDate(0, 0, 1))
	for _, tc := range []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{"failed read", context.Canceled, http.StatusInternalServerError, "context canceled"},
		{"missing row", scheduleSvc.ErrDateframeNotFound, http.StatusNotFound, "dateframe not found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resource := schedulesAPI.NewSchedulesResource(failedScheduleRead{Service: services.Schedule, err: tc.err}, db)
			request := testutil.NewRequest("GET", fmt.Sprintf("/dateframes/%d", dateframe.ID), nil)
			response := testutil.ExecuteWithAuth(t, resource.Router(), request, testutil.AdminTestClaims(1))
			assert.Equal(t, tc.status, response.Code)
			assert.Contains(t, response.Body.String(), tc.message)
		})
	}
}
