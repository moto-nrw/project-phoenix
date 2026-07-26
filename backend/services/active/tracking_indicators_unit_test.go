package active

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTrackingIndicators_EmptyStudentIDs(t *testing.T) {
	svc := &service{}

	result, err := svc.GetTrackingIndicators(context.Background(), []int64{}, []string{"Mensa"})

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestGetTrackingIndicators_EmptyLabels(t *testing.T) {
	svc := &service{}

	result, err := svc.GetTrackingIndicators(context.Background(), []int64{100}, []string{})

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestGetTrackingIndicators_RepoError(t *testing.T) {
	visitRepo := &mockVisitRepository{
		getTodayVisitNamesFunc: func(ctx context.Context, studentIDs []int64) ([]active.VisitGroupNames, error) {
			return nil, errors.New("database connection lost")
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}

	result, err := svc.GetTrackingIndicators(context.Background(), []int64{100}, []string{"Mensa"})

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "GetTrackingIndicators")
}

func TestGetTrackingIndicators_NoVisits(t *testing.T) {
	visitRepo := &mockVisitRepository{
		getTodayVisitNamesFunc: func(ctx context.Context, studentIDs []int64) ([]active.VisitGroupNames, error) {
			return nil, nil
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}

	result, err := svc.GetTrackingIndicators(context.Background(), []int64{100, 200}, []string{"Mensa", "Hausaufgaben"})

	require.NoError(t, err)
	assert.Len(t, result, 2)
	// All false because no visits
	assert.Equal(t, []bool{false, false}, result[100])
	assert.Equal(t, []bool{false, false}, result[200])
}

func TestGetTrackingIndicators_SubstringMatching(t *testing.T) {
	visitRepo := &mockVisitRepository{
		getTodayVisitNamesFunc: func(ctx context.Context, studentIDs []int64) ([]active.VisitGroupNames, error) {
			return []active.VisitGroupNames{
				{StudentID: 100, ActivityGroupName: "Hausaufgaben Gruppe A", RoomName: "Raum 101"},
			}, nil
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}

	result, err := svc.GetTrackingIndicators(context.Background(), []int64{100}, []string{"Hausaufgaben"})

	require.NoError(t, err)
	assert.Equal(t, []bool{true}, result[100])
}

func TestGetTrackingIndicators_CaseInsensitive(t *testing.T) {
	visitRepo := &mockVisitRepository{
		getTodayVisitNamesFunc: func(ctx context.Context, studentIDs []int64) ([]active.VisitGroupNames, error) {
			return []active.VisitGroupNames{
				{StudentID: 100, ActivityGroupName: "MENSA Gruppe", RoomName: "Speisesaal"},
			}, nil
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}

	result, err := svc.GetTrackingIndicators(context.Background(), []int64{100}, []string{"mensa"})

	require.NoError(t, err)
	assert.Equal(t, []bool{true}, result[100])
}

func TestGetTrackingIndicators_MatchesRoomName(t *testing.T) {
	visitRepo := &mockVisitRepository{
		getTodayVisitNamesFunc: func(ctx context.Context, studentIDs []int64) ([]active.VisitGroupNames, error) {
			return []active.VisitGroupNames{
				{StudentID: 100, ActivityGroupName: "Gruppe B", RoomName: "Mensa"},
			}, nil
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}

	result, err := svc.GetTrackingIndicators(context.Background(), []int64{100}, []string{"Mensa"})

	require.NoError(t, err)
	assert.Equal(t, []bool{true}, result[100])
}

func TestGetTrackingIndicators_MultipleLabelsPartialMatch(t *testing.T) {
	visitRepo := &mockVisitRepository{
		getTodayVisitNamesFunc: func(ctx context.Context, studentIDs []int64) ([]active.VisitGroupNames, error) {
			return []active.VisitGroupNames{
				{StudentID: 100, ActivityGroupName: "Hausaufgaben", RoomName: "Raum 1"},
				{StudentID: 100, ActivityGroupName: "Sport", RoomName: "Turnhalle"},
			}, nil
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}

	result, err := svc.GetTrackingIndicators(
		context.Background(),
		[]int64{100},
		[]string{"Hausaufgaben", "Mensa", "Sport"},
	)

	require.NoError(t, err)
	assert.Equal(t, []bool{true, false, true}, result[100])
}

func TestGetTrackingIndicators_MultipleStudents(t *testing.T) {
	visitRepo := &mockVisitRepository{
		getTodayVisitNamesFunc: func(ctx context.Context, studentIDs []int64) ([]active.VisitGroupNames, error) {
			return []active.VisitGroupNames{
				{StudentID: 100, ActivityGroupName: "Mensa", RoomName: "Speisesaal"},
				{StudentID: 200, ActivityGroupName: "Hausaufgaben", RoomName: "Raum 3"},
			}, nil
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}

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
	visitRepo := &mockVisitRepository{
		getTodayVisitNamesFunc: func(ctx context.Context, studentIDs []int64) ([]active.VisitGroupNames, error) {
			// Only student 100 has visits, student 200 has none
			return []active.VisitGroupNames{
				{StudentID: 100, ActivityGroupName: "Mensa", RoomName: "Speisesaal"},
			}, nil
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}

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
	visitRepo := &mockVisitRepository{
		getTodayVisitNamesFunc: func(ctx context.Context, studentIDs []int64) ([]active.VisitGroupNames, error) {
			return []active.VisitGroupNames{
				{StudentID: 100, ActivityGroupName: "Hausaufgaben", RoomName: "Raum 1"},
			}, nil
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}

	result, err := svc.GetTrackingIndicators(
		context.Background(),
		[]int64{100},
		[]string{"  Hausaufgaben  "},
	)

	require.NoError(t, err)
	assert.Equal(t, []bool{true}, result[100])
}
