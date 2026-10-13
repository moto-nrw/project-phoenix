package timetracking

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type exportStaffQueryFunc func(context.Context) ([]TimeExportStaff, error)

func (f exportStaffQueryFunc) ListExportStaff(ctx context.Context) ([]TimeExportStaff, error) {
	return f(ctx)
}

func TestDayExport_StaffLookupFailureStopsExport(t *testing.T) {
	t.Parallel()
	failure := errors.New("staff lookup failed")
	svc := &staffTimeExportService{staffRepo: exportStaffQueryFunc(func(context.Context) ([]TimeExportStaff, error) {
		return []TimeExportStaff{{ID: 42}}, failure
	})}
	file, rows, staff, err := svc.exportDays(context.Background(), TimeExportRequest{}, Date("2026-07-01"), Date("2026-07-31"))
	require.ErrorIs(t, err, failure)
	assert.ErrorContains(t, err, "failed to list staff for export")
	assert.Nil(t, file)
	assert.Zero(t, rows)
	assert.Zero(t, staff)
}

func TestTimeExportStaffOrder_PreservesCaseInsensitiveNamesAndIDTieBreak(t *testing.T) {
	t.Parallel()
	staff := []TimeExportStaff{
		{ID: 9, FirstName: "Anna", LastName: "Zorn"},
		{ID: 5, FirstName: "Ben", LastName: "Adler"},
		{ID: 4, FirstName: "BEN", LastName: "adler"},
		{ID: 8},
		{ID: 6, FirstName: "Anna", LastName: "Adler"},
	}
	slices.SortStableFunc(staff, compareExportStaff)
	ids := make([]int64, len(staff))
	for i, row := range staff {
		ids[i] = row.ID
	}
	assert.Equal(t, []int64{8, 6, 4, 5, 9}, ids)
}
