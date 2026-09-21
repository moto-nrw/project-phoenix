package carerequests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPickupImpactContentPreservesWireEncoding(t *testing.T) {
	t.Parallel()
	blocks := []Block{
		{ID: 17, Title: "Früh", StartTime: time.Date(2026, 9, 21, 12, 30, 5, 0, time.UTC), EndTime: time.Date(2026, 9, 21, 13, 45, 0, 0, time.UTC)},
		{ID: 23, Title: "Sport", StartTime: time.Date(2026, 9, 21, 14, 0, 0, 0, time.UTC), EndTime: time.Date(2026, 9, 21, 15, 0, 0, 0, time.UTC)},
	}
	assert.Equal(t, "17\x00Früh\x0012:30:05\x0013:45:00\x0023\x00Sport\x0014:00:00\x0015:00:00\x00", string(PickupImpactContent(blocks)))
	assert.Empty(t, PickupImpactContent(nil))
	reversed := []Block{blocks[1], blocks[0]}
	assert.NotEqual(t, PickupImpactContent(blocks), PickupImpactContent(reversed), "reviewed order is part of the confirmation token")
}
