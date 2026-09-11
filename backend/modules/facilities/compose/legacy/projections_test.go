package legacy

import (
	"context"
	"errors"
	"testing"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type occupancyGroupsStub struct {
	activeModels.GroupRepository
	facts []activeModels.RoomOccupancy
	err   error
}

func (s occupancyGroupsStub) ListRoomOccupancy(context.Context, []int64) ([]activeModels.RoomOccupancy, error) {
	return s.facts, s.err
}

type activityProjectionStub struct {
	groups []*activityModels.Group
	err    error
}

func (s activityProjectionStub) ListWithCategory(context.Context, *activityModels.GroupListQuery) ([]*activityModels.Group, error) {
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
	facts := []activeModels.RoomOccupancy{{
		RoomID: 2, ActivityGroupIDs: []int64{20, 10, 30}, StudentCount: 3,
		SupervisorStaffIDs: []int64{7, 6, 7},
	}}
	activities := activityProjectionStub{groups: []*activityModels.Group{
		activityGroup(20, "Zirkus", "Bewegung"),
		activityGroup(10, "Atelier", "Kreativ"),
	}}
	membership := membershipQueryStub{staff: []schoolmembership.Staff{{ID: 7, PersonID: 70}, {ID: 6, PersonID: 60}}}
	persons := personQueryStub{people: []peopledirectory.Person{
		{ID: 70, FirstName: "Berta", LastName: "Zwei"},
		{ID: 60, FirstName: "Anna", LastName: "Eins"},
	}}

	project := OccupancyProjection(occupancyGroupsStub{facts: facts}, activities, membership, persons)
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
	fact := []activeModels.RoomOccupancy{{RoomID: 1, ActivityGroupIDs: []int64{1}, SupervisorStaffIDs: []int64{2}}}
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
			project := OccupancyProjection(test.groups, test.activities, test.membership, test.persons)
			_, err := project(context.Background(), room)
			assert.ErrorIs(t, err, want)
		})
	}
}

func activityGroup(id int64, name, category string) *activityModels.Group {
	group := &activityModels.Group{Name: name, Category: &activityModels.Category{Name: category}}
	group.ID = id
	return group
}

func pointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
