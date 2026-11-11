package guardianhold_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/guardianhold"
	"github.com/stretchr/testify/require"
)

func batchFamily(f *fakeFamily) guardianhold.BatchLocks {
	return guardianhold.BatchLocks{
		Locks: f.locks(),
		Children: func(_ context.Context, ids []int64) error {
			f.calls = append(f.calls, fmt.Sprintf("children %v", ids))
			return f.childError
		},
		Relationships: func(_ context.Context, ids []int64, profile int64) (map[int64]func(string) bool, error) {
			f.calls = append(f.calls, fmt.Sprintf("relationships %v/%d", ids, profile))
			if !f.linked {
				return nil, nil
			}
			return map[int64]func(string) bool{3: func(p string) bool { return f.granted[p] }, 5: func(string) bool { return false }}, nil
		},
	}
}

func TestHoldManyLocksAndChecksOneIdentityForAllSiblings(t *testing.T) {
	t.Parallel()
	f := newFamily(7)
	ids := []int64{9, 3, 5, 3}
	grants, err := guardianhold.HoldMany(context.Background(), batchFamily(f), guardianhold.Request{Email: f.email, ProfileID: 20, Permissions: []string{"parent_portal.enrollment.submit"}}, ids)
	require.NoError(t, err)
	require.Equal(t, map[int64]bool{3: true, 5: false, 9: false}, grants, "missing relationships and missing permissions fail closed")
	require.Equal(t, []string{"owner 20", "membership 7", "children [3 5 9]", "profile 20", "relationships [3 5 9]/20", "owner 20"}, f.calls)
	require.Equal(t, []int64{9, 3, 5, 3}, ids)
}

func TestHoldManyRefusesWithdrawnIdentityAndPropagatesLockErrors(t *testing.T) {
	t.Parallel()
	req := guardianhold.Request{Email: "eva@example.test", ProfileID: 20, Permissions: []string{"parent_portal.enrollment.submit"}}
	failure := errors.New("lock failed")
	for _, tc := range []struct {
		name   string
		change func(*fakeFamily)
		want   error
	}{
		{"membership", func(f *fakeFamily) { f.member = false }, guardianhold.ErrNotLinked},
		{"profile", func(f *fakeFamily) { f.missing[20] = true }, guardianhold.ErrNotLinked},
		{"binding", func(f *fakeFamily) { f.email = "changed@example.test" }, guardianhold.ErrNotLinked},
		{"child lock", func(f *fakeFamily) { f.childError = failure }, failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFamily(7)
			tc.change(f)
			grants, err := guardianhold.HoldMany(context.Background(), batchFamily(f), req, []int64{3, 5})
			require.ErrorIs(t, err, tc.want)
			require.Empty(t, grants)
		})
	}
	f := newFamily(7)
	locks := batchFamily(f)
	locks.Relationships = nil
	_, err := guardianhold.HoldMany(context.Background(), locks, req, []int64{3})
	require.ErrorIs(t, err, guardianhold.ErrUnavailable)
}
