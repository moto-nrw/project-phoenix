package grouplive

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// A guest reads the group with groups:read but without users:read (ADR 0091).
// The substitution owner refuses the guest role, so the roster must not ask
// it: the transfers stay empty instead of failing the whole projection.
func TestGuestRosterLoadsWithoutTransfers(t *testing.T) {
	t.Parallel()
	for _, canReadStudentPlans := range []bool{false, true} {
		f := &fakes{
			caller: func(context.Context) (Caller, error) {
				return Caller{CanReadGroups: true, CanReadStudentPlans: canReadStudentPlans}, nil
			},
			supervisedGroups: func(context.Context) ([]GroupRecord, error) { return twoGroups(), nil },
			members: func(context.Context, int64) ([]RosterStudent, error) {
				return []RosterStudent{{ID: 11, FirstName: "Mila"}}, nil
			},
			handovers: func(context.Context, int64, Date) ([]Transfer, error) {
				if !canReadStudentPlans {
					return nil, errors.New("substitution action forbidden")
				}
				return []Transfer{{ID: 5, GroupID: 1, SubstituteStaffID: 9, SubstituteName: "Vera Vertretung"}}, nil
			},
		}

		projection, err := newService(f).LiveGroup(context.Background(), 1)

		require.NoError(t, err)
		require.Len(t, projection.Students, 1)
		if canReadStudentPlans {
			require.Equal(t, 1, f.handoverCalls)
			require.Len(t, projection.Transfers, 1)
		} else {
			require.Zero(t, f.handoverCalls, "a guest sees no substituting staff")
			require.Empty(t, projection.Transfers)
			require.NotNil(t, projection.Transfers)
		}
	}
}
