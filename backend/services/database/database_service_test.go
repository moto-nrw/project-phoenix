package database

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixedCount(value int, err error) func(context.Context) (int, error) {
	return func(context.Context) (int, error) { return value, err }
}

type testStatsCapabilities StatsPermissions

func (c testStatsCapabilities) ViewStudents() bool          { return c.CanViewStudents }
func (c testStatsCapabilities) ViewTeachers() bool          { return c.CanViewTeachers }
func (c testStatsCapabilities) ViewRooms() bool             { return c.CanViewRooms }
func (c testStatsCapabilities) ViewActivities() bool        { return c.CanViewActivities }
func (c testStatsCapabilities) ViewGroups() bool            { return c.CanViewGroups }
func (c testStatsCapabilities) ViewRoles() bool             { return c.CanViewRoles }
func (c testStatsCapabilities) ViewDevices() bool           { return c.CanViewDevices }
func (c testStatsCapabilities) ViewPermissionCatalog() bool { return c.CanViewPermissions }
func (c testStatsCapabilities) ViewTimetables() bool        { return c.CanViewTimetables }
func (c testStatsCapabilities) ViewGradeTransitions() bool  { return c.CanViewGradeTransitions }

func TestDatabaseService_GetStatsHonorsCapabilities(t *testing.T) {
	t.Parallel()
	service := NewService(StatsDependencies{
		Students: fixedCount(12, nil),
		Rooms:    fixedCount(4, nil),
		Teachers: func(context.Context) (int, error) {
			t.Fatal("hidden collector was called")
			return 0, nil
		},
	}, nil)

	result, err := service.GetStats(context.Background(), testStatsCapabilities{CanViewStudents: true, CanViewRooms: true, CanViewTimetables: true})

	require.NoError(t, err)
	assert.Equal(t, 12, result.Students)
	assert.Equal(t, 4, result.Rooms)
	assert.Zero(t, result.Teachers)
	assert.True(t, result.Permissions.CanViewTimetables)
}

func TestDatabaseService_GetStatsKeepsVisibleCardOnCountFailure(t *testing.T) {
	t.Parallel()
	service := NewService(StatsDependencies{
		Students: fixedCount(0, errors.New("database unavailable")),
	}, nil)

	result, err := service.GetStats(context.Background(), testStatsCapabilities{CanViewStudents: true})

	require.NoError(t, err)
	assert.True(t, result.Permissions.CanViewStudents)
	assert.Zero(t, result.Students)
}
