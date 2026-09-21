package contracttest_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

var errApprovalProbe = errors.New("approval dependency unavailable")

type pickupApprovalFixture struct {
	input       carerequests.PickupApproval
	staffID     int64
	exceptionID int64
	guardianID  int64
	calls       []string
	fail        string
	completed   bool
	open        bool
	existing    *careplan.PickupException
	written     *careplan.PickupException
}

func (f *pickupApprovalFixture) step(name string) error {
	f.calls = append(f.calls, name)
	if f.fail == name {
		return errApprovalProbe
	}
	return nil
}
func (f *pickupApprovalFixture) EnrolledUntil(context.Context, int64) (*calendar.Date, error) {
	return nil, f.step("enrollment")
}
func (f *pickupApprovalFixture) ActingStaffID(context.Context) (int64, error) {
	return f.staffID, f.step("staff")
}
func (f *pickupApprovalFixture) LockStudentAndExceptionDay(context.Context, int64, string) error {
	return f.step("student-day-lock")
}
func (f *pickupApprovalFixture) LockStudentAttendance(context.Context, int64) error {
	return f.step("attendance-lock")
}
func (f *pickupApprovalFixture) AttendanceCompletion(context.Context, int64, calendar.Date) (bool, bool, error) {
	return f.open, f.completed, f.step("attendance")
}
func (f *pickupApprovalFixture) Preview(context.Context, int64, calendar.Date, time.Time) ([]carerequests.Block, error) {
	return nil, f.step("preview")
}
func (f *pickupApprovalFixture) Sync(context.Context, int64) error { return f.step("sync") }
func (f *pickupApprovalFixture) FindForDate(context.Context, int64, calendar.Date) (*careplan.PickupException, error) {
	return f.existing, f.step("exception")
}
func (f *pickupApprovalFixture) Create(_ context.Context, row careplan.PickupException) (int64, error) {
	f.written = &row
	return f.exceptionID, f.step("create")
}
func (f *pickupApprovalFixture) Update(_ context.Context, row careplan.PickupException) error {
	f.written = &row
	return f.step("update")
}
func (*pickupApprovalFixture) IsUniqueViolation(error) bool { return false }

func newPickupApprovalFixture(t *testing.T) *pickupApprovalFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Pickup", "Approval", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Pickup", "Reviewer")
	guardian := testpkg.CreateTestAccount(t, db, "parent")
	exception := testpkg.CreateTestPickupException(t, db, student.ID, calendar.Date("2026-09-21"), staff.ID, "14:30", "Termin")
	token := securityruntime.Fingerprint(carerequests.PickupImpactContent(nil))
	return &pickupApprovalFixture{staffID: staff.ID, exceptionID: exception.ID, guardianID: guardian.ID,
		input: carerequests.PickupApproval{
			TenantID: testpkg.Tenant(t), StudentID: student.ID, Payload: json.RawMessage(`{"date":"2026-09-21","pickup_time":"14:30","reason":"Termin"}`),
			ExpectedImpactToken: &token, RequireImpactToken: true,
		}}
}
func approvalService(t *testing.T, fixture *pickupApprovalFixture) carerequests.PickupApprovals {
	t.Helper()
	service, err := compose.NewPickupApprovals(compose.PickupApprovalDependencies{
		Fingerprint: securityruntime.Fingerprint,
		People:      fixture, Presence: fixture, Exceptions: fixture, Excusal: fixture, Locker: fixture,
		Today: func() calendar.Date { return "2026-09-21" },
	})
	require.NoError(t, err)
	return service
}

func TestNativePickupApprovalLocksBeforeWritingAndSyncing(t *testing.T) {
	t.Parallel()
	f := newPickupApprovalFixture(t)
	id, err := approvalService(t, f).ApplyPickupApproval(context.Background(), f.input)
	require.NoError(t, err)
	require.Equal(t, f.exceptionID, id)
	require.Equal(t, []string{"enrollment", "student-day-lock", "preview", "attendance-lock", "attendance", "staff", "exception", "create", "sync"}, f.calls)
	require.Equal(t, f.input.TenantID, f.written.TenantID)
	require.Equal(t, f.input.StudentID, f.written.StudentID)
	require.Equal(t, f.staffID, f.written.CreatedBy)
	require.Equal(t, careplan.ExceptionSourceStaff, f.written.Source)
	require.Equal(t, careplan.Date("2026-09-21"), f.written.ExceptionDate)
	require.Equal(t, "14:30", f.written.PickupTime.Format("15:04"))
}

func TestNativePickupApprovalRequiresImpactFingerprint(t *testing.T) {
	t.Parallel()
	fixture := &pickupApprovalFixture{}
	service, err := compose.NewPickupApprovals(compose.PickupApprovalDependencies{
		People: fixture, Presence: fixture, Exceptions: fixture, Excusal: fixture, Locker: fixture,
		Today: func() calendar.Date { return "2026-09-21" },
	})
	require.ErrorContains(t, err, "impact fingerprint")
	require.Nil(t, service)
}

func TestNativePickupApprovalRejectsMalformedStoredPayloadBeforeEffects(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		"{", "null", "[]",
		`{"date":"2026-09-21","pickup_time":1430,"reason":"Termin"}`,
		`{"date":"2026-09-21","pickup_time":"14:30","reason":null}`,
	} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			f := newPickupApprovalFixture(t)
			f.input.Payload = json.RawMessage(raw)
			_, err := approvalService(t, f).ApplyPickupApproval(testpkg.Ctx(t), f.input)
			require.ErrorIs(t, err, carerequests.ErrInvalidPayload)
			require.Empty(t, f.calls)
			require.Nil(t, f.written)
		})
	}
}

func TestNativePickupApprovalFailsClosedBeforeMutation(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"enrollment", "student-day-lock", "preview", "attendance-lock", "attendance", "staff", "exception"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			f := newPickupApprovalFixture(t)
			f.fail = stage
			_, err := approvalService(t, f).ApplyPickupApproval(context.Background(), f.input)
			require.ErrorIs(t, err, errApprovalProbe)
			require.Nil(t, f.written)
			require.NotContains(t, f.calls, "sync")
			require.Equal(t, stage, f.calls[len(f.calls)-1])
		})
	}
}

func TestNativePickupApprovalPreservesCheckoutAndManualExceptionGuards(t *testing.T) {
	t.Parallel()
	clock := time.Date(0, 1, 1, 13, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name    string
		fixture pickupApprovalFixture
		want    error
	}{
		{"completed", pickupApprovalFixture{completed: true}, carerequests.ErrPickupChangeAlreadyCompleted},
		{"staff exception", pickupApprovalFixture{existing: &careplan.PickupException{Source: careplan.ExceptionSourceStaff}}, carerequests.ErrPickupChangeConflict},
		{"manual partial absence", pickupApprovalFixture{existing: &careplan.PickupException{ExcusedFrom: &clock}}, carerequests.ErrPickupChangeConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPickupApprovalFixture(t)
			f.completed, f.existing = tc.fixture.completed, tc.fixture.existing
			_, err := approvalService(t, f).ApplyPickupApproval(context.Background(), f.input)
			require.ErrorIs(t, err, tc.want)
			require.Nil(t, f.written)
			require.NotContains(t, f.calls, "sync")
		})
	}
}

func TestNativePickupApprovalReclaimsGuardianExceptionWithoutLosingAutoExcusal(t *testing.T) {
	t.Parallel()
	f := newPickupApprovalFixture(t)
	guardian := f.guardianID
	clock := time.Date(0, 1, 1, 13, 0, 0, 0, time.UTC)
	f.open, f.completed = true, true
	f.existing = &careplan.PickupException{
		ID: f.exceptionID, TenantID: f.input.TenantID, StudentID: f.input.StudentID, ExceptionDate: "2026-09-21", Source: careplan.ExceptionSourceGuardian,
		CreatedByGuardian: &guardian, ExcusedAuto: true, ExcusedFrom: &clock,
	}
	id, err := approvalService(t, f).ApplyPickupApproval(context.Background(), f.input)
	require.NoError(t, err)
	require.Equal(t, f.exceptionID, id)
	require.Equal(t, f.staffID, f.written.CreatedBy)
	require.Nil(t, f.written.CreatedByGuardian)
	require.True(t, f.written.ExcusedAuto)
	require.Equal(t, calendar.NormalizeWallClock(clock), *f.written.ExcusedFrom)
	require.Contains(t, f.calls, "update")
	require.NotContains(t, f.calls, "create")
	require.Equal(t, "sync", f.calls[len(f.calls)-1])
}
