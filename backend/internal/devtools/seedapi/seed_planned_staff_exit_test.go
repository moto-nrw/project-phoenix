package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPlannedSeedExitDayIsTheEndOfTheMonthAfterNext(t *testing.T) {
	t.Parallel()
	for today, want := range map[string]string{
		"2026-10-28": "2026-12-31",
		"2026-11-01": "2027-01-31",
		"2026-12-31": "2027-02-28",
	} {
		parsed, err := time.Parse(seedDateLayout, today)
		assert.NoError(t, err)
		assert.Equal(t, want, plannedSeedExitDay(seedDate{Time: parsed}).String(), today)
	}
}
