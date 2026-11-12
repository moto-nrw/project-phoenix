package importpkg

import (
	"context"
	"errors"
	"testing"

	importModels "github.com/moto-ogs/project-phoenix/modules/dataimport"
	"github.com/moto-ogs/project-phoenix/modules/dataimport/ports"
	"github.com/stretchr/testify/require"
)

type staffEmailMembership struct {
	ports.StaffMembership
	members []ports.Staff
}

func (m staffEmailMembership) ListStaff(_ context.Context, filter ports.StaffFilter) ([]ports.Staff, error) {
	return m.members, nil
}

func (m staffEmailMembership) CreateStaff(_ context.Context, input ports.CreateStaff) (ports.Staff, error) {
	return ports.Staff{ID: 91, PersonID: input.PersonID}, nil
}

func TestStaffBatchEmailResolutionPreservesMatchesMissesAndAmbiguity(t *testing.T) {
	t.Parallel()
	accountID := int64(11)
	batchCalls := 0
	config := &StaffImportConfig{StaffImportDeps: ports.StaffImportDeps{
		Membership: staffEmailMembership{members: []ports.Staff{{ID: 21, PersonID: 31}, {ID: 22, PersonID: 32}, {ID: 23, PersonID: 33}}},
		Persons: PersonDirectoryFuncs{ListPersonsByAccountFunc: func(_ context.Context, ids []int64) ([]ports.Person, error) {
			require.ElementsMatch(t, []int64{11, 12}, ids)
			return []ports.Person{{ID: 31, AccountID: &accountID}, {ID: 34, AccountID: &accountID}}, nil
		}},
		FindSchoolEmailIdentities: func(_ context.Context, emails []string) (map[string]int64, map[string][]int64, error) {
			batchCalls++
			require.ElementsMatch(t, []string{"account@example.test", "invited@example.test", "missing@example.test", "ambiguous@example.test", "priority@example.test"}, emails)
			return map[string]int64{"account@example.test": 11, "priority@example.test": 12}, map[string][]int64{
				"account@example.test": {32}, "invited@example.test": {32, 32}, "ambiguous@example.test": {32, 33}, "priority@example.test": {32},
			}, nil
		},
		FindSchoolAccount: func(context.Context, string) (int64, bool, error) {
			t.Fatal("cached matches and misses must not fall back to scalar reads")
			return 0, false, nil
		},
	}}
	rows := []importModels.StaffImportRow{
		{Email: " Account@Example.Test "}, {Email: "invited@example.test"}, {Email: "missing@example.test"},
		{Email: "ambiguous@example.test"}, {Email: "priority@example.test"},
	}
	ctx := context.WithValue(context.Background(), importModeKey{}, importModels.ImportModeUpsert)
	prepared, err := config.BatchWriteConfig(ctx, rows)
	require.NoError(t, err)
	for repeat := 0; repeat < 2; repeat++ {
		for i, want := range []int64{21, 22, 0, 0, 0} {
			id, err := prepared.FindExisting(ctx, rows[i])
			if i == 3 {
				require.ErrorContains(t, err, "mehrere offene Einladungen")
				continue
			}
			require.NoError(t, err)
			if want == 0 {
				require.Nil(t, id)
			} else {
				require.NotNil(t, id)
				require.Equal(t, want, *id)
			}
		}
	}
	require.Equal(t, 1, batchCalls)
}

func TestStaffBatchEmailMissChangesOnlyAfterSuccessfulInvitation(t *testing.T) {
	t.Parallel()
	invitationFailure := errors.New("invitation failed")
	invitationErr := invitationFailure
	config := &StaffImportConfig{StaffImportDeps: ports.StaffImportDeps{
		Membership: staffEmailMembership{}, Persons: newFakePersons(),
		Transactions: guardianPhoneFactTransactions{Transactions: writeMatchTransactions{}, tenantID: 42},
		Invitations:  &staffInvitationMock{InviteStaffFn: func(context.Context, importModels.StaffInvitation) error { return invitationErr }},
	}, emailMatches: map[string]staffEmailMatch{"new@example.test": {}}}
	row := importModels.StaffImportRow{Email: "New@Example.Test", FirstName: "New", LastName: "Staff"}
	_, err := config.Create(context.Background(), row)
	require.ErrorIs(t, err, invitationFailure)
	id, err := config.matchedEmail(context.Background(), row.Email)
	require.NoError(t, err)
	require.Nil(t, id, "failed rows leave the cached miss intact")
	invitationErr = nil
	created, err := config.Create(context.Background(), row)
	require.NoError(t, err)
	id, err = config.FindExisting(context.Background(), importModels.StaffImportRow{Email: " new@example.test "})
	require.NoError(t, err)
	require.NotNil(t, id)
	require.Equal(t, created, *id, "later rows resolve the new email identity without another lookup")
}
