package presence

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type atSchoolCounterFake struct {
	count   int
	inClass int
	err     error
	seen    []int64
}

func (f *atSchoolCounterFake) CountAtSchoolToday(_ context.Context, ids []int64) (int, int, error) {
	f.seen = ids
	return f.count, f.inClass, f.err
}

// TestSplitAtSchoolFromHome pins #3260: the children still in class leave the
// "Zuhause" figure for their own one.
func TestSplitAtSchoolFromHome(t *testing.T) {
	t.Parallel()

	t.Run("moves the counted children out of home", func(t *testing.T) {
		t.Parallel()
		counter := &atSchoolCounterFake{count: 3}
		atSchool, _, home, err := splitAtSchoolFromHome(context.Background(), counter, 5, []int64{1, 2, 3, 4, 5})
		require.NoError(t, err)
		assert.Equal(t, []int64{1, 2, 3, 4, 5}, counter.seen)
		assert.Equal(t, 3, atSchool)
		assert.Equal(t, 2, home)
	})

	// #5656: a child in class has not gone home either.
	t.Run("moves the children in class out of home", func(t *testing.T) {
		t.Parallel()
		counter := &atSchoolCounterFake{count: 1, inClass: 2}
		atSchool, inClass, home, err := splitAtSchoolFromHome(context.Background(), counter, 5, []int64{1, 2, 3, 4, 5})
		require.NoError(t, err)
		assert.Equal(t, 1, atSchool)
		assert.Equal(t, 2, inClass)
		assert.Equal(t, 2, home)
	})

	t.Run("clamps home at zero", func(t *testing.T) {
		t.Parallel()
		// A stale home figure can be lower than the at-school count.
		atSchool, _, home, err := splitAtSchoolFromHome(context.Background(), &atSchoolCounterFake{count: 2}, 1, []int64{1, 2})
		require.NoError(t, err)
		assert.Equal(t, 2, atSchool)
		assert.Equal(t, 0, home)
	})

	t.Run("skips the counter without candidates", func(t *testing.T) {
		t.Parallel()
		counter := &atSchoolCounterFake{count: 9}
		atSchool, _, home, err := splitAtSchoolFromHome(context.Background(), counter, 4, nil)
		require.NoError(t, err)
		assert.Nil(t, counter.seen)
		assert.Equal(t, 0, atSchool)
		assert.Equal(t, 4, home)
	})

	t.Run("keeps home without a counter", func(t *testing.T) {
		t.Parallel()
		atSchool, _, home, err := splitAtSchoolFromHome(context.Background(), nil, 4, []int64{1})
		require.NoError(t, err)
		assert.Equal(t, 0, atSchool)
		assert.Equal(t, 4, home)
	})

	t.Run("fails when the counter fails", func(t *testing.T) {
		t.Parallel()
		boom := errors.New("planning down")
		_, _, _, err := splitAtSchoolFromHome(context.Background(), &atSchoolCounterFake{err: boom}, 1, []int64{1})
		require.ErrorIs(t, err, boom)
	})
}

func TestSplitAtSchoolOrKeepHome(t *testing.T) {
	t.Parallel()

	atSchool, inClass, home := splitAtSchoolOrKeepHome(
		context.Background(),
		&atSchoolCounterFake{err: errors.New("planning down")},
		4,
		[]int64{1},
		slog.Default(),
	)

	assert.Equal(t, 0, atSchool)
	assert.Equal(t, 0, inClass)
	assert.Equal(t, 4, home)
}

// inClassCounterFake answers "Im Unterricht" for the listed students and
// "Schule" for every other one, so a test sees which figure is used.
type inClassCounterFake struct {
	inClass map[int64]bool
	calls   int
}

func (f *inClassCounterFake) CountAtSchoolToday(_ context.Context, ids []int64) (int, int, error) {
	f.calls++
	atSchool, inClass := 0, 0
	for _, id := range ids {
		if f.inClass[id] {
			inClass++
		} else {
			atSchool++
		}
	}
	return atSchool, inClass, nil
}

// TestAbsentInClass pins #5710: a sick or excused child staff checked out to
// class counts as "Im Unterricht" only; a derived "Schule" never beats the
// day status.
func TestAbsentInClass(t *testing.T) {
	t.Parallel()

	counter := &inClassCounterFake{inClass: map[int64]bool{1: true, 4: true}}
	sick, excused, err := absentInClass(context.Background(), counter, []int64{1, 2, 3}, []int64{4, 5})
	require.NoError(t, err)
	assert.Equal(t, 1, sick)
	assert.Equal(t, 1, excused)

	none := &inClassCounterFake{}
	sick, excused, err = absentInClass(context.Background(), none, nil, nil)
	require.NoError(t, err)
	assert.Zero(t, sick+excused)
	assert.Zero(t, none.calls)

	sick, excused = absentInClassOrNone(context.Background(), &atSchoolCounterFake{err: errors.New("planning down")}, []int64{1}, nil, slog.Default())
	assert.Zero(t, sick+excused)
}
