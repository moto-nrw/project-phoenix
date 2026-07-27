package schedule

// Error-branch unit tests for the staff shift series service (#1889). The
// happy paths live in the hermetic integration suite; these mocks exist to
// exercise every repository-failure return without a database (same pattern
// as staff_shift_service_mock_test.go).

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type seriesMockRepo struct {
	createFn        func(ctx context.Context, series *scheduleModels.StaffShiftSeries) error
	findByIDFn      func(ctx context.Context, id any) (*scheduleModels.StaffShiftSeries, error)
	capValidUntilFn func(ctx context.Context, id int64, until timezone.Date) error
}

func (m *seriesMockRepo) Create(ctx context.Context, series *scheduleModels.StaffShiftSeries) error {
	if m.createFn != nil {
		return m.createFn(ctx, series)
	}
	series.ID = 1
	return nil
}

func (m *seriesMockRepo) FindByID(ctx context.Context, id any) (*scheduleModels.StaffShiftSeries, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, sql.ErrNoRows
}

func (m *seriesMockRepo) Update(context.Context, *scheduleModels.StaffShiftSeries) error { return nil }
func (m *seriesMockRepo) Delete(context.Context, any) error                              { return nil }

func (m *seriesMockRepo) CapValidUntil(ctx context.Context, id int64, until timezone.Date) error {
	if m.capValidUntilFn != nil {
		return m.capValidUntilFn(ctx, id, until)
	}
	return nil
}

func (m *seriesMockRepo) CapAllByStaffID(context.Context, int64, timezone.Date) (int64, error) {
	return 0, nil
}

type seriesMockExceptionRepo struct {
	findDatesFn func(ctx context.Context, seriesID int64) ([]timezone.Date, error)
	repointFn   func(ctx context.Context, fromID, toID int64, from timezone.Date) (int64, error)
}

func (m *seriesMockExceptionRepo) Create(context.Context, *scheduleModels.StaffShiftSeriesException) error {
	return nil
}

func (m *seriesMockExceptionRepo) FindDatesBySeriesID(ctx context.Context, seriesID int64) ([]timezone.Date, error) {
	if m.findDatesFn != nil {
		return m.findDatesFn(ctx, seriesID)
	}
	return nil, nil
}

func (m *seriesMockExceptionRepo) RepointToSeriesFrom(ctx context.Context, fromID, toID int64, from timezone.Date) (int64, error) {
	if m.repointFn != nil {
		return m.repointFn(ctx, fromID, toID, from)
	}
	return 0, nil
}

// seriesShiftMockRepo embeds the shared shiftMockRepo and adds failure
// injection for the series-specific bulk operations.
type seriesShiftMockRepo struct {
	shiftMockRepo
	bulkCreateFn        func(ctx context.Context, shifts []*scheduleModels.StaffShift) error
	deleteNonDetachedFn func(ctx context.Context, seriesID int64, from timezone.Date) (int64, error)
	repointDetachedFn   func(ctx context.Context, fromID, toID int64, from timezone.Date) (int64, error)
	findDetachedFn      func(ctx context.Context, seriesID int64, from timezone.Date) ([]*scheduleModels.StaffShift, error)
	deleteFn            func(ctx context.Context, id any) error
}

func (m *seriesShiftMockRepo) BulkCreate(ctx context.Context, shifts []*scheduleModels.StaffShift) error {
	if m.bulkCreateFn != nil {
		return m.bulkCreateFn(ctx, shifts)
	}
	return nil
}

func (m *seriesShiftMockRepo) DeleteNonDetachedBySeriesFrom(ctx context.Context, seriesID int64, from timezone.Date) (int64, error) {
	if m.deleteNonDetachedFn != nil {
		return m.deleteNonDetachedFn(ctx, seriesID, from)
	}
	return 0, nil
}

func (m *seriesShiftMockRepo) RepointDetachedSeriesFrom(ctx context.Context, fromID, toID int64, from timezone.Date) (int64, error) {
	if m.repointDetachedFn != nil {
		return m.repointDetachedFn(ctx, fromID, toID, from)
	}
	return 0, nil
}

func (m *seriesShiftMockRepo) FindDetachedBySeriesFrom(ctx context.Context, seriesID int64, from timezone.Date) ([]*scheduleModels.StaffShift, error) {
	if m.findDetachedFn != nil {
		return m.findDetachedFn(ctx, seriesID, from)
	}
	return nil, nil
}

func (m *seriesShiftMockRepo) Delete(ctx context.Context, id any) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

type seriesFakePeriodRepo struct {
	scheduleModels.CalendarPeriodRepository
	period *scheduleModels.CalendarPeriod
	err    error
}

func (r seriesFakePeriodRepo) FindByID(context.Context, any) (*scheduleModels.CalendarPeriod, error) {
	return r.period, r.err
}

type seriesServiceMocks struct {
	series     *seriesMockRepo
	exceptions *seriesMockExceptionRepo
	shifts     *seriesShiftMockRepo
	staff      *shiftMockStaffRepo
	period     seriesFakePeriodRepo
}

func newSeriesServiceForTest(m seriesServiceMocks) StaffShiftSeriesService {
	if m.series == nil {
		m.series = &seriesMockRepo{}
	}
	if m.exceptions == nil {
		m.exceptions = &seriesMockExceptionRepo{}
	}
	if m.shifts == nil {
		m.shifts = &seriesShiftMockRepo{}
	}
	if m.staff == nil {
		m.staff = &shiftMockStaffRepo{findByIDFunc: func(context.Context, interface{}) (*usersModels.Staff, error) {
			staff := &usersModels.Staff{}
			staff.ID = 5
			return staff, nil
		}}
	}
	if m.period.period == nil && m.period.err == nil {
		today := timezone.TodayDate()
		m.period.period = &scheduleModels.CalendarPeriod{
			StartDate:       today.AddDays(-30),
			EndDate:         today.AddDays(30),
			WeekCycleLength: 1,
			IsActive:        true,
		}
	}
	// nil db skips the advisory lock (same convention as the single-shift
	// unit tests).
	return NewStaffShiftSeriesService(m.series, m.exceptions, m.shifts, m.staff, m.period, nil, nil, nil)
}

func unitSeries(t *testing.T) *scheduleModels.StaffShiftSeries {
	t.Helper()
	start, err := time.Parse("15:04", "09:00")
	require.NoError(t, err)
	end, err := time.Parse("15:04", "12:00")
	require.NoError(t, err)
	return &scheduleModels.StaffShiftSeries{
		StaffID:          5,
		Weekdays:         []int16{1, 2, 3, 4, 5, 6, 7},
		StartTime:        timezone.WallClock(start),
		EndTime:          timezone.WallClock(end),
		CalendarPeriodID: 8,
		WeekPattern:      scheduleModels.WeekPatternEvery,
		ValidFrom:        timezone.TodayDate().AddDays(-7),
		CreatedBy:        5,
	}
}

func TestCreateSeriesUnit_ErrorBranches(t *testing.T) {
	dbErr := errors.New("db down")

	t.Run("invalid series data", func(t *testing.T) {
		service := newSeriesServiceForTest(seriesServiceMocks{})
		bad := unitSeries(t)
		bad.Weekdays = nil
		_, err := service.CreateSeries(context.Background(), bad)
		assert.ErrorIs(t, err, ErrSeriesInvalid)
	})

	t.Run("staff lookup failure", func(t *testing.T) {
		service := newSeriesServiceForTest(seriesServiceMocks{
			staff: &shiftMockStaffRepo{findByIDFunc: func(context.Context, interface{}) (*usersModels.Staff, error) {
				return nil, dbErr
			}},
		})
		_, err := service.CreateSeries(context.Background(), unitSeries(t))
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrSeriesInvalid)
	})

	t.Run("staff missing", func(t *testing.T) {
		service := newSeriesServiceForTest(seriesServiceMocks{
			staff: &shiftMockStaffRepo{findByIDFunc: func(context.Context, interface{}) (*usersModels.Staff, error) {
				return nil, sql.ErrNoRows
			}},
		})
		_, err := service.CreateSeries(context.Background(), unitSeries(t))
		assert.ErrorIs(t, err, ErrSeriesInvalid)
	})

	t.Run("period lookup failure", func(t *testing.T) {
		service := newSeriesServiceForTest(seriesServiceMocks{period: seriesFakePeriodRepo{err: dbErr}})
		_, err := service.CreateSeries(context.Background(), unitSeries(t))
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrSeriesInvalid)
	})

	t.Run("period missing", func(t *testing.T) {
		service := newSeriesServiceForTest(seriesServiceMocks{period: seriesFakePeriodRepo{err: sql.ErrNoRows}})
		_, err := service.CreateSeries(context.Background(), unitSeries(t))
		assert.ErrorIs(t, err, ErrSeriesInvalid)
	})

	t.Run("series insert failure", func(t *testing.T) {
		service := newSeriesServiceForTest(seriesServiceMocks{
			series: &seriesMockRepo{createFn: func(context.Context, *scheduleModels.StaffShiftSeries) error {
				return dbErr
			}},
		})
		_, err := service.CreateSeries(context.Background(), unitSeries(t))
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("exception read failure", func(t *testing.T) {
		service := newSeriesServiceForTest(seriesServiceMocks{
			exceptions: &seriesMockExceptionRepo{findDatesFn: func(context.Context, int64) ([]timezone.Date, error) {
				return nil, dbErr
			}},
		})
		_, err := service.CreateSeries(context.Background(), unitSeries(t))
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("existing shift read failure", func(t *testing.T) {
		service := newSeriesServiceForTest(seriesServiceMocks{
			shifts: &seriesShiftMockRepo{shiftMockRepo: shiftMockRepo{
				findByStaffAndDateRangeFunc: func(context.Context, int64, timezone.Date, timezone.Date) ([]*scheduleModels.StaffShift, error) {
					return nil, dbErr
				},
			}},
		})
		_, err := service.CreateSeries(context.Background(), unitSeries(t))
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("bulk insert failure", func(t *testing.T) {
		service := newSeriesServiceForTest(seriesServiceMocks{
			shifts: &seriesShiftMockRepo{bulkCreateFn: func(context.Context, []*scheduleModels.StaffShift) error {
				return dbErr
			}},
		})
		_, err := service.CreateSeries(context.Background(), unitSeries(t))
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("empty window materializes nothing", func(t *testing.T) {
		bulkCalled := false
		service := newSeriesServiceForTest(seriesServiceMocks{
			shifts: &seriesShiftMockRepo{bulkCreateFn: func(_ context.Context, shifts []*scheduleModels.StaffShift) error {
				bulkCalled = len(shifts) > 0
				return nil
			}},
		})
		series := unitSeries(t)
		// valid_until tomorrow (exclusive) puts the whole window behind the
		// past boundary: nothing to create, no skipped days.
		until := timezone.TodayDate().AddDays(1)
		series.ValidUntil = &until
		result, err := service.CreateSeries(context.Background(), series)
		require.NoError(t, err)
		assert.Equal(t, 0, result.Created)
		assert.Empty(t, result.SkippedDates)
		assert.False(t, bulkCalled)
	})
}

func splitInput(t *testing.T, seriesID int64) SplitSeriesInput {
	t.Helper()
	start, err := time.Parse("15:04", "10:00")
	require.NoError(t, err)
	end, err := time.Parse("15:04", "14:00")
	require.NoError(t, err)
	return SplitSeriesInput{
		SeriesID:      seriesID,
		EffectiveDate: timezone.TodayDate().AddDays(3),
		StartTime:     timezone.WallClock(start),
		EndTime:       timezone.WallClock(end),
		ActorStaffID:  5,
	}
}

func storedSeries(t *testing.T) *scheduleModels.StaffShiftSeries {
	series := unitSeries(t)
	series.ID = 12
	return series
}

func TestSplitSeriesUnit_ErrorBranches(t *testing.T) {
	dbErr := errors.New("db down")
	found := func(t *testing.T) *seriesMockRepo {
		return &seriesMockRepo{findByIDFn: func(context.Context, any) (*scheduleModels.StaffShiftSeries, error) {
			return storedSeries(t), nil
		}}
	}

	t.Run("unknown id", func(t *testing.T) {
		service := newSeriesServiceForTest(seriesServiceMocks{})
		_, err := service.SplitSeries(context.Background(), splitInput(t, 12))
		assert.ErrorIs(t, err, ErrSeriesNotFound)

		_, err = service.SplitSeries(context.Background(), splitInput(t, 0))
		assert.ErrorIs(t, err, ErrSeriesNotFound)
	})

	t.Run("series lookup failure", func(t *testing.T) {
		service := newSeriesServiceForTest(seriesServiceMocks{
			series: &seriesMockRepo{findByIDFn: func(context.Context, any) (*scheduleModels.StaffShiftSeries, error) {
				return nil, dbErr
			}},
		})
		_, err := service.SplitSeries(context.Background(), splitInput(t, 12))
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrSeriesNotFound)
	})

	t.Run("invalid successor times", func(t *testing.T) {
		service := newSeriesServiceForTest(seriesServiceMocks{series: found(t)})
		input := splitInput(t, 12)
		input.EndTime = input.StartTime
		_, err := service.SplitSeries(context.Background(), input)
		assert.ErrorIs(t, err, ErrSeriesInvalid)
	})

	t.Run("cap failure", func(t *testing.T) {
		repo := found(t)
		repo.capValidUntilFn = func(context.Context, int64, timezone.Date) error { return dbErr }
		service := newSeriesServiceForTest(seriesServiceMocks{series: repo})
		_, err := service.SplitSeries(context.Background(), splitInput(t, 12))
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("delete failure", func(t *testing.T) {
		service := newSeriesServiceForTest(seriesServiceMocks{
			series: found(t),
			shifts: &seriesShiftMockRepo{deleteNonDetachedFn: func(context.Context, int64, timezone.Date) (int64, error) {
				return 0, dbErr
			}},
		})
		_, err := service.SplitSeries(context.Background(), splitInput(t, 12))
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("successor insert failure", func(t *testing.T) {
		repo := found(t)
		repo.createFn = func(context.Context, *scheduleModels.StaffShiftSeries) error { return dbErr }
		service := newSeriesServiceForTest(seriesServiceMocks{series: repo})
		_, err := service.SplitSeries(context.Background(), splitInput(t, 12))
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("detached repoint failure", func(t *testing.T) {
		service := newSeriesServiceForTest(seriesServiceMocks{
			series: found(t),
			shifts: &seriesShiftMockRepo{repointDetachedFn: func(context.Context, int64, int64, timezone.Date) (int64, error) {
				return 0, dbErr
			}},
		})
		_, err := service.SplitSeries(context.Background(), splitInput(t, 12))
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("exception repoint failure", func(t *testing.T) {
		service := newSeriesServiceForTest(seriesServiceMocks{
			series: found(t),
			exceptions: &seriesMockExceptionRepo{repointFn: func(context.Context, int64, int64, timezone.Date) (int64, error) {
				return 0, dbErr
			}},
		})
		_, err := service.SplitSeries(context.Background(), splitInput(t, 12))
		assert.ErrorIs(t, err, dbErr)
	})
}

func TestEndSeriesUnit_ErrorBranches(t *testing.T) {
	dbErr := errors.New("db down")
	found := func(t *testing.T) *seriesMockRepo {
		return &seriesMockRepo{findByIDFn: func(context.Context, any) (*scheduleModels.StaffShiftSeries, error) {
			return storedSeries(t), nil
		}}
	}

	t.Run("cap failure", func(t *testing.T) {
		repo := found(t)
		repo.capValidUntilFn = func(context.Context, int64, timezone.Date) error { return dbErr }
		service := newSeriesServiceForTest(seriesServiceMocks{series: repo})
		_, err := service.EndSeries(context.Background(), 12, timezone.TodayDate().AddDays(3))
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("delete failure", func(t *testing.T) {
		service := newSeriesServiceForTest(seriesServiceMocks{
			series: found(t),
			shifts: &seriesShiftMockRepo{deleteNonDetachedFn: func(context.Context, int64, timezone.Date) (int64, error) {
				return 0, dbErr
			}},
		})
		_, err := service.EndSeries(context.Background(), 12, timezone.TodayDate().AddDays(3))
		assert.ErrorIs(t, err, dbErr)
	})

	t.Run("effective clamps to valid_from and tomorrow", func(t *testing.T) {
		var capped timezone.Date
		repo := found(t)
		repo.capValidUntilFn = func(_ context.Context, _ int64, until timezone.Date) error {
			capped = until
			return nil
		}
		service := newSeriesServiceForTest(seriesServiceMocks{series: repo})
		_, err := service.EndSeries(context.Background(), 12, timezone.TodayDate().AddDays(-30))
		require.NoError(t, err)
		assert.Equal(t, timezone.TodayDate().AddDays(1), capped,
			"an end date in the past must clamp to tomorrow (past rows stay)")
	})
}

// GetSeries is what the series editor reads before it writes an edit back
// through the split (#2028): it must hand out the stored rule unchanged and
// report a missing series as ErrSeriesNotFound, never as a nil rule.
func TestGetSeriesUnit(t *testing.T) {
	t.Run("returns the stored rule", func(t *testing.T) {
		stored := storedSeries(t)
		stored.Weekdays = []int16{2, 4}
		stored.WeekPattern = scheduleModels.WeekPatternA
		until := timezone.TodayDate().AddDays(14)
		stored.ValidUntil = &until

		service := newSeriesServiceForTest(seriesServiceMocks{
			series: &seriesMockRepo{findByIDFn: func(_ context.Context, id any) (*scheduleModels.StaffShiftSeries, error) {
				assert.Equal(t, int64(12), id)
				return stored, nil
			}},
		})

		got, err := service.GetSeries(context.Background(), stored.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, stored.ID, got.ID)
		assert.Equal(t, []int16{2, 4}, got.Weekdays)
		assert.Equal(t, scheduleModels.WeekPatternA, got.WeekPattern)
		require.NotNil(t, got.ValidUntil)
		assert.Equal(t, until, *got.ValidUntil)
	})

	t.Run("unknown id", func(t *testing.T) {
		service := newSeriesServiceForTest(seriesServiceMocks{})
		_, err := service.GetSeries(context.Background(), 12)
		assert.ErrorIs(t, err, ErrSeriesNotFound)

		_, err = service.GetSeries(context.Background(), 0)
		assert.ErrorIs(t, err, ErrSeriesNotFound)
	})

	t.Run("lookup failure", func(t *testing.T) {
		dbErr := errors.New("db down")
		service := newSeriesServiceForTest(seriesServiceMocks{
			series: &seriesMockRepo{findByIDFn: func(context.Context, any) (*scheduleModels.StaffShiftSeries, error) {
				return nil, dbErr
			}},
		})
		_, err := service.GetSeries(context.Background(), 12)
		assert.ErrorIs(t, err, dbErr)
		assert.NotErrorIs(t, err, ErrSeriesNotFound)
	})
}
