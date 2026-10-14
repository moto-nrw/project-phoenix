package shiftplanning_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/workforce/workforcetest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The typed root composes the shift-type administration the internal route
// tests of this directory run over. It imports their package, so this
// external test file binds the seam they read.
func init() {
	workforcetest.ComposeShiftTypes = func(t *testing.T) (*bun.DB, workforcetest.ShiftTypes) {
		t.Helper()
		db := testpkg.SetupTestDB(t)
		module, err := api.NewShiftTypeTestModule(testpkg.NewDatabase(t, db))
		require.NoError(t, err)
		return db, workforcetest.ShiftTypes{
			Administration: module.ShiftTypes,
			CategoryShiftType: func(ctx context.Context, categoryID int64) (*int64, error) {
				category, err := module.Repositories.Rows.ActivityCategory.FindByID(ctx, categoryID)
				if err != nil {
					return nil, err
				}
				return category.ShiftTypeID, nil
			},
		}
	}
}
