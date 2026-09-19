package requestreview

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPolicyKeepsPermissionPrerequisitesAndSettingFailures(t *testing.T) {
	t.Parallel()
	failure := errors.New("setting unavailable")
	for _, tc := range []struct {
		name      string
		principal Principal
		wantError error
		wantScope Scope
	}{
		{"admin bypasses setting", Principal{Admin: true}, nil, Scope{SchoolWide: true}},
		{"read only has no review", Principal{UsersRead: true}, nil, Scope{}},
		{"absence requires read", Principal{UsersAbsence: true}, ErrAbsenceReadRequired, Scope{}},
		{"write needs school opt-in", Principal{UsersUpdate: true}, failure, Scope{}},
		{"absence and read need opt-in", Principal{UsersRead: true, UsersAbsence: true}, failure, Scope{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy, err := New(Dependencies{
				Principal:          func(context.Context) Principal { return tc.principal },
				GroupLeaderEnabled: func(context.Context) (bool, error) { return false, failure },
				GroupIDs:           func(context.Context) ([]int64, error) { t.Fatal("groups must not be loaded"); return nil, nil },
			})
			require.NoError(t, err)
			scope, err := policy.Scope(context.Background())
			if tc.wantError != nil {
				require.ErrorIs(t, err, tc.wantError)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantScope, scope)
			}
		})
	}
}

func TestPolicyDisabledSettingDoesNotResolveGroups(t *testing.T) {
	t.Parallel()
	policy, err := New(Dependencies{
		Principal:          func(context.Context) Principal { return Principal{UsersUpdate: true} },
		GroupLeaderEnabled: func(context.Context) (bool, error) { return false, nil },
		GroupIDs: func(context.Context) ([]int64, error) {
			t.Fatal("disabled setting must not resolve groups")
			return nil, nil
		},
	})
	require.NoError(t, err)
	scope, err := policy.Scope(context.Background())
	require.NoError(t, err)
	require.Equal(t, Scope{}, scope)
}
