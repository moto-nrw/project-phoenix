package owners

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	configModel "github.com/moto-ogs/project-phoenix/modules/settings/records"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Mensa-Monatsfreigabe asks Student Presence for the first complete
// day under the school's own window and keeps its errors (#4686).
func TestMealBillingAttendanceCutoffMatchesCleanup(t *testing.T) {
	t.Parallel()
	var asked int
	find := mealBillingAvailableFrom(func(context.Context) (int, error) { return 30, nil },
		func(_ context.Context, days int) (calendar.Date, error) {
			asked = days
			return "2026-10-09", nil
		})
	from, err := find(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 30, asked)
	assert.Equal(t, calendar.Date("2026-10-09"), from)

	unavailable := errors.New("setting unavailable")
	find = mealBillingAvailableFrom(func(context.Context) (int, error) { return 0, unavailable },
		func(context.Context, int) (calendar.Date, error) { panic("no window, no question") })
	_, err = find(context.Background())
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

// The Träger side asks each school for its first complete day under that
// school's own retention (#4686).
func TestMealAttendanceFromNamesTheCutoffOfEachSchool(t *testing.T) {
	t.Parallel()
	completeFrom := func(_ context.Context, schoolID int64, days int) (calendar.Date, error) {
		if schoolID == 2 && days == 365 {
			return "2026-10-01", nil
		}
		return calendar.Date(fmt.Sprintf("days-%d", days)), nil
	}
	find := MealAttendanceFrom(settingsByTenant{days: map[int64]string{1: "30", 2: "365"}}, completeFrom)
	cutoffs, err := find(context.Background(), []int64{1, 2})
	require.NoError(t, err)
	assert.Equal(t, map[int64]calendar.Date{1: "days-30", 2: "2026-10-01"}, cutoffs)

	_, err = find(context.Background(), []int64{3})
	require.Error(t, err, "a school without settings is no silent default")

	empty, err := find(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}
