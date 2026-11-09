package checkin_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	checkinAPI "github.com/moto-nrw/project-phoenix/modules/devicescan/http/checkin"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestScanRecordCancellationPrecedesEndedCare(t *testing.T) {
	t.Parallel()
	for _, memo := range []bool{false, true} {
		t.Run(fmt.Sprintf("memo_%t", memo), func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			now := calendar.NewDate(2026, time.November, 6).BerlinMidnight().Add(12 * time.Hour)
			var cancel context.CancelFunc
			calls := 0
			workflowClock := func() time.Time {
				calls++
				if calls == 2 {
					cancel()
				}
				return now
			}
			module, err := api.NewCheckinWorkflowClockTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), workflowClock, func() time.Time { return now })
			require.NoError(t, err)
			k := &kiosk{db: db, resource: checkinAPI.NewResource(module.DeviceScan, testRuntime(), nil), rosters: module.Rosters}
			device, deviceID := k.deviceRow(t, "canceled-care")
			staff, _, _ := k.staff(t, "Canceled", "Care")
			tag, studentID, _ := k.studentCard(t, "Ended", "Care", "1a")
			ended := calendar.DateFromTime(now).AddDays(-1)
			testpkg.SetStudentLifecycle(t, db, studentID, testpkg.StudentStatusActive, nil, &ended)
			roomID, _, sessionID := k.roomWithSession(t, "Cancellation Room")
			testpkg.LinkDeviceToActiveGroup(t, db, sessionID, deviceID)
			testpkg.SetTestTenantSetting(t, db, testpkg.Tenant(t), "operations.presence_mode", `"detailed"`)
			router := testutil.NewTenantRouter(db)
			router.Mount("/", k.resource.Router())
			req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", checkinBody(tag, roomID), device, staff)
			ctx := req.Context()
			if memo {
				ctx = settings.WithSettingsRequestCache(ctx)
			}
			ctx, cancel = context.WithCancel(ctx)
			defer cancel()
			rr := testutil.ExecuteRequestForTest(t, router, req.WithContext(ctx))
			require.Equal(t, 2, calls, "cancel exactly at the visit recorder clock, after all session reads")
			require.Equal(t, 500, rr.Code, rr.Body.String())
			require.JSONEq(t, `{"status":500,"detail":"failed to create visit record","code":"general.server"}`, rr.Body.String())
			var visits int
			require.NoError(t, db.NewRaw("SELECT count(*) FROM active.visits WHERE student_id = ?", studentID).Scan(testpkg.Ctx(t), &visits))
			require.Zero(t, visits)
		})
	}
}
