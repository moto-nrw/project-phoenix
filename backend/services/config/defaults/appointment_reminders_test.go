package defaults_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The guardian appointment reminder (#1671) is the one reminder setting that
// defaults ON, so its default is pinned here rather than left to the generic
// registration sweep: flipping it silently would either start or stop mailing
// every school's parents.
func TestAppointmentReminderSettings(t *testing.T) {
	t.Parallel()

	t.Run("enabled defaults on", func(t *testing.T) {
		def := config.GetDefinition(config.KeyCalendarAppointmentReminderEnabled)
		require.NotNil(t, def, "calendar.appointment_reminder_enabled should be registered")
		assert.Equal(t, config.FieldBoolean, def.Type)
		assert.Equal(t, true, def.Default,
			"schools that already mail guardians about an appointment also get the reminder")
		assert.Equal(t, "reminders", def.Tab)
		assert.Equal(t, "config:update", def.WritePermission)
	})

	t.Run("lead time is bounded and depends on the toggle", func(t *testing.T) {
		def := config.GetDefinition(config.KeyCalendarAppointmentReminderLeadHours)
		require.NotNil(t, def, "calendar.appointment_reminder_lead_hours should be registered")
		assert.Equal(t, config.FieldNumber, def.Type)
		assert.Equal(t, 24, def.Default)

		require.NotNil(t, def.Validation, "an unbounded lead time would scan arbitrary windows")
		require.NotNil(t, def.Validation.Min)
		require.NotNil(t, def.Validation.Max)
		assert.Equal(t, float64(1), *def.Validation.Min)
		assert.Equal(t, float64(168), *def.Validation.Max)

		require.NotNil(t, def.DependsOn)
		assert.Equal(t, config.KeyCalendarAppointmentReminderEnabled, def.DependsOn.Key)
	})
}
