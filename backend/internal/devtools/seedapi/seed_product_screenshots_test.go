package api

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The public demo runs Friday's plan on the weekend (#3921); the screenshot
// run photographs a Tuesday and must show no Saturday or Sunday columns.
func TestProductScreenshotsSwitchOffTheWeekendPlanOnly(t *testing.T) {
	t.Parallel()

	setting := func(options SeedOptions) string {
		seeder := NewSeeder(newSeedTestAdapter("http://localhost:8080"), newSeedTestRandom(), false, options)
		return string(seeder.definition.Settings[profileSettingWeekendFollowsFriday].Value)
	}
	assert.Equal(t, "true", setting(SeedOptions{}), "the demo keeps the weekend care day")
	assert.Equal(t, "true", setting(SeedOptions{StandingDemo: true}), "the standing demo keeps the weekend care day")
	assert.Equal(t, "false", setting(SeedOptions{ProductScreenshots: true}))

	standing := NewSeeder(newSeedTestAdapter("http://localhost:8080"), newSeedTestRandom(), false, SeedOptions{StandingDemo: true})
	assert.Equal(t, "true", string(standing.definition.Settings[profileSettingAttendanceNFC].Value), "the standing demo's simulator requires NFC device routes")
}

// The simulated screenshot day fills the Statistik; the seed-time session
// would stand in the Tagesplan as a one-minute block.
func TestProductScreenshotsLeaveOutTheStatisticsSession(t *testing.T) {
	t.Parallel()

	hasStatistics := func(options SeedOptions) bool {
		steps := fullDemoWorkflow(&Seeder{options: options}).Steps
		return slices.ContainsFunc(steps, func(step Step) bool { _, ok := step.(seedStatisticsDemoStep); return ok })
	}
	assert.True(t, hasStatistics(SeedOptions{OnlyProfile: DefaultProfileKey}), "a fresh demo school shows the Statistik at once")
	assert.False(t, hasStatistics(SeedOptions{OnlyProfile: DefaultProfileKey, ProductScreenshots: true}))

	full := fullDemoWorkflow(&Seeder{options: SeedOptions{OnlyProfile: DefaultProfileKey}}).Steps
	screenshots := fullDemoWorkflow(&Seeder{options: SeedOptions{OnlyProfile: DefaultProfileKey, ProductScreenshots: true}}).Steps
	assert.Len(t, screenshots, len(full)-1, "nothing else is left out")
}
