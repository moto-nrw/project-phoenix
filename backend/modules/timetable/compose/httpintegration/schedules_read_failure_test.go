package httpintegration_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schedulesAPI "github.com/moto-nrw/project-phoenix/modules/timetable/compose/httpadapter"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
)

type failedDateframeRead struct {
	schedulesAPI.Dateframes
	err error
}

func (s failedDateframeRead) FindDateframe(context.Context, int64) (schoolcalendar.Dateframe, error) {
	return schoolcalendar.Dateframe{}, fmt.Errorf("get dateframe: %w", s.err)
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
		{"missing row", schoolcalendar.ErrDateframeNotFound, http.StatusNotFound, "dateframe not found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resource := schedulesAPI.NewSchedulesResource(failedDateframeRead{Dateframes: services.Calendar, err: tc.err}, services.Timetable, services.TimeframeGuard, db)
			request := testutil.NewRequest("GET", fmt.Sprintf("/dateframes/%d", dateframe.ID), nil)
			response := testutil.ExecuteWithAuth(t, resource.Router(), request, testutil.AdminTestClaims(1))
			assert.Equal(t, tc.status, response.Code)
			assert.Contains(t, response.Body.String(), tc.message)
		})
	}
}
