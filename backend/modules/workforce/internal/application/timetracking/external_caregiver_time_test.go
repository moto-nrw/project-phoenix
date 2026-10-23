package timetracking

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

type externalCaregiverMonthCloseStaff struct{}

func (externalCaregiverMonthCloseStaff) ListMonthCloseStaff(context.Context) ([]workforce.MonthCloseStaff, error) {
	return []workforce.MonthCloseStaff{{ID: 42}, {ID: 43, ExternalCaregiver: true}, {ID: 41}}, nil
}

// External caregivers (#3823) record no working time: a school-wide month
// close leaves them out.
func TestStaffMonthCloseService_SkipsExternalCaregivers(t *testing.T) {
	t.Parallel()
	repo := &timestampMonthCloseSnapshotRepo{}
	service := NewStaffMonthCloseService(repo, timestampMonthCloseMonthService{}, externalCaregiverMonthCloseStaff{}, &wtmMockSettings{accountStart: "2025-01-01"}, nil)
	result, err := service.CloseMonth(context.Background(), 77, 2025, 8, "Lohnlauf")
	require.NoError(t, err)
	require.Len(t, result.Snapshots, 2)
	assert.Equal(t, []int64{41, 42}, repo.lockedStaff)
}

// The overview and the exports leave external caregivers out as well.
func TestOverviewAndExportStaffSkipExternalCaregivers(t *testing.T) {
	t.Parallel()
	overview := &staffOverviewService{staffRepo: overviewStaffQueryFunc(func(context.Context) ([]OverviewStaff, error) {
		return []OverviewStaff{{ID: 42}, {ID: 43, ExternalCaregiver: true}}, nil
	})}
	members, err := overview.activeStaff(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []OverviewStaff{{ID: 42}}, members)
	assert.Equal(t, []TimeExportStaff{{ID: 42}}, workingExportStaff([]TimeExportStaff{{ID: 43, ExternalCaregiver: true}, {ID: 42}}))
}
