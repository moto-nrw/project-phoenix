package analytics

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIdentifySchoolsNamesEachSchoolUnderItsDeployment(t *testing.T) {
	t.Parallel()

	server := newPostHogServer(t, http.StatusOK)
	tracker := newServerTracker(t, server, nil)

	identified := IdentifySchools(tracker, []School{
		{ID: 6, Name: " OGS Altenberge "},
		{ID: 7, Name: ""},
		{ID: 0, Name: "ohne ID"},
	})
	require.NoError(t, tracker.Close())

	assert.Equal(t, 1, identified, "a school without ID or name is skipped")
	events := server.events()
	require.Len(t, events, 1)
	event := events[0]
	assert.Equal(t, "$groupidentify", event.Event)
	assert.Equal(t, "$school_"+testDeployment+":6", event.DistinctID)
	assert.Equal(t, "school", event.Properties["$group_type"])
	assert.Equal(t, testDeployment+":6", event.Properties["$group_key"])
	assert.Equal(t, map[string]any{"name": "OGS Altenberge", "deployment": testDeployment}, event.Properties["$group_set"])
	assert.Equal(t, testDeployment, event.Properties["deployment"])
	assert.Equal(t, true, event.Properties["$geoip_disable"])
	assert.NotContains(t, event.Properties, "$process_person_profile", "a group is no person")
}

func TestTrackerDropsAnyOtherGroupIdentify(t *testing.T) {
	t.Parallel()

	server := newPostHogServer(t, http.StatusOK)
	logger, logs := newTestLogger()
	tracker := newServerTracker(t, server, logger)

	tracker.Capture("", "$groupidentify", map[string]any{
		"$group_type": "company",
		"$group_key":  "1",
		"$group_set":  map[string]any{"name": "x"},
	})
	tracker.Capture("", "$groupidentify", map[string]any{"$group_type": "school", "$group_key": "1"})
	require.NoError(t, tracker.Close())

	assert.Empty(t, server.events())
	assert.Contains(t, logs.String(), "only school groups are identified")
}
