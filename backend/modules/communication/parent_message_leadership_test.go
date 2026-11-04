package communication_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type leadershipStaffFacts struct {
	leadership, readers       []communication.ParentMessageLeader
	leadershipErr, readersErr error
}

func (s leadershipStaffFacts) StaffWithLeadershipPermission(context.Context) ([]communication.ParentMessageLeader, error) {
	return s.leadership, s.leadershipErr
}

func (s leadershipStaffFacts) StaffWithMessageReadPermission(context.Context) ([]communication.ParentMessageLeader, error) {
	return s.readers, s.readersErr
}

type leadershipAdminFacts struct {
	ids []int64
	err error
}

func (a leadershipAdminFacts) ListEffectiveAdminAccountIDs(context.Context) ([]int64, error) {
	return a.ids, a.err
}

func TestParentMessageLeadershipCombinesEffectiveReadersOnce(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	namedAccount := testpkg.CreateTestAccount(t, db, "leadership-named")
	otherAccount := testpkg.CreateTestAccount(t, db, "leadership-other")
	leadershipOnly := testpkg.CreateTestAccount(t, db, "leadership-only")
	readOnly := testpkg.CreateTestAccount(t, db, "read-only")
	office := testpkg.CreateTestAccount(t, db, "leadership-office")
	named := communication.ParentMessageLeader{AccountID: namedAccount.ID, FirstName: "Anna", LastName: "Meyer"}
	other := communication.ParentMessageLeader{AccountID: otherAccount.ID, FirstName: "Berta", LastName: "Meyer"}
	policy := communication.NewParentMessageLeadership(leadershipStaffFacts{
		leadership: []communication.ParentMessageLeader{other, {AccountID: leadershipOnly.ID}, named, named, {}},
		readers:    []communication.ParentMessageLeader{named, other, {AccountID: readOnly.ID}, {}},
	}, leadershipAdminFacts{ids: []int64{office.ID, namedAccount.ID, office.ID, 0}})

	leaders, err := policy.ParentMessageLeaders(testpkg.Ctx(t))
	require.NoError(t, err)
	require.Equal(t, []communication.ParentMessageLeader{named, other, {AccountID: office.ID}}, leaders,
		"both permissions are required for staff; admins are added once without losing staff names")
	ids, err := policy.LeadershipAccountIDs(testpkg.Ctx(t))
	require.NoError(t, err)
	require.Equal(t, []int64{namedAccount.ID, otherAccount.ID, office.ID}, ids,
		"push recipients are exactly the sorted conversation readers")
}

func TestParentMessageLeadershipPropagatesReadFailures(t *testing.T) {
	t.Parallel()
	failure := errors.New("directory unavailable")
	for _, stage := range []string{"leadership", "readers", "admins"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			staff := leadershipStaffFacts{}
			admins := leadershipAdminFacts{}
			switch stage {
			case "leadership":
				staff.leadershipErr = failure
			case "readers":
				staff.readersErr = failure
			case "admins":
				admins.err = failure
			}
			policy := communication.NewParentMessageLeadership(staff, admins)
			leaders, err := policy.ParentMessageLeaders(t.Context())
			require.ErrorIs(t, err, failure)
			require.Nil(t, leaders)
			ids, err := policy.LeadershipAccountIDs(t.Context())
			require.ErrorIs(t, err, failure)
			require.Nil(t, ids)
		})
	}
}
