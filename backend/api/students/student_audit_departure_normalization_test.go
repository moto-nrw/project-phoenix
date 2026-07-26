package students

import (
	"testing"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeDeparturePlanForAudit_UsesLegacyMaps(t *testing.T) {
	student := &userModels.Student{
		DepartureDays: userModels.DepartureDays{
			userModels.PickupDayMonday: userModels.DepartureBus,
		},
		BusDays: userModels.BusDays{
			userModels.PickupDayTuesday: true,
		},
		PickupDays: userModels.PickupDays{
			userModels.PickupDayWednesday: true,
		},
	}

	normalizeDeparturePlanForAudit(student)

	assert.Equal(t, userModels.DepartureAlone, student.DepartureDays.ModeFor(userModels.PickupDayMonday))
	assert.Equal(t, userModels.DepartureBus, student.DepartureDays.ModeFor(userModels.PickupDayTuesday))
	assert.Equal(t, userModels.DeparturePickup, student.DepartureDays.ModeFor(userModels.PickupDayWednesday))
}

func TestStudentAuditBeforeImage_NormalizesLegacyDeparturePlan(t *testing.T) {
	student := &userModels.Student{
		// The database column is NOT NULL DEFAULT '{}', so a legacy row can
		// carry an allocated but empty modern map alongside its old fields.
		AllowedDepartureModes: userModels.AllowedDepartureModes{},
		BusDays: userModels.BusDays{
			userModels.PickupDayTuesday: true,
		},
		PickupDays: userModels.PickupDays{
			userModels.PickupDayWednesday: true,
		},
	}

	before := studentAuditBeforeImage(student, true)

	assert.Equal(t, userModels.DepartureBus, before.DepartureDays.ModeFor(userModels.PickupDayTuesday))
	assert.Equal(t, userModels.DeparturePickup, before.DepartureDays.ModeFor(userModels.PickupDayWednesday))
	assert.Nil(t, student.DepartureDays, "normalizing the audit copy must not mutate the locked student")
}
