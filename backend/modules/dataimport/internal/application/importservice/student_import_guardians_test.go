package importpkg

import (
	"context"
	"errors"
	"testing"

	importModels "github.com/moto-ogs/project-phoenix/modules/dataimport"
	"github.com/moto-ogs/project-phoenix/modules/dataimport/ports"
	"github.com/stretchr/testify/require"
)

func TestGuardianPhoneCreateFactRequiresFreshTransactionalProfile(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		created  *ports.Guardian
		active   bool
		tenantID int64
		fresh    bool
	}{
		{"new profile", &ports.Guardian{ID: 41, TenantID: 23}, true, 23, true},
		{"no create fact", nil, true, 23, false},
		{"positive IDs without transaction", &ports.Guardian{ID: 41, TenantID: 23}, false, 23, false},
		{"reused profile", &ports.Guardian{ID: 41, TenantID: 23, ReusedExisting: true}, true, 23, false},
		{"wrong guardian", &ports.Guardian{ID: 42, TenantID: 23}, true, 23, false},
		{"missing guardian ID", &ports.Guardian{TenantID: 23}, true, 23, false},
		{"negative guardian ID", &ports.Guardian{ID: -41, TenantID: 23}, true, 23, false},
		{"wrong tenant", &ports.Guardian{ID: 41, TenantID: 24}, true, 23, false},
		{"missing tenant", &ports.Guardian{ID: 41}, true, 0, false},
		{"negative tenant", &ports.Guardian{ID: 41, TenantID: -23}, true, -23, false},
		{"phones already written", &ports.Guardian{ID: 41, TenantID: 23, PhoneNumbers: []ports.GuardianPhone{{PhoneNumber: "old"}}}, true, 23, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reads := 0
			var added []ports.GuardianPhoneInput
			config := NewStudentImportConfig(ports.StudentImportDeps{
				Transactions: guardianPhoneFactTransactions{active: tc.active, tenantID: tc.tenantID},
				Guardians: GuardianDirectoryFuncs{
					ListGuardianPhonesFunc: func(_ context.Context, id int64) ([]ports.GuardianPhone, error) {
						require.EqualValues(t, 41, id)
						reads++
						return []ports.GuardianPhone{{PhoneNumber: "0151 123456"}}, nil
					},
					AddGuardianPhoneFunc: func(_ context.Context, id int64, input ports.GuardianPhoneInput) (ports.GuardianPhone, error) {
						require.EqualValues(t, 41, id)
						added = append(added, input)
						return ports.GuardianPhone{}, nil
					},
				},
			})
			require.NoError(t, config.createGuardianPhoneNumbers(context.Background(), 41, []importModels.PhoneImportData{
				{},
				{PhoneNumber: "0151 123456", PhoneType: "mobile"},
				{PhoneNumber: "0221 654321", PhoneType: "home", Label: "Zuhause", IsPrimary: true},
				{PhoneNumber: "0221 654321"},
			}, tc.created))
			if tc.fresh {
				require.Zero(t, reads)
				require.Len(t, added, 2)
				require.Same(t, tc.created, added[0].CreatedGuardian)
				require.Equal(t, "0151 123456", added[0].PhoneNumber)
			} else {
				require.Equal(t, 1, reads)
				require.Len(t, added, 1)
			}
			last := added[len(added)-1]
			require.Nil(t, last.CreatedGuardian, "the create fact is consumed by the first write only")
			require.Equal(t, "0221 654321", last.PhoneNumber)
			require.Equal(t, "home", last.PhoneType)
			require.Equal(t, "Zuhause", *last.Label)
			require.True(t, last.IsPrimary)
		})
	}
}

func TestGuardianPhoneImportPreservesLookupAndWriteFailures(t *testing.T) {
	t.Parallel()
	for _, fresh := range []bool{false, true} {
		t.Run(map[bool]string{false: "lookup before write", true: "new profile write failure"}[fresh], func(t *testing.T) {
			t.Parallel()
			failure := errors.New("directory unavailable")
			writes := 0
			config := NewStudentImportConfig(ports.StudentImportDeps{
				Transactions: guardianPhoneFactTransactions{active: true, tenantID: 23},
				Guardians: GuardianDirectoryFuncs{
					ListGuardianPhonesFunc: func(context.Context, int64) ([]ports.GuardianPhone, error) { return nil, failure },
					AddGuardianPhoneFunc: func(context.Context, int64, ports.GuardianPhoneInput) (ports.GuardianPhone, error) {
						writes++
						return ports.GuardianPhone{}, failure
					},
				},
			})
			var created *ports.Guardian
			if fresh {
				created = &ports.Guardian{ID: 41, TenantID: 23}
			}
			err := config.createGuardianPhoneNumbers(context.Background(), 41, []importModels.PhoneImportData{{PhoneNumber: "invalid"}, {PhoneNumber: "0151 123456"}}, created)
			require.ErrorIs(t, err, failure)
			if fresh {
				require.EqualError(t, err, "phone 1: directory unavailable")
				require.Equal(t, 1, writes)
			} else {
				require.EqualError(t, err, "Telefonnummern laden: directory unavailable")
				require.Zero(t, writes)
			}
		})
	}
}

type guardianPhoneFactTransactions struct {
	importModels.Transactions
	active   bool
	tenantID int64
}

func (t guardianPhoneFactTransactions) HasTransaction(context.Context) bool { return t.active }
func (t guardianPhoneFactTransactions) TenantID(context.Context) int64      { return t.tenantID }
