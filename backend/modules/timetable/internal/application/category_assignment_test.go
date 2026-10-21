package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type assignableCategoryRepo struct {
	timetable.CategoryQuery
	category timetable.Category
	err      error
}

func (r *assignableCategoryRepo) FindCategoryForShare(context.Context, int64) (timetable.Category, error) {
	return r.category, r.err
}

func TestValidateAssignableCategory(t *testing.T) {
	t.Parallel()

	categoryID := time.Now().UnixNano()
	t.Run("accepts an active category", func(t *testing.T) {
		err := validateAssignableCategory(
			context.Background(),
			&assignableCategoryRepo{category: timetable.Category{Name: "Aktiv"}},
			categoryID,
			"test",
		)
		require.NoError(t, err)
	})

	t.Run("rejects an archived category", func(t *testing.T) {
		archivedAt := time.Now()
		err := validateAssignableCategory(
			context.Background(),
			&assignableCategoryRepo{category: timetable.Category{Name: "Archiviert", ArchivedAt: &archivedAt}},
			categoryID,
			"test",
		)
		require.ErrorIs(t, err, timetable.ErrCategoryNotAssignable)
	})

	t.Run("rejects an unknown tenant-scoped category", func(t *testing.T) {
		err := validateAssignableCategory(
			context.Background(),
			&assignableCategoryRepo{err: timetable.ErrCategoryNotFound},
			categoryID,
			"test",
		)
		require.ErrorIs(t, err, timetable.ErrCategoryNotAssignable)
	})

	t.Run("preserves repository failures", func(t *testing.T) {
		repoErr := errors.New("database unavailable")
		err := validateAssignableCategory(
			context.Background(),
			&assignableCategoryRepo{err: repoErr},
			categoryID,
			"test",
		)
		assert.ErrorIs(t, err, repoErr)
		assert.NotErrorIs(t, err, timetable.ErrCategoryNotAssignable)
	})
}
