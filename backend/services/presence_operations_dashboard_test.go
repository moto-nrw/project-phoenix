package services

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type atSchoolCounterFake struct {
	count int
	err   error
	seen  []int64
}

func (f *atSchoolCounterFake) CountAtSchoolToday(_ context.Context, ids []int64) (int, error) {
	f.seen = ids
	return f.count, f.err
}

// TestSplitAtSchoolFromHome pins #3260: the children still in class leave the
// "Zuhause" figure for their own one.
func TestSplitAtSchoolFromHome(t *testing.T) {
	t.Parallel()

	t.Run("moves the counted children out of home", func(t *testing.T) {
		t.Parallel()
		counter := &atSchoolCounterFake{count: 3}
		atSchool, home, err := splitAtSchoolFromHome(context.Background(), counter, 5, []int64{1, 2, 3, 4, 5})
		require.NoError(t, err)
		assert.Equal(t, []int64{1, 2, 3, 4, 5}, counter.seen)
		assert.Equal(t, 3, atSchool)
		assert.Equal(t, 2, home)
	})

	t.Run("clamps home at zero", func(t *testing.T) {
		t.Parallel()
		// StudentsHome is clamped upstream and can be lower than the candidates.
		atSchool, home, err := splitAtSchoolFromHome(context.Background(), &atSchoolCounterFake{count: 2}, 1, []int64{1, 2})
		require.NoError(t, err)
		assert.Equal(t, 2, atSchool)
		assert.Equal(t, 0, home)
	})

	t.Run("skips the counter without candidates", func(t *testing.T) {
		t.Parallel()
		counter := &atSchoolCounterFake{count: 9}
		atSchool, home, err := splitAtSchoolFromHome(context.Background(), counter, 4, nil)
		require.NoError(t, err)
		assert.Nil(t, counter.seen)
		assert.Equal(t, 0, atSchool)
		assert.Equal(t, 4, home)
	})

	t.Run("keeps home without a counter", func(t *testing.T) {
		t.Parallel()
		atSchool, home, err := splitAtSchoolFromHome(context.Background(), nil, 4, []int64{1})
		require.NoError(t, err)
		assert.Equal(t, 0, atSchool)
		assert.Equal(t, 4, home)
	})

	t.Run("fails when the counter fails", func(t *testing.T) {
		t.Parallel()
		boom := errors.New("planning down")
		_, _, err := splitAtSchoolFromHome(context.Background(), &atSchoolCounterFake{err: boom}, 1, []int64{1})
		require.ErrorIs(t, err, boom)
	})
}

func TestSplitAtSchoolOrKeepHome(t *testing.T) {
	t.Parallel()

	atSchool, home := splitAtSchoolOrKeepHome(
		context.Background(),
		&atSchoolCounterFake{err: errors.New("planning down")},
		4,
		[]int64{1},
		slog.Default(),
	)

	assert.Equal(t, 0, atSchool)
	assert.Equal(t, 4, home)
}
