package application

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
)

// The daily-checkout gates are pure decisions over the settings, the pickup
// plan, the education group and the yard; the clock is pinned.

func gateService(t *testing.T, arrange func(h *harness)) *Service {
	t.Helper()
	h := newHarness(t)
	h.settings = &fakeSettings{}
	if arrange != nil {
		arrange(h)
	}
	return h.service()
}

func studentInGroup() *ports.Student { return &ports.Student{ID: 1, GroupID: ptr(int64(1))} }

func visitIn(roomID int64) *ports.CurrentVisit {
	return &ports.CurrentVisit{Session: &ports.SessionRef{RoomID: roomID}}
}

func TestDailyCheckoutTime(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	gate, err := gateService(t, nil).dailyCheckoutTime(ctx)
	require.NoError(t, err)
	assert.Nil(t, gate, "no configured time means no gate")

	gate, err = gateService(t, func(h *harness) { h.settings.dailyCheckout = "14:30" }).dailyCheckoutTime(ctx)
	require.NoError(t, err)
	require.NotNil(t, gate)
	assert.Equal(t, 14, gate.Hour())
	assert.Equal(t, 30, gate.Minute())
	assert.Equal(t, fixedNow.Year(), gate.Year(), "the gate is today's instant")

	for raw, wantErr := range map[string]string{
		"invalid": "invalid checkout time format",
		"25:00":   "invalid hour",
		"12:99":   "invalid minute",
		"-1:00":   "invalid hour",
		"12:-5":   "invalid minute",
	} {
		_, err := gateService(t, func(h *harness) { h.settings.dailyCheckout = raw }).dailyCheckoutTime(ctx)
		assert.ErrorContains(t, err, wantErr, raw)
	}
	for _, raw := range []string{"00:00", "23:59", "12:00"} {
		gate, err := gateService(t, func(h *harness) { h.settings.dailyCheckout = raw }).dailyCheckoutTime(ctx)
		require.NoError(t, err, raw)
		require.NotNil(t, gate, raw)
	}
}

func TestShouldUpgradeToDailyCheckout_Preconditions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := gateService(t, nil)

	assert.False(t, s.shouldUpgradeToDailyCheckout(ctx, devicescan.ScanActionCheckedIn, &ports.Student{ID: 1}, nil), "only a checkout upgrades")
	assert.False(t, s.shouldUpgradeToDailyCheckout(ctx, devicescan.ScanActionCheckedOut, &ports.Student{ID: 1}, nil), "no group")
	assert.False(t, s.shouldUpgradeToDailyCheckout(ctx, devicescan.ScanActionCheckedOut, studentInGroup(), nil), "no visit")
	assert.False(t, s.shouldUpgradeToDailyCheckout(ctx, devicescan.ScanActionCheckedOut, studentInGroup(), &ports.CurrentVisit{}), "no session")
}

func TestShouldUpgradeToDailyCheckout_NoTimeGateAndRoomlessGroup(t *testing.T) {
	t.Parallel()
	s := gateService(t, func(h *harness) { h.groups.group = &ports.Group{ID: 1} })

	assert.True(t, s.shouldUpgradeToDailyCheckout(context.Background(), devicescan.ScanActionCheckedOut, studentInGroup(), visitIn(1)))
}

func TestShouldShowDailyCheckoutWithGroup(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("preconditions", func(t *testing.T) {
		t.Parallel()
		s := gateService(t, nil)
		assert.False(t, s.shouldShowDailyCheckoutWithGroup(ctx, &ports.Student{ID: 1}, visitIn(1)))
		assert.False(t, s.shouldShowDailyCheckoutWithGroup(ctx, studentInGroup(), nil))
		assert.False(t, s.shouldShowDailyCheckoutWithGroup(ctx, studentInGroup(), &ports.CurrentVisit{}))
	})
	t.Run("before the checkout time", func(t *testing.T) {
		t.Parallel()
		s := gateService(t, func(h *harness) {
			h.clock = fakeClock{now: time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)}
			h.settings.dailyCheckout = "13:00"
			h.groups.group = &ports.Group{ID: 1}
		})
		assert.False(t, s.shouldShowDailyCheckoutWithGroup(ctx, studentInGroup(), visitIn(1)))
	})
	t.Run("no time gate and a roomless group", func(t *testing.T) {
		t.Parallel()
		s := gateService(t, func(h *harness) { h.groups.group = &ports.Group{ID: 1} })
		assert.True(t, s.shouldShowDailyCheckoutWithGroup(ctx, studentInGroup(), visitIn(1)))
	})
	t.Run("own group room", func(t *testing.T) {
		t.Parallel()
		s := gateService(t, func(h *harness) { h.groups.group = &ports.Group{ID: 1, RoomID: ptr(int64(42))} })
		assert.True(t, s.shouldShowDailyCheckoutWithGroup(ctx, studentInGroup(), visitIn(42)))
		assert.False(t, s.shouldShowDailyCheckoutWithGroup(ctx, studentInGroup(), visitIn(99)), "another room is not offered")
	})
	t.Run("an unparsable gate closes", func(t *testing.T) {
		t.Parallel()
		s := gateService(t, func(h *harness) { h.settings.dailyCheckout = "not-a-time" })
		assert.False(t, s.shouldShowDailyCheckoutWithGroup(ctx, studentInGroup(), visitIn(1)))
	})
	t.Run("a group lookup failure closes", func(t *testing.T) {
		t.Parallel()
		s := gateService(t, func(h *harness) { h.groups.err = errBoom })
		assert.False(t, s.shouldShowDailyCheckoutWithGroup(ctx, studentInGroup(), visitIn(1)))
	})
	t.Run("every room when the tenant enabled it", func(t *testing.T) {
		t.Parallel()
		s := gateService(t, func(h *harness) {
			h.clock = fakeClock{now: time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)}
			h.settings.dailyCheckout = "00:00"
			h.settings.allRooms = true
			h.groups.group = &ports.Group{ID: 1, RoomID: ptr(int64(42))}
		})
		assert.True(t, s.shouldShowDailyCheckoutWithGroup(ctx, studentInGroup(), visitIn(99)))
	})
}

// The GS-Barnstorf shape (#2377): the child's group owns a room, and the
// visit being closed happened in the Schulhof instead. "nach Hause" must be
// offered without being sent automatically.
func schulhofScenario(t *testing.T, arrange func(h *harness)) (*Service, *ports.Student, *ports.CurrentVisit) {
	t.Helper()
	const groupRoomID, schulhofRoomID = int64(42), int64(7)
	s := gateService(t, func(h *harness) {
		h.groups.group = &ports.Group{ID: 1, RoomID: ptr(groupRoomID)}
		h.rooms.byName[facilities.SchulhofRoomName] = canonicalSchulhof(schulhofRoomID)
		if arrange != nil {
			arrange(h)
		}
	})
	return s, studentInGroup(), visitIn(schulhofRoomID)
}

func TestSchulhofDailyCheckout(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("offered at the yard", func(t *testing.T) {
		t.Parallel()
		s, student, visit := schulhofScenario(t, nil)
		assert.True(t, s.shouldShowDailyCheckoutWithGroup(ctx, student, visit))
	})
	t.Run("but never auto-sent home", func(t *testing.T) {
		t.Parallel()
		s, student, visit := schulhofScenario(t, nil)
		assert.False(t, s.shouldUpgradeToDailyCheckout(ctx, devicescan.ScanActionCheckedOut, student, visit))
	})
	t.Run("the time gate still applies", func(t *testing.T) {
		t.Parallel()
		s, student, visit := schulhofScenario(t, func(h *harness) {
			h.clock = fakeClock{now: time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)}
			h.settings.dailyCheckout = "13:00"
		})
		assert.False(t, s.shouldShowDailyCheckoutWithGroup(ctx, student, visit))
	})
	t.Run("an ordinary room is not offered", func(t *testing.T) {
		t.Parallel()
		s, student, _ := schulhofScenario(t, nil)
		assert.False(t, s.shouldShowDailyCheckoutWithGroup(ctx, student, visitIn(99)))
	})
	t.Run("a school without a yard is not an error", func(t *testing.T) {
		t.Parallel()
		s, student, visit := schulhofScenario(t, func(h *harness) { delete(h.rooms.byName, facilities.SchulhofRoomName) })
		assert.False(t, s.shouldShowDailyCheckoutWithGroup(ctx, student, visit))
	})
	t.Run("a non-system room named Schulhof does not unlock it", func(t *testing.T) {
		t.Parallel()
		s, student, visit := schulhofScenario(t, func(h *harness) {
			h.rooms.byName[facilities.SchulhofRoomName] = facilities.Room{ID: 7, Name: facilities.SchulhofRoomName}
		})
		assert.False(t, s.shouldShowDailyCheckoutWithGroup(ctx, student, visit))
	})
	t.Run("the own group room still upgrades", func(t *testing.T) {
		t.Parallel()
		s, student, _ := schulhofScenario(t, nil)
		assert.True(t, s.shouldUpgradeToDailyCheckout(ctx, devicescan.ScanActionCheckedOut, student, visitIn(42)))
	})
}

func TestIsAfterCheckoutTimeGate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	student := &ports.Student{ID: 1}
	noon := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	pickupAt := func(hour, minute int) *ports.Pickup {
		return &ports.Pickup{Time: ptr(time.Date(2000, 1, 1, hour, minute, 0, 0, time.UTC))}
	}

	t.Run("per-student disabled falls back to the global gate", func(t *testing.T) {
		t.Parallel()
		assert.True(t, gateService(t, nil).isAfterCheckoutTimeGate(ctx, student))
		s := gateService(t, func(h *harness) { h.clock = fakeClock{now: noon}; h.settings.dailyCheckout = "13:00" })
		assert.False(t, s.isAfterCheckoutTimeGate(ctx, student))
	})
	t.Run("before pickup minus delta", func(t *testing.T) {
		t.Parallel()
		s := gateService(t, func(h *harness) {
			h.clock = fakeClock{now: noon}
			h.settings.perStudent, h.settings.delta = true, 15
			h.pickups.pickup = pickupAt(14, 0)
		})
		assert.False(t, s.isAfterCheckoutTimeGate(ctx, student))
	})
	t.Run("after pickup minus delta", func(t *testing.T) {
		t.Parallel()
		s := gateService(t, func(h *harness) {
			h.clock = fakeClock{now: noon}
			h.settings.perStudent, h.settings.delta = true, 15
			h.pickups.pickup = pickupAt(12, 5)
		})
		assert.True(t, s.isAfterCheckoutTimeGate(ctx, student))
	})
	t.Run("no pickup time falls back to the global gate", func(t *testing.T) {
		t.Parallel()
		s := gateService(t, func(h *harness) { h.settings.perStudent = true; h.pickups.pickup = &ports.Pickup{} })
		assert.True(t, s.isAfterCheckoutTimeGate(ctx, student))
	})
	t.Run("a pickup lookup failure falls back to the global gate", func(t *testing.T) {
		t.Parallel()
		s := gateService(t, func(h *harness) { h.settings.perStudent = true; h.pickups.err = errBoom })
		assert.True(t, s.isAfterCheckoutTimeGate(ctx, student))
	})
	t.Run("a missing pickup plan falls back to the global gate", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.settings = &fakeSettings{perStudent: true}
		h.pickups = nil
		s := NewService(Dependencies{
			Fleet: h.fleet, Presence: h.presence, Rooms: h.rooms, Principals: h.principals, People: h.people,
			Visits: h.visits, Sessions: h.sessions, Attendance: h.attendance, Settings: h.settings,
			UnitOfWork: h.unit, Clock: h.clock, Logger: h.logger,
		})
		assert.True(t, s.isAfterCheckoutTimeGate(ctx, student))
	})
	t.Run("a zero delta waits for the pickup time itself", func(t *testing.T) {
		t.Parallel()
		s := gateService(t, func(h *harness) {
			h.clock = fakeClock{now: noon}
			h.settings.perStudent, h.settings.delta = true, 0
			h.pickups.pickup = pickupAt(12, 1)
		})
		assert.False(t, s.isAfterCheckoutTimeGate(ctx, student))
	})
	t.Run("an unreadable delta defaults to fifteen minutes", func(t *testing.T) {
		t.Parallel()
		s := gateService(t, func(h *harness) {
			h.clock = fakeClock{now: noon}
			h.settings.perStudent, h.settings.deltaErr = true, errBoom
			h.pickups.pickup = pickupAt(12, 10)
		})
		assert.True(t, s.isAfterCheckoutTimeGate(ctx, student))
	})
}
