package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
)

func attendanceState() *ports.AttendanceState {
	checkIn := fixedNow.Add(-2 * time.Hour)
	return &ports.AttendanceState{Status: "checked_in", Date: timezone.NewDate(2026, 8, 5), CheckInTime: &checkIn, CheckedInBy: "Staff Member"}
}

func TestAttendanceStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("answers the state with the group", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.people.students[testPersonID].GroupID = ptr(int64(3))
		h.groups.group = &ports.Group{ID: 3, Name: "Test Class 1a"}
		h.attendance.state = attendanceState()

		status, err := h.service().AttendanceStatus(ctx, testTag)

		require.NoError(t, err)
		assert.Equal(t, devicescan.AttendanceStudent{ID: testStudentID, FirstName: "Max", LastName: "Muster", Group: &devicescan.AttendanceGroup{ID: 3, Name: "Test Class 1a"}}, status.Student)
		assert.Equal(t, "checked_in", status.Attendance.Status)
		assert.Equal(t, "2026-08-05", status.Attendance.Date)
		assert.Equal(t, "Staff Member", status.Attendance.CheckedInBy)
	})
	t.Run("a group lookup failure only drops the group", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.people.students[testPersonID].GroupID = ptr(int64(3))
		h.groups.err = errBoom
		h.attendance.state = attendanceState()

		status, err := h.service().AttendanceStatus(ctx, testTag)

		require.NoError(t, err)
		assert.Nil(t, status.Student.Group)
	})
	t.Run("an empty tag is invalid", func(t *testing.T) {
		t.Parallel()
		_, err := newHarness(t).service().AttendanceStatus(ctx, "")
		requireFailure(t, err, devicescan.FailureInvalidRequest, devicescan.MessageRFIDParameterRequired)
	})
	t.Run("a staff card is not a student", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		delete(h.people.students, testPersonID)
		_, err := h.service().AttendanceStatus(ctx, testTag)
		requireFailure(t, err, devicescan.FailureNotFound, devicescan.MessagePersonNotStudent)
	})
	t.Run("an alumnus is not a student", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.people.students[testPersonID].Alumnus = true
		_, err := h.service().AttendanceStatus(ctx, testTag)
		requireFailure(t, err, devicescan.FailureNotFound, devicescan.MessagePersonNotStudent)
	})
	t.Run("a status lookup failure is a server fault", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.attendance.statusErr = errors.New("active: GetStudentAttendanceStatus: database operation failed")
		_, err := h.service().AttendanceStatus(ctx, testTag)
		requireFailure(t, err, devicescan.FailureInternal, "active: GetStudentAttendanceStatus: database operation failed")
	})
	t.Run("requires a device", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.principals.device = nil
		_, err := h.service().AttendanceStatus(ctx, testTag)
		require.ErrorIs(t, err, devicescan.ErrDeviceUnauthorized)
	})
}

// The three attendance routes distinguish a missing card (404 plus an
// unregistered-scan record) from a lookup failure (500, no record).
func TestAttendanceRFIDLookupDistinguishesMissingTagFromFailure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	routes := map[string]func(s *Service) error{
		"status": func(s *Service) error { _, err := s.AttendanceStatus(ctx, "UNKNOWN"); return err },
		"toggle": func(s *Service) error {
			_, err := s.ToggleAttendance(ctx, devicescan.AttendanceToggleCommand{RFIDTag: "UNKNOWN", Action: devicescan.AttendanceActionConfirm})
			return err
		},
		"daily checkout": func(s *Service) error {
			_, err := s.ToggleAttendance(ctx, devicescan.AttendanceToggleCommand{RFIDTag: "UNKNOWN", Action: devicescan.AttendanceActionDailyCheckout, Destination: devicescan.DestinationHome})
			return err
		},
	}
	for name, call := range routes {
		t.Run(name+"/missing", func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			err := call(h.service())
			failure := classifiedFailure(t, err, devicescan.FailureNotFound, devicescan.MessageRFIDTagNotFound)
			assert.Empty(t, failure.Code, "the attendance routes carry no code")
			require.Len(t, h.fleet.scans, 1)
			assert.Equal(t, "UNKNOWN", h.fleet.scans[0].TagUID)
		})
		t.Run(name+"/unavailable", func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			h.people.personErr = errors.New("database unavailable")
			err := call(h.service())
			requireFailure(t, err, devicescan.FailureInternal, devicescan.MessageInternalServerError)
			assert.Empty(t, h.fleet.scans)
		})
	}
}

func TestToggleAttendance(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	confirm := devicescan.AttendanceToggleCommand{RFIDTag: testTag, Action: devicescan.AttendanceActionConfirm}

	t.Run("cancel answers without a lookup", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.people.personErr = errBoom

		result, err := h.service().ToggleAttendance(ctx, devicescan.AttendanceToggleCommand{RFIDTag: "ANY", Action: devicescan.AttendanceActionCancel})

		require.NoError(t, err)
		assert.Equal(t, devicescan.AttendanceActionCancelled, result.Action)
		assert.Equal(t, "Attendance tracking cancelled", result.Message)
		assert.Nil(t, result.Attendance)
	})
	t.Run("confirm toggles and answers the new state", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.principals.staff = &ports.Staff{ID: testStaffID}
		h.attendance.toggleAction = devicescan.ScanActionCheckedIn
		h.attendance.state = attendanceState()

		result, err := h.service().ToggleAttendance(ctx, confirm)

		require.NoError(t, err)
		assert.Equal(t, devicescan.ScanActionCheckedIn, result.Action)
		assert.Equal(t, "Hallo Max!", result.Message)
		require.NotNil(t, result.Attendance)
		assert.Equal(t, "checked_in", result.Attendance.Status)
		assert.Nil(t, result.FeedbackEnabled)
		assert.Equal(t, []toggleCall{{StudentID: testStudentID, StaffID: testStaffID, DeviceID: testDeviceID, SkipAuthCheck: false}}, h.attendance.toggles)
	})
	t.Run("checkout farewell", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.attendance.toggleAction = devicescan.ScanActionCheckedOut
		h.attendance.state = attendanceState()

		result, err := h.service().ToggleAttendance(ctx, confirm)

		require.NoError(t, err)
		assert.Equal(t, "Tschüss Max!", result.Message)
	})
	t.Run("an unassigned card is not found", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.people.persons[testTag].HasTag = false
		_, err := h.service().ToggleAttendance(ctx, confirm)
		requireFailure(t, err, devicescan.FailureNotFound, devicescan.MessageRFIDTagUnassigned)
	})
	t.Run("an alumnus is rejected", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.people.students[testPersonID].Alumnus = true
		_, err := h.service().ToggleAttendance(ctx, confirm)
		requireFailure(t, err, devicescan.FailureNotFound, devicescan.MessagePersonNotStudent)
		assert.Empty(t, h.attendance.toggles)
	})
	t.Run("classified refusals keep the service wording", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			sentinel error
			kind     devicescan.FailureKind
		}{
			{ports.ErrConflict, devicescan.FailureConflict},
			{ports.ErrNotFound, devicescan.FailureNotFound},
			{ports.ErrInvalid, devicescan.FailureInvalidRequest},
		} {
			h := newHarness(t)
			h.attendance.toggleErr = mapped{err: errors.New("active: ToggleStudentAttendance: refused"), sentinel: tc.sentinel}
			_, err := h.service().ToggleAttendance(ctx, confirm)
			requireFailure(t, err, tc.kind, "active: ToggleStudentAttendance: refused")
		}
	})
	t.Run("a departed child is not a student", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.attendance.toggleErr = mapped{err: errors.New("active: ToggleStudentAttendance: student has graduated"), sentinel: ports.ErrStudentNotInCare}
		_, err := h.service().ToggleAttendance(ctx, confirm)
		requireFailure(t, err, devicescan.FailureNotFound, devicescan.MessagePersonNotStudent)
	})
	t.Run("an unclassified failure keeps its text", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.attendance.toggleErr = errors.New("active: ToggleStudentAttendance: database operation failed")
		_, err := h.service().ToggleAttendance(ctx, confirm)
		requireFailure(t, err, devicescan.FailureInternal, "active: ToggleStudentAttendance: database operation failed")
	})
	t.Run("a state read failure after the toggle is a server fault", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.attendance.toggleAction = devicescan.ScanActionCheckedIn
		h.attendance.statusErr = errBoom
		_, err := h.service().ToggleAttendance(ctx, confirm)
		requireFailure(t, err, devicescan.FailureInternal, errBoom.Error())
	})
}

func TestToggleAttendance_DailyCheckout(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	home := devicescan.AttendanceToggleCommand{RFIDTag: testTag, Action: devicescan.AttendanceActionDailyCheckout, Destination: devicescan.DestinationHome}

	t.Run("going home", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.attendance.dailyAction = devicescan.ScanActionCheckedOutDaily
		h.settings.feedback = true

		result, err := h.service().ToggleAttendance(ctx, home)

		require.NoError(t, err)
		assert.Equal(t, devicescan.ScanActionCheckedOutDaily, result.Action)
		assert.Equal(t, "Tschüss Max!", result.Message)
		assert.Equal(t, devicescan.AttendanceStudent{ID: testStudentID, FirstName: "Max", LastName: "Muster"}, result.Student)
		assert.Nil(t, result.Attendance)
		require.NotNil(t, result.FeedbackEnabled)
		assert.True(t, *result.FeedbackEnabled)
		assert.Equal(t, []string{devicescan.DestinationHome}, h.attendance.dailyCalls)
	})
	t.Run("staying in transit", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.attendance.dailyAction = devicescan.ScanActionCheckedOut
		h.settings.feedbackErr = errBoom
		transit := home
		transit.Destination = devicescan.DestinationTransit

		result, err := h.service().ToggleAttendance(ctx, transit)

		require.NoError(t, err)
		assert.Equal(t, devicescan.ScanActionCheckedOut, result.Action)
		assert.Equal(t, "Viel Spaß!", result.Message)
		assert.Nil(t, result.FeedbackEnabled, "an unresolved setting omits the flag")
	})
	t.Run("without today's attendance", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.attendance.dailyErr = mapped{err: errors.New("student has no attendance record for today"), sentinel: ports.ErrNoAttendanceRecord}

		_, err := h.service().ToggleAttendance(ctx, home)

		requireFailure(t, err, devicescan.FailureNotFound, "student has no attendance record for today")
	})
	t.Run("a card of no student", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		delete(h.people.students, testPersonID)

		_, err := h.service().ToggleAttendance(ctx, home)

		requireFailure(t, err, devicescan.FailureNotFound, devicescan.MessagePersonNotStudent)
	})
	t.Run("requires a device", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.principals.device = nil

		_, err := h.service().ToggleAttendance(ctx, home)

		require.ErrorIs(t, err, devicescan.ErrDeviceUnauthorized)
	})
}

// mapped mirrors the composition's classified error: the sentinel answers
// errors.Is, the wrapped cause keeps its wording.
type mapped struct {
	err      error
	sentinel error
}

func (e mapped) Error() string        { return e.err.Error() }
func (e mapped) Unwrap() error        { return e.err }
func (e mapped) Is(target error) bool { return target == e.sentinel }
