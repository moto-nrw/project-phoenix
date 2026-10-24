package careplanhttp

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"

	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingAbsenceNotifier struct {
	reports []studentpresence.AbsenceReport
}

func (n *recordingAbsenceNotifier) NotifyAbsenceReported(_ context.Context, report studentpresence.AbsenceReport) error {
	n.reports = append(n.reports, report)
	return nil
}

func TestStaffManualStatusAbsenceNotificationCallback(t *testing.T) {
	t.Parallel()
	tenantID := testpkg.UniqueTestTenantID(t)
	const actorID = 23
	today := calendar.NewDate(2026, 8, 24)
	t.Run("manual status change enqueues before commit", func(t *testing.T) {
		notifier := &recordingAbsenceNotifier{}
		resource := testResource(ResourceConfig{NotifyAbsenceReported: func(ctx context.Context, tenantID int64, ids []int64, status string, dates []calendar.Date, fromParent bool, actorID int64) error {
			return notifier.NotifyAbsenceReported(ctx, studentpresence.AbsenceReport{TenantID: tenantID, StudentIDs: ids, Status: status, Dates: dates, FromParent: fromParent, ActorAccountID: actorID})
		}})
		baseCtx := tenant.WithTenantID(context.Background(), tenantID)
		baseCtx = context.WithValue(baseCtx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: actorID})
		ctx, commit := tenant.WithAfterCommitHooksForTest(baseCtx)
		excused := true

		require.NoError(t, resource.scheduleStudentUpdateWakes(
			ctx,
			tenantID,
			41,
			&UpdateStudentRequest{Excused: &excused},
			false,
			absencerecords.StudentStatusDayExcused,
			today,
		))
		require.Len(t, notifier.reports, 1)
		commit()

		require.Len(t, notifier.reports, 1)
		assert.Equal(t, absencerecords.StudentStatusDayExcused, notifier.reports[0].Status)
		assert.Equal(t, int64(actorID), notifier.reports[0].ActorAccountID)
	})
}
