package owners

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMealBillingAttendanceCutoffMatchesCleanup(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		days   int
		cutoff calendar.Date
	}{{30, "2026-10-09"}, {365, "2025-11-08"}} {
		find := mealBillingAvailableFrom(func(context.Context) (int, error) { return scenario.days, nil },
			func() calendar.Date { return "2026-11-08" })
		cutoff, err := find(context.Background())
		require.NoError(t, err)
		assert.Equal(t, scenario.cutoff, cutoff)
	}
	unavailable := errors.New("setting unavailable")
	find := mealBillingAvailableFrom(func(context.Context) (int, error) { return 0, unavailable },
		func() calendar.Date { return "2026-11-08" })
	_, err := find(context.Background())
	require.ErrorIs(t, err, unavailable)
}

// settingsByTenant answers ResolveManyForTenants with one stored retention
// per school, as the settings of the schools hold them.
type settingsByTenant struct {
	days map[int64]string
}

func (s settingsByTenant) ResolveManyForTenants(_ context.Context, tenantIDs []int64, keys []string) (map[int64]*configModel.SettingsSnapshot, error) {
	result := map[int64]*configModel.SettingsSnapshot{}
	for _, id := range tenantIDs {
		days, ok := s.days[id]
		if !ok {
			continue
		}
		stored := []*configModel.SettingValue{{TenantID: id, SettingKey: keys[0], Value: json.RawMessage(days)}}
		snapshot, err := configModel.NewSettingsSnapshot(configModel.DefaultRegistry(), id, keys, stored)
		if err != nil {
			return nil, err
		}
		result[id] = snapshot
	}
	return result, nil
}

// The Träger side of the Mensa billing applies the cutoff of the school
// side to each school's own retention (#4686).
func TestMealAttendanceFromNamesTheCutoffOfEachSchool(t *testing.T) {
	t.Parallel()
	today := func() calendar.Date { return "2026-11-08" }
	find := MealAttendanceFrom(settingsByTenant{days: map[int64]string{1: "30", 2: "365"}}, today)
	cutoffs, err := find(context.Background(), []int64{1, 2})
	require.NoError(t, err)
	assert.Equal(t, map[int64]calendar.Date{1: "2026-10-09", 2: "2025-11-08"}, cutoffs)

	_, err = find(context.Background(), []int64{3})
	require.Error(t, err, "a school without settings is no silent default")

	empty, err := find(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}
