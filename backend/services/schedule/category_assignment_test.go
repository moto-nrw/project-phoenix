package schedule

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type assignableCategoryRepo struct {
	activitiesModel.CategoryRepository
	category *activitiesModel.Category
	err      error
}

func (r *assignableCategoryRepo) FindByIDForShare(context.Context, int64) (*activitiesModel.Category, error) {
	return r.category, r.err
}

func TestValidateAssignableCategory(t *testing.T) {
	categoryID := time.Now().UnixNano()
	t.Run("accepts an active category", func(t *testing.T) {
		err := validateAssignableCategory(
			context.Background(),
			&assignableCategoryRepo{category: &activitiesModel.Category{Name: "Aktiv"}},
			categoryID,
			"test",
		)
		require.NoError(t, err)
	})

	t.Run("rejects an archived category", func(t *testing.T) {
		archivedAt := time.Now()
		err := validateAssignableCategory(
			context.Background(),
			&assignableCategoryRepo{category: &activitiesModel.Category{Name: "Archiviert", ArchivedAt: &archivedAt}},
			categoryID,
			"test",
		)
		require.ErrorIs(t, err, ErrCategoryNotAssignable)
	})

	t.Run("rejects an unknown tenant-scoped category", func(t *testing.T) {
		err := validateAssignableCategory(
			context.Background(),
			&assignableCategoryRepo{err: sql.ErrNoRows},
			categoryID,
			"test",
		)
		require.ErrorIs(t, err, ErrCategoryNotAssignable)
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
		assert.NotErrorIs(t, err, ErrCategoryNotAssignable)
	})
}
