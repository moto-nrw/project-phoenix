package studentpresencehttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
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

func TestStaffAbsenceNotificationCallbacks(t *testing.T) {
	t.Parallel()

	tenantID := testpkg.UniqueTestTenantID(t)
	const actorID = 23
	today := calendar.NewDate(2026, 8, 24)

	t.Run("planned status write keeps the whole submission together", func(t *testing.T) {
		notifier := &recordingAbsenceNotifier{}
		resource := &Resource{ResourceConfig: ResourceConfig{
			AbsenceNotifier: notifier}}
		ctx := tenant.WithTenantID(context.Background(), tenantID)
		ctx = context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: actorID})
		req := httptest.NewRequest(http.MethodPost, "/status-days", nil).WithContext(ctx)

		writeContext := resource.newStatusDayCreateWriteContext(
			req,
			[]string{"users:update"},
			absencerecords.StudentStatusDaySick,
			[]calendar.Date{today},
		)
		require.NoError(t, writeContext.AfterCreate(ctx, []int64{41, 42}))

		require.Len(t, notifier.reports, 1)
		report := notifier.reports[0]
		assert.Equal(t, tenantID, report.TenantID)
		assert.Equal(t, []int64{41, 42}, report.StudentIDs)
		assert.Equal(t, absencerecords.StudentStatusDaySick, report.Status)
		assert.Equal(t, []calendar.Date{today}, report.Dates)
		assert.False(t, report.FromParent)
		assert.Equal(t, int64(actorID), report.ActorAccountID)
	})

}

func TestStatusDayDateHelpers(t *testing.T) {
	t.Parallel()

	dates, err := parseStatusDayDates([]string{"2026-05-25", "2026-05-27"})
	require.NoError(t, err)
	require.Len(t, dates, 2)
	assert.Equal(t, calendar.NewDate(2026, 5, 25), dates[0])
	assert.Equal(t, calendar.NewDate(2026, 5, 27), dates[1])

	_, err = parseStatusDayDates([]string{"2026-05-25", "broken"})
	require.ErrorContains(t, err, "invalid date format")
}

func TestCreateStudentStatusDaysRequestRejectsRangeOverOneYear(t *testing.T) {
	t.Parallel()

	req := &CreateStudentStatusDaysRequest{
		Status: absencerecords.StudentStatusDaySick,
		Dates:  []string{"2026-01-01", "2027-01-02"},
	}

	err := req.Bind(nil)

	require.ErrorContains(t, err, "cannot span more than 366 days")
}

func TestApplyAndClearLiveStatusForToday(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 25, 9, 30, 0, 0, time.UTC)
	student := &studentpresence.StudentRecord{}

	studentpresence.ApplyLiveStatusForToday(student, absencerecords.StudentStatusDaySick, now)
	require.NotNil(t, student.Sick)
	require.NotNil(t, student.SickSince)
	require.NotNil(t, student.Excused)
	assert.True(t, *student.Sick)
	assert.False(t, *student.Excused)
	assert.Equal(t, now, *student.SickSince)
	assert.Nil(t, student.ExcusedSince)

	studentpresence.ApplyLiveStatusForToday(student, absencerecords.StudentStatusDayExcused, now.Add(time.Hour))
	require.NotNil(t, student.Excused)
	require.NotNil(t, student.ExcusedSince)
	require.NotNil(t, student.Sick)
	assert.True(t, *student.Excused)
	assert.False(t, *student.Sick)
	assert.Nil(t, student.SickSince)

	studentpresence.ClearLiveStatusForToday(student, absencerecords.StudentStatusDayExcused)
	require.NotNil(t, student.Excused)
	assert.False(t, *student.Excused)
	assert.Nil(t, student.ExcusedSince)

	studentpresence.ApplyLiveStatusForToday(student, absencerecords.StudentStatusDaySick, now)
	studentpresence.ClearLiveStatusForToday(student, absencerecords.StudentStatusDaySick)
	require.NotNil(t, student.Sick)
	assert.False(t, *student.Sick)
	assert.Nil(t, student.SickSince)

	studentpresence.ApplyLiveStatusForToday(student, absencerecords.StudentStatusDaySick, now)
	studentpresence.ApplyLiveStatusForToday(student, absencerecords.StudentStatusDayClassTrip, now.Add(2*time.Hour))
	require.NotNil(t, student.Sick)
	require.NotNil(t, student.Excused)
	assert.False(t, *student.Sick)
	assert.False(t, *student.Excused)
	assert.Nil(t, student.SickSince)
	assert.Nil(t, student.ExcusedSince)
}

// TestAbsenceStatusesAreDeliveryStatuses pins the status-day values this
// route hands to Delivery's absence notification against the statuses
// Delivery announces. Delivery declares them itself and names no Care Plan
// package (#4072); a rename on either side would silently stop the
// notification.
func TestAbsenceStatusesAreDeliveryStatuses(t *testing.T) {
	t.Parallel()

	assert.Equal(t, absencerecords.StudentStatusDaySick, "sick")
	assert.Equal(t, absencerecords.StudentStatusDayExcused, "excused")
}
