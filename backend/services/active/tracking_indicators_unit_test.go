package active

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type trackingPresenceSource struct {
	StudentPresence
	names func(context.Context, []int64) ([]visitGroupNames, error)
	rows  []visitGroupNames
}

func (p *trackingPresenceSource) ListVisitLocations(ctx context.Context, filter studentpresence.VisitLocationFilter) ([]studentpresence.VisitLocation, error) {
	rows, err := p.names(ctx, filter.StudentIDs)
	p.rows = rows
	result := make([]studentpresence.VisitLocation, 0, len(rows))
	for index, row := range rows {
		id := int64(index + 1)
		result = append(result, studentpresence.VisitLocation{Visit: studentpresence.Visit{StudentID: row.StudentID}, Group: &studentpresence.VisitGroup{RoomID: id, TemplateID: &id}})
	}
	return result, err
}

type trackingRooms struct {
	facilities.RoomRepository
	source *trackingPresenceSource
}

func (r trackingRooms) FindByIDs(context.Context, []int64) ([]*facilities.Room, error) {
	rows := make([]*facilities.Room, 0, len(r.source.rows))
	for index, row := range r.source.rows {
		rows = append(rows, &facilities.Room{ID: int64(index + 1), Name: row.RoomName})
	}
	return rows, nil
}

type trackingGroups struct {
	activities.GroupRepository
	source *trackingPresenceSource
}

func (r trackingGroups) FindByIDs(context.Context, []int64) ([]*activities.Group, error) {
	rows := make([]*activities.Group, 0, len(r.source.rows))
	for index, row := range r.source.rows {
		rows = append(rows, &activities.Group{Model: activities.Model{ID: int64(index + 1)}, Name: row.ActivityGroupName})
	}
	return rows, nil
}

func trackingService(names func(context.Context, []int64) ([]visitGroupNames, error)) *service {
	presence := &trackingPresenceSource{names: names}
	return &service{ServiceDependencies: ServiceDependencies{
		SchoolPresence: presence, RoomRepo: trackingRooms{source: presence}, ActivityGroupRepo: trackingGroups{source: presence},
	}}
}

func TestGetTrackingIndicators_EmptyStudentIDs(t *testing.T) {
	t.Parallel()

	svc := &service{}

	result, err := svc.GetTrackingIndicators(context.Background(), []int64{}, []string{"Mensa"})

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestGetTrackingIndicators_EmptyLabels(t *testing.T) {
	t.Parallel()

	svc := &service{}

	result, err := svc.GetTrackingIndicators(context.Background(), []int64{100}, []string{})

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestGetTrackingIndicators_RepoError(t *testing.T) {
	t.Parallel()

	visitRepo := &mockVisitRepository{
		getTodayVisitNamesFunc: func(ctx context.Context, studentIDs []int64) ([]visitGroupNames, error) {
			return nil, errors.New("database connection lost")
		},
	}

	svc := trackingService(visitRepo.getTodayVisitNamesFunc)

	result, err := svc.GetTrackingIndicators(context.Background(), []int64{100}, []string{"Mensa"})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "GetTrackingIndicators")
}

func TestGetTrackingIndicators_NoVisits(t *testing.T) {
	t.Parallel()

	visitRepo := &mockVisitRepository{
		getTodayVisitNamesFunc: func(ctx context.Context, studentIDs []int64) ([]visitGroupNames, error) {
			return nil, nil
		},
	}

	svc := trackingService(visitRepo.getTodayVisitNamesFunc)

	result, err := svc.GetTrackingIndicators(context.Background(), []int64{100, 200}, []string{"Mensa", "Hausaufgaben"})

	require.NoError(t, err)
	assert.Len(t, result, 2)
	// All false because no visits
	assert.Equal(t, []bool{false, false}, result[100])
	assert.Equal(t, []bool{false, false}, result[200])
}

func TestGetTrackingIndicators_SubstringMatching(t *testing.T) {
	t.Parallel()

	visitRepo := &mockVisitRepository{
		getTodayVisitNamesFunc: func(ctx context.Context, studentIDs []int64) ([]visitGroupNames, error) {
			return []visitGroupNames{
				{StudentID: 100, ActivityGroupName: "Hausaufgaben Gruppe A", RoomName: "Raum 101"},
			}, nil
		},
	}

	svc := trackingService(visitRepo.getTodayVisitNamesFunc)

	result, err := svc.GetTrackingIndicators(context.Background(), []int64{100}, []string{"Hausaufgaben"})

	require.NoError(t, err)
	assert.Equal(t, []bool{true}, result[100])
}

func TestGetTrackingIndicators_CaseInsensitive(t *testing.T) {
	t.Parallel()

	visitRepo := &mockVisitRepository{
		getTodayVisitNamesFunc: func(ctx context.Context, studentIDs []int64) ([]visitGroupNames, error) {
			return []visitGroupNames{
				{StudentID: 100, ActivityGroupName: "MENSA Gruppe", RoomName: "Speisesaal"},
			}, nil
		},
	}

	svc := trackingService(visitRepo.getTodayVisitNamesFunc)

	result, err := svc.GetTrackingIndicators(context.Background(), []int64{100}, []string{"mensa"})

	require.NoError(t, err)
	assert.Equal(t, []bool{true}, result[100])
}

func TestGetTrackingIndicators_MatchesRoomName(t *testing.T) {
	t.Parallel()

	visitRepo := &mockVisitRepository{
		getTodayVisitNamesFunc: func(ctx context.Context, studentIDs []int64) ([]visitGroupNames, error) {
			return []visitGroupNames{
				{StudentID: 100, ActivityGroupName: "Gruppe B", RoomName: "Mensa"},
			}, nil
		},
	}

	svc := trackingService(visitRepo.getTodayVisitNamesFunc)

	result, err := svc.GetTrackingIndicators(context.Background(), []int64{100}, []string{"Mensa"})

	require.NoError(t, err)
	assert.Equal(t, []bool{true}, result[100])
}

func TestGetTrackingIndicators_MultipleLabelsPartialMatch(t *testing.T) {
	t.Parallel()

	visitRepo := &mockVisitRepository{
		getTodayVisitNamesFunc: func(ctx context.Context, studentIDs []int64) ([]visitGroupNames, error) {
			return []visitGroupNames{
				{StudentID: 100, ActivityGroupName: "Hausaufgaben", RoomName: "Raum 1"},
				{StudentID: 100, ActivityGroupName: "Sport", RoomName: "Turnhalle"},
			}, nil
		},
	}

	svc := trackingService(visitRepo.getTodayVisitNamesFunc)

	result, err := svc.GetTrackingIndicators(
		context.Background(),
		[]int64{100},
		[]string{"Hausaufgaben", "Mensa", "Sport"},
	)

	require.NoError(t, err)
	assert.Equal(t, []bool{true, false, true}, result[100])
}

func TestGetTrackingIndicators_MultipleStudents(t *testing.T) {
	t.Parallel()

	visitRepo := &mockVisitRepository{
		getTodayVisitNamesFunc: func(ctx context.Context, studentIDs []int64) ([]visitGroupNames, error) {
			return []visitGroupNames{
				{StudentID: 100, ActivityGroupName: "Mensa", RoomName: "Speisesaal"},
				{StudentID: 200, ActivityGroupName: "Hausaufgaben", RoomName: "Raum 3"},
			}, nil
		},
	}

	svc := trackingService(visitRepo.getTodayVisitNamesFunc)

	result, err := svc.GetTrackingIndicators(
		context.Background(),
		[]int64{100, 200},
		[]string{"Mensa", "Hausaufgaben"},
	)

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, []bool{true, false}, result[100])
	assert.Equal(t, []bool{false, true}, result[200])
}

func TestGetTrackingIndicators_StudentWithNoVisits(t *testing.T) {
	t.Parallel()

	visitRepo := &mockVisitRepository{
		getTodayVisitNamesFunc: func(ctx context.Context, studentIDs []int64) ([]visitGroupNames, error) {
			// Only student 100 has visits, student 200 has none
			return []visitGroupNames{
				{StudentID: 100, ActivityGroupName: "Mensa", RoomName: "Speisesaal"},
			}, nil
		},
	}

	svc := trackingService(visitRepo.getTodayVisitNamesFunc)

	result, err := svc.GetTrackingIndicators(
		context.Background(),
		[]int64{100, 200},
		[]string{"Mensa"},
	)

	require.NoError(t, err)
	assert.Equal(t, []bool{true}, result[100])
	assert.Equal(t, []bool{false}, result[200])
}

func TestGetTrackingIndicators_WhitespaceInLabels(t *testing.T) {
	t.Parallel()

	visitRepo := &mockVisitRepository{
		getTodayVisitNamesFunc: func(ctx context.Context, studentIDs []int64) ([]visitGroupNames, error) {
			return []visitGroupNames{
				{StudentID: 100, ActivityGroupName: "Hausaufgaben", RoomName: "Raum 1"},
			}, nil
		},
	}

	svc := trackingService(visitRepo.getTodayVisitNamesFunc)

	result, err := svc.GetTrackingIndicators(
		context.Background(),
		[]int64{100},
		[]string{"  Hausaufgaben  "},
	)

	require.NoError(t, err)
	assert.Equal(t, []bool{true}, result[100])
}
