package application

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSickAtCutoffTreatsClearanceAtCutoffAsEffective(t *testing.T) {
	t.Parallel()
	berlin := time.FixedZone("Europe/Berlin", 2*60*60)
	cutoff := time.Date(2026, 9, 7, 9, 0, 0, 0, berlin)
	reportedAt := cutoff.Add(-30 * time.Minute)
	clearedAt := cutoff.UTC()

	assert.False(t, sickAtCutoff(reportedAt, &clearedAt, cutoff))
}
