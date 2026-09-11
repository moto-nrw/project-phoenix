package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
)

func TestSelectPickupNote(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Ring bell at side entrance\nGrandma is picking up today", selectPickupNote(&ports.Pickup{
		Notes: "Recurring note should be ignored when day notes exist", DayNotes: []string{"Ring bell at side entrance", "Grandma is picking up today"},
	}))
	assert.Equal(t, "Wait at the side entrance", selectPickupNote(&ports.Pickup{Notes: "Wait at the side entrance"}))
	assert.Equal(t, "", selectPickupNote(nil))
	assert.Equal(t, "Recurring fallback note", selectPickupNote(&ports.Pickup{Notes: "Recurring fallback note", DayNotes: []string{"   ", "\t"}}))
	assert.Equal(t, "", selectPickupNote(&ports.Pickup{Notes: "  ", DayNotes: []string{}}))
}

func TestPickupInfo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("returns the plan without writing", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.pickups.pickup = &ports.Pickup{Time: ptr(time.Date(2000, 1, 1, 15, 30, 0, 0, time.UTC)), DayNotes: []string{"Mama holt heute frueher ab"}}
		h.sessions.current = &ports.Session{ID: 201}

		info, err := h.service().PickupInfo(ctx, testTag)

		require.NoError(t, err)
		assert.Equal(t, testStudentID, info.StudentID)
		assert.Equal(t, "Max Muster", info.StudentName)
		assert.Equal(t, "15:30", info.PickupTime)
		assert.Equal(t, "Mama holt heute frueher ab", info.PickupNote)
		assert.Equal(t, fixedNow, info.ProcessedAt)
		assert.Empty(t, h.visits.recorded)
		assert.Empty(t, h.visits.ended)
		assert.Equal(t, []int64{201}, h.sessions.touched, "the device session heartbeat is refreshed")
	})
	t.Run("a failed session heartbeat still answers the plan", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.pickups.pickup = &ports.Pickup{Time: ptr(time.Date(2000, 1, 1, 15, 30, 0, 0, time.UTC))}
		h.sessions.current = &ports.Session{ID: 201}
		h.sessions.touchErr = errBoom

		info, err := h.service().PickupInfo(ctx, testTag)

		require.NoError(t, err)
		assert.Equal(t, "15:30", info.PickupTime)
		assert.Equal(t, []int64{201}, h.sessions.touched, "the heartbeat was attempted")
	})
	t.Run("omits time and note without a plan", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		info, err := h.service().PickupInfo(ctx, testTag)

		require.NoError(t, err)
		assert.Empty(t, info.PickupTime)
		assert.Empty(t, info.PickupNote)
	})
	t.Run("a staff card is rejected", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		delete(h.people.students, testPersonID)
		h.people.staff[testPersonID] = &ports.StaffMember{ID: testStaffID}

		_, err := h.service().PickupInfo(ctx, testTag)

		requireFailure(t, err, devicescan.FailureInvalidRequest, devicescan.MessageStudentRFIDRequiredForPickup)
	})
	t.Run("a card of neither is not found", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		delete(h.people.students, testPersonID)

		_, err := h.service().PickupInfo(ctx, testTag)

		requireFailure(t, err, devicescan.FailureNotFound, devicescan.MessageRFIDTagNotStudentOrStaff)
	})
	t.Run("an unknown card is recorded", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)

		_, err := h.service().PickupInfo(ctx, "UNKNOWN")

		failure := classifiedFailure(t, err, devicescan.FailureNotFound, devicescan.MessageRFIDTagNotFound)
		assert.Equal(t, devicescan.CodeRFIDTagNotFound, failure.Code)
		assert.Len(t, h.fleet.scans, 1)
	})
	t.Run("a card lookup failure does not leak", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.people.personErr = errors.New("rfid lookup exploded")

		_, err := h.service().PickupInfo(ctx, testTag)

		requireFailure(t, err, devicescan.FailureInternal, devicescan.MessageInternalServerError)
	})
	t.Run("a student lookup failure is a server fault", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.people.studentErr = errors.New("student lookup exploded")

		_, err := h.service().PickupInfo(ctx, testTag)

		requireFailure(t, err, devicescan.FailureInternal, "student lookup exploded")
	})
	t.Run("a staff lookup failure is a server fault", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		delete(h.people.students, testPersonID)
		h.people.staffErr = errors.New("staff lookup exploded")

		_, err := h.service().PickupInfo(ctx, testTag)

		requireFailure(t, err, devicescan.FailureInternal, "staff lookup exploded")
	})
	t.Run("a pickup lookup failure is a server fault", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.pickups.err = errors.New("schedule lookup exploded")

		_, err := h.service().PickupInfo(ctx, testTag)

		requireFailure(t, err, devicescan.FailureInternal, "schedule lookup exploded")
	})
	t.Run("requires a device", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.principals.device = nil

		_, err := h.service().PickupInfo(ctx, testTag)

		require.ErrorIs(t, err, devicescan.ErrDeviceUnauthorized)
	})
}
