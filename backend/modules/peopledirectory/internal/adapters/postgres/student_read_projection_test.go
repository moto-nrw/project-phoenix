package postgres

import (
	"testing"
	"time"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/stretchr/testify/assert"
)

func TestRosterConversionPreservesStudentReadRecord(t *testing.T) {
	t.Parallel()

	state := &directory.StudentState{
		CreatedAt:             time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC),
		UpdatedAt:             time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC),
		DepartureDays:         directory.DepartureDays{"mon": directory.DepartureBus},
		AllowedDepartureModes: directory.AllowedDepartureModes{"mon": {directory.DepartureBus}},
		PickupDays:            directory.PickupDays{"tue": true},
		BusDays:               directory.BusDays{"mon": true},
	}
	row := directory.StudentRosterEntry{Record: state, FirstName: "Roster", LastName: "Child"}

	converted := toDomainRosterEntry(row)
	assert.Equal(t, state.CreatedAt, converted.Record.CreatedAt)
	assert.Equal(t, state.UpdatedAt, converted.Record.UpdatedAt)
	assert.Equal(t, state.DepartureDays, converted.Record.DepartureDays)
	assert.Equal(t, state.AllowedDepartureModes, converted.Record.AllowedDepartureModes)
	assert.Equal(t, state.PickupDays, converted.Record.PickupDays)
	assert.Equal(t, state.BusDays, converted.Record.BusDays)
	assert.Equal(t, row.FirstName, converted.FirstName)
	assert.Equal(t, row.LastName, converted.LastName)
}
