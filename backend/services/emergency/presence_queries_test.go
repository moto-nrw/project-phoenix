package emergency

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type locationFacts struct {
	rows   []studentpresence.VisitLocation
	err    error
	filter studentpresence.VisitLocationFilter
}

func (f *locationFacts) ListVisitLocations(_ context.Context, filter studentpresence.VisitLocationFilter) ([]studentpresence.VisitLocation, error) {
	f.filter = filter
	return f.rows, f.err
}

type roomFacts struct {
	err error
	ids []int64
}

func (f *roomFacts) names(_ context.Context, ids []int64) (map[int64]string, error) {
	f.ids = ids
	return map[int64]string{10: "Kreativraum"}, f.err
}

func TestEmergencyVisitLocationsPreserveSelectionAndReadFailures(t *testing.T) {
	t.Parallel()
	visits := &locationFacts{rows: []studentpresence.VisitLocation{
		{Visit: studentpresence.Visit{StudentID: 1}, Group: &studentpresence.VisitGroup{RoomID: 10}},
		{Visit: studentpresence.Visit{StudentID: 2}, Group: &studentpresence.VisitGroup{RoomID: 10}},
		{Visit: studentpresence.Visit{StudentID: 3}},
	}}
	rooms := &roomFacts{}
	service := NewService(Dependencies{Visits: visits, RoomNames: rooms.names})
	ids := []int64{1, 2, 3}
	locations, err := service.loadVisitLocations(context.Background(), ids)
	require.NoError(t, err)
	require.Equal(t, map[int64]string{1: "Kreativraum", 2: "Kreativraum"}, locations)
	require.Equal(t, []int64{10}, rooms.ids)
	require.Equal(t, studentpresence.VisitLocationFilter{
		VisitFilter:       studentpresence.VisitFilter{StudentIDs: ids, OpenOnly: true},
		RunningGroupsOnly: true, LatestPerStudent: true,
	}, visits.filter)

	rooms.err = assert.AnError
	locations, err = service.loadVisitLocations(context.Background(), ids)
	require.ErrorIs(t, err, assert.AnError)
	require.Nil(t, locations)

	rooms.ids = nil
	visits.err = assert.AnError
	locations, err = service.loadVisitLocations(context.Background(), ids)
	require.ErrorIs(t, err, assert.AnError)
	require.Nil(t, locations)
	require.Nil(t, rooms.ids, "failed visit reads must stop room lookup")
}
