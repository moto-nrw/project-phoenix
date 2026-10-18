package facilities

import (
	"context"
	"errors"
	"testing"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type occupancyGroupsStub struct {
	facts []facilitiesModule.RoomOccupancyFact
	err   error
}

func (s occupancyGroupsStub) ListRoomOccupancy(context.Context, []int64) ([]facilitiesModule.RoomOccupancyFact, error) {
	return s.facts, s.err
}

type activityProjectionStub struct {
	groups map[int64]facilitiesModule.ActivityGroupName
	err    error
}

func (s activityProjectionStub) ListActivityGroupNames(context.Context, []int64) (map[int64]facilitiesModule.ActivityGroupName, error) {
	return s.groups, s.err
}

type membershipQueryStub struct {
	schoolmembership.Query
	staff []schoolmembership.Staff
	err   error
}

func (s membershipQueryStub) ListStaff(context.Context, schoolmembership.StaffFilter) ([]schoolmembership.Staff, error) {
	return s.staff, s.err
}

type personQueryStub struct {
	peopledirectory.Query
	people []peopledirectory.Person
	err    error
}

func (s personQueryStub) ListPersonsByID(context.Context, []int64) ([]peopledirectory.Person, error) {
	return s.people, s.err
}

func TestOccupancyProjectionBuildsStableBatchProjection(t *testing.T) {
	t.Parallel()
	rooms := []facilitiesModule.Room{{ID: 2, Name: "Fuchsbau"}, {ID: 1, Name: "Igelraum"}}
	facts := []facilitiesModule.RoomOccupancyFact{{
		RoomID: 2, ActivityGroupIDs: []int64{20, 10, 30}, StudentCount: 3,
		SupervisorStaffIDs: []int64{7, 6, 7},
	}}
	activities := activityProjectionStub{groups: map[int64]facilitiesModule.ActivityGroupName{
		20: activityGroup("Zirkus", "Bewegung"),
		10: activityGroup("Atelier", "Kreativ"),
	}}
	membership := membershipQueryStub{staff: []schoolmembership.Staff{{ID: 7, PersonID: 70}, {ID: 6, PersonID: 60}}}
	persons := personQueryStub{people: []peopledirectory.Person{
		{ID: 70, FirstName: "Berta", LastName: "Zwei"},
		{ID: 60, FirstName: "Anna", LastName: "Eins"},
	}}

	project := NewOccupancyProjection(occupancyGroupsStub{facts: facts}, activities, membership, persons)
	result, err := project(context.Background(), rooms)

	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.True(t, result[0].IsOccupied)
	assert.Equal(t, 3, result[0].StudentCount)
	assert.Equal(t, "Atelier, Zirkus", pointerValue(result[0].GroupName))
	assert.Equal(t, "Bewegung, Kreativ", pointerValue(result[0].CategoryName))
	assert.Equal(t, "Anna Eins, Berta Zwei", pointerValue(result[0].SupervisorNames))
	assert.False(t, result[1].IsOccupied)
}

func TestOccupancyProjectionPropagatesDependencyErrors(t *testing.T) {
	t.Parallel()
	want := errors.New("dependency failed")
	room := []facilitiesModule.Room{{ID: 1}}
	fact := []facilitiesModule.RoomOccupancyFact{{RoomID: 1, ActivityGroupIDs: []int64{1}, SupervisorStaffIDs: []int64{2}}}
	tests := []struct {
		name       string
		groups     occupancyGroupsStub
		activities activityProjectionStub
		membership membershipQueryStub
		persons    personQueryStub
	}{
		{name: "occupancy", groups: occupancyGroupsStub{err: want}},
		{name: "activities", groups: occupancyGroupsStub{facts: fact}, activities: activityProjectionStub{err: want}},
		{name: "membership", groups: occupancyGroupsStub{facts: fact}, activities: activityProjectionStub{}, membership: membershipQueryStub{err: want}},
		{name: "people", groups: occupancyGroupsStub{facts: fact}, activities: activityProjectionStub{}, membership: membershipQueryStub{staff: []schoolmembership.Staff{{ID: 2, PersonID: 3}}}, persons: personQueryStub{err: want}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := NewOccupancyProjection(test.groups, test.activities, test.membership, test.persons)
			_, err := project(context.Background(), room)
			assert.ErrorIs(t, err, want)
		})
	}
}

func activityGroup(name, category string) facilitiesModule.ActivityGroupName {
	return facilitiesModule.ActivityGroupName{Name: name, Category: &category}
}

func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
