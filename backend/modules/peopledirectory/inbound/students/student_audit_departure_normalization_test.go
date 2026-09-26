package students

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeDeparturePlanForAudit_UsesLegacyMaps(t *testing.T) {
	t.Parallel()

	student := &Student{
		DepartureDays: departure.DepartureDays{
			departure.PickupDayMonday: departure.DepartureBus,
		},
		BusDays: departure.BusDays{
			departure.PickupDayTuesday: true,
		},
		PickupDays: departure.PickupDays{
			departure.PickupDayWednesday: true,
		},
	}

	normalizeDeparturePlanForAudit(student)

	assert.Equal(t, departure.DepartureAlone, student.DepartureDays.ModeFor(departure.PickupDayMonday))
	assert.Equal(t, departure.DepartureBus, student.DepartureDays.ModeFor(departure.PickupDayTuesday))
	assert.Equal(t, departure.DeparturePickup, student.DepartureDays.ModeFor(departure.PickupDayWednesday))
}

func TestStudentAuditBeforeImage_NormalizesLegacyDeparturePlan(t *testing.T) {
	t.Parallel()

	student := &Student{
		// The database column is NOT NULL DEFAULT '{}', so a legacy row can
		// carry an allocated but empty modern map alongside its old fields.
		AllowedDepartureModes: departure.AllowedDepartureModes{},
		BusDays: departure.BusDays{
			departure.PickupDayTuesday: true,
		},
		PickupDays: departure.PickupDays{
			departure.PickupDayWednesday: true,
		},
	}

	before := studentAuditBeforeImage(student, true)

	assert.Equal(t, departure.DepartureBus, before.DepartureDays.ModeFor(departure.PickupDayTuesday))
	assert.Equal(t, departure.DeparturePickup, before.DepartureDays.ModeFor(departure.PickupDayWednesday))
	assert.Nil(t, student.DepartureDays, "normalizing the audit copy must not mutate the locked student")
}
