package peopledirectory_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModuleDirectoryFilterIsNormalizedBeforeTheEngine(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := peopledirectory.NewModule(engine)
	ctx := context.Background()

	_, err := module.ListStudentDirectory(ctx, peopledirectory.StudentDirectoryFilter{
		IDs:           []int64{7, 7, 0, 3},
		KeepAlumni:    []int64{9, -1, 9},
		SchoolClasses: []string{"2a", "", "2a"},
		CareStatusOn:  "2026-09-18",
	})
	require.NoError(t, err)
	assert.Equal(t, []int64{7, 3}, engine.directory.IDs, "ids are deduplicated in order")
	assert.Equal(t, []int64{9}, engine.directory.KeepAlumni)
	assert.Equal(t, []string{"2a"}, engine.directory.SchoolClasses)
}

func TestModuleDirectoryCountIgnoresThePageWindow(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := peopledirectory.NewModule(engine)

	_, err := module.CountStudentDirectory(context.Background(), peopledirectory.StudentDirectoryFilter{
		Page: 4, PageSize: 25, CareStatusOn: "2026-09-18",
	})
	require.NoError(t, err)
	assert.Zero(t, engine.directory.Page, "the total names the selection, never the page")
	assert.Zero(t, engine.directory.PageSize)
}

func TestModuleDirectoryFilterRejectsUnusableWindows(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := peopledirectory.NewModule(engine)
	ctx := context.Background()

	for name, filter := range map[string]peopledirectory.StudentDirectoryFilter{
		"negative page":      {Page: -1, CareStatusOn: "2026-09-18"},
		"negative page size": {PageSize: -10, CareStatusOn: "2026-09-18"},
		"unknown care status": {
			CareStatus: "graduated", CareStatusOn: "2026-09-18",
		},
		// Without a day the enrolment boundary would silently widen to every
		// child, which is a data leak dressed as a missing parameter.
		"bounded care status without a day": {CareStatus: peopledirectory.StudentCareStatusEnded},
		"malformed day":                     {CareStatusOn: "18.09.2026"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := module.ListStudentDirectory(ctx, filter)
			require.ErrorIs(t, err, peopledirectory.ErrInvalidStudent)
		})
	}
	assert.Zero(t, engine.calls, "a rejected filter never reaches the engine")
}

func TestModuleDirectoryAllCareStatusNeedsNoDay(t *testing.T) {
	t.Parallel()
	module := peopledirectory.NewModule(&recordingEngine{})

	_, err := module.ListStudentDirectory(context.Background(), peopledirectory.StudentDirectoryFilter{
		CareStatus: peopledirectory.StudentCareStatusAll,
	})
	require.NoError(t, err, "an unbounded selection has no boundary to freeze")
}

func TestModuleRecordReadsValidateBeforeTheEngine(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := peopledirectory.NewModule(engine)
	ctx := context.Background()

	_, err := module.FindStudentRecord(ctx, 0)
	require.ErrorIs(t, err, peopledirectory.ErrInvalidStudent)
	_, err = module.FindStudentRecordForMutation(ctx, -3)
	require.ErrorIs(t, err, peopledirectory.ErrInvalidStudent)

	records, err := module.ListStudentRecordsByID(ctx, []int64{0, -2})
	require.NoError(t, err)
	assert.Empty(t, records)
	assert.Zero(t, engine.calls, "no read of nothing reaches the engine")

	_, err = module.ListStudentRecordsByID(ctx, []int64{8, 8, 2})
	require.NoError(t, err)
	assert.Equal(t, []int64{8, 2}, engine.recordIDs)

	_, err = module.FindStudentRecordForMutation(ctx, 11)
	require.NoError(t, err)
	assert.Equal(t, int64(11), engine.lockedRecord)
}
