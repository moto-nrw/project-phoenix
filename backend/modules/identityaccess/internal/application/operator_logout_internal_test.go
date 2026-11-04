package application

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/require"
)

// rotatingLogoutStore models a refresh that already read the family before
// DELETE's snapshot and commits its successor while that DELETE waits.
// Each pass can therefore delete rows and still leave a live successor.
type rotatingLogoutStore struct {
	*fakeStore
	rotationsLeft int
	passes        int
}

func (s *rotatingLogoutStore) DeleteOperatorSessionsByFamily(ctx context.Context, familyID string) ([]domain.OperatorSession, domain.OperationStats, error) {
	s.passes++
	deleted, queryStats, err := s.fakeStore.DeleteOperatorSessionsByFamily(ctx, familyID)
	if err != nil || len(deleted) == 0 || s.rotationsLeft == 0 {
		return deleted, queryStats, err
	}
	s.rotationsLeft--
	successor := deleted[0]
	successor.Generation++
	successor.Token = fmt.Sprintf("logout-race-successor-%d", successor.Generation)
	_, _, err = s.InsertOperatorSession(ctx, successor)
	return deleted, queryStats, err
}

func TestOperatorLogoutRequiresConfirmedRevocation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		rotations int
		confirmed bool
	}{
		{"empty on the final pass", 2, true},
		{"final deletion without confirmation", 3, false},
		{"successor outlives every pass", 4, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			f := newOperatorFixture(t)
			operator := f.seedOperator(42)
			_, _, err := f.auth.IssueTokensForAuthenticatedOperator(ctx, operator.ID, "", "")
			require.NoError(t, err)
			sessions := f.store.operatorSessionsOf(operator.ID)
			require.Len(t, sessions, 1)
			familyID := sessions[0].FamilyID
			store := &rotatingLogoutStore{fakeStore: f.store, rotationsLeft: tc.rotations}
			f.auth.operators.operators = store

			err = f.auth.LogoutOperator(ctx, operator.ID, familyID)
			if tc.confirmed {
				require.NoError(t, err)
				require.Empty(t, f.store.operatorSessionsOf(operator.ID))
			} else {
				require.Error(t, err, "logout must not confirm an unverified revocation")
			}
			require.Equal(t, 4, store.passes, "logout stays bounded")
			if tc.rotations == 4 {
				require.Len(t, f.store.operatorSessionsOf(operator.ID), 1, "a refresh successor survived")
			}

			// The caller can retry after contention stops, including when the
			// last deletion actually emptied the family but could not prove it.
			require.NoError(t, f.auth.LogoutOperator(ctx, operator.ID, familyID))
			require.Empty(t, f.store.operatorSessionsOf(operator.ID))
			require.NoError(t, f.auth.LogoutOperator(ctx, operator.ID, familyID), "already revoked is idempotent")
		})
	}
}
