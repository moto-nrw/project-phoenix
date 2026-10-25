package grouplive

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAbsencePermissionProtectsLegacyAndEffectiveStatuses(t *testing.T) {
	t.Parallel()
	since := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	for _, allowed := range []bool{false, true} {
		for _, effective := range []bool{false, true} {
			f := &fakes{
				caller:           func(context.Context) (Caller, error) { return Caller{CanReadStudentPlans: allowed}, nil },
				supervisedGroups: func(context.Context) ([]GroupRecord, error) { return twoGroups(), nil },
				members: func(context.Context, int64) ([]RosterStudent, error) {
					return []RosterStudent{{ID: 11, Sick: !effective, SickSince: &since}, {ID: 12, Excused: !effective, ExcusedSince: &since}}, nil
				},
				statuses: func(context.Context, []int64, Date) (map[int64]EffectiveStatus, error) {
					if !allowed {
						t.Error("guest must not load absence statuses")
					}
					return map[int64]EffectiveStatus{11: {Sick: effective, SickSince: &since}, 12: {Excused: effective, ExcusedSince: &since}}, nil
				},
			}
			got, err := newService(f).LiveGroup(context.Background(), 1)
			require.NoError(t, err)
			require.Len(t, got.Students, 2)
			require.Equal(t, allowed, got.Students[0].Sick)
			require.Equal(t, allowed, got.Students[1].Excused)
			if allowed {
				require.Equal(t, &since, got.Students[0].SickSince)
				require.Equal(t, &since, got.Students[1].ExcusedSince)
			} else {
				require.Nil(t, got.Students[0].SickSince)
				require.Nil(t, got.Students[1].ExcusedSince)
			}
		}
	}
}
