package application

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/stretchr/testify/assert"
)

// The idempotency key is what makes a second scheduler tick, an overlapping
// window or a restarted process harmless. It has to separate exactly three
// things plus the current appointment revision. A content edit must be able to
// replace a pending reminder that was cancelled as stale.
func TestAppointmentReminderKey(t *testing.T) {
	t.Parallel()

	base := appointmentReminderKey(42, 3, appointments.NewDate(2026, 1, 5), 7)

	assert.Equal(t, base, appointmentReminderKey(42, 3, appointments.NewDate(2026, 1, 5), 7),
		"the same occurrence and guardian must produce the same key, or the mail repeats")
	assert.NotEqual(t, base, appointmentReminderKey(42, 3, appointments.NewDate(2026, 1, 12), 7),
		"every occurrence of a series gets its own reminder")
	assert.NotEqual(t, base, appointmentReminderKey(42, 3, appointments.NewDate(2026, 1, 5), 8),
		"both parents of a child get their own mail")
	assert.NotEqual(t, base, appointmentReminderKey(43, 3, appointments.NewDate(2026, 1, 5), 7))
	assert.NotEqual(t, base, appointmentReminderKey(42, 4, appointments.NewDate(2026, 1, 5), 7),
		"an updated appointment needs a replacement reminder key")
}

// A claim is taken per guardian profile before the push is dispatched. Between
// claim and dispatch a guardian can lose access, and those stale claims have to
// be released — otherwise the reminder is never retried for them.
func TestWithoutReminderProfiles(t *testing.T) {
	t.Parallel()

	claimed := []int64{11, 12, 13}

	assert.Equal(t, []int64{12}, withoutReminderProfiles(claimed, []int64{11, 13}),
		"a guardian who is no longer reachable must have their claim released")
	assert.Empty(t, withoutReminderProfiles(claimed, claimed))
	assert.Equal(t, claimed, withoutReminderProfiles(claimed, nil),
		"losing every recipient releases every claim")
	assert.Empty(t, withoutReminderProfiles(nil, claimed))
}
