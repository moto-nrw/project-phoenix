package contracttest_test

import (
	"context"
	"errors"
	"testing"

	people "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain/userscontract"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/require"
)

func TestAddGuardianPhone_CreatedProfileFact(t *testing.T) {
	t.Parallel()
	const guardianID, schoolID int64 = 21, 23
	ambient := tenant.WithTenantID(guardianAmbientTxContext(), schoolID)
	created := people.Guardian{ID: guardianID, TenantID: schoolID}
	reused, foreign, mismatched, zeroID, noTenant, withPhone := created, created, created, created, created, created
	reused.ReusedExisting = true
	foreign.TenantID++
	mismatched.ID++
	zeroID.ID = 0
	noTenant.TenantID = 0
	withPhone.PhoneNumbers = []people.GuardianPhone{{PhoneNumber: "0171 1234567"}}
	lockFailure, lookupFailure, countFailure := errors.New("lock failed"), errors.New("lookup failed"), errors.New("count failed")
	for _, tc := range []struct {
		name                         string
		ctx                          context.Context
		created                      *people.Guardian
		lockErr, lookupErr, countErr error
		invalid                      bool
		want                         []string
		err                          error
	}{
		{name: "fresh first phone", ctx: ambient, created: &created, want: []string{"lock", "guardian", "create"}},
		{name: "no fact", ctx: ambient, want: []string{"lock", "guardian", "count", "create"}},
		{name: "reused", ctx: ambient, created: &reused, want: []string{"lock", "guardian", "count", "create"}},
		{name: "foreign tenant", ctx: ambient, created: &foreign, want: []string{"lock", "guardian", "count", "create"}},
		{name: "other guardian", ctx: ambient, created: &mismatched, want: []string{"lock", "guardian", "count", "create"}},
		{name: "zero ID", ctx: ambient, created: &zeroID, want: []string{"lock", "guardian", "count", "create"}},
		{name: "zero tenant", ctx: ambient, created: &noTenant, want: []string{"lock", "guardian", "count", "create"}},
		{name: "missing scope", ctx: guardianAmbientTxContext(), created: &noTenant, want: []string{"lock", "guardian", "count", "create"}},
		{name: "no transaction", ctx: tenant.WithTenantID(context.Background(), schoolID), created: &created, want: []string{"lock", "guardian", "count", "create"}},
		{name: "has phones", ctx: ambient, created: &withPhone, want: []string{"lock", "guardian", "count", "create"}},
		{name: "lock before validation", ctx: ambient, created: &created, lockErr: lockFailure, invalid: true, want: []string{"lock"}, err: lockFailure},
		{name: "lookup before validation", ctx: ambient, created: &created, lookupErr: lookupFailure, invalid: true, want: []string{"lock", "guardian"}, err: lookupFailure},
		{name: "count before validation", ctx: ambient, countErr: countFailure, invalid: true, want: []string{"lock", "guardian", "count"}, err: countFailure},
		{name: "invalid first phone", ctx: ambient, created: &created, invalid: true, want: []string{"lock", "guardian"}, err: people.ErrInvalidGuardian},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls []string
			service := peopleCompose.NewGuardianService(peoplePorts.GuardianServiceDependencies{
				GuardianProfileRepo: &fakeProfileRepo{
					lockFn: func(_ context.Context, id int64) error {
						require.Equal(t, guardianID, id)
						calls = append(calls, "lock")
						return tc.lockErr
					},
					findByIDFn: func(_ context.Context, id int64) (*people.GuardianProfileSnapshot, error) {
						require.Equal(t, guardianID, id)
						calls = append(calls, "guardian")
						return &people.GuardianProfileSnapshot{}, tc.lookupErr
					},
				},
				GuardianPhoneNumberRepo: firstPhoneRepo{
					count: func(_ context.Context, id int64) (int, int, error) {
						require.Equal(t, guardianID, id)
						calls = append(calls, "count")
						return 2, 9, tc.countErr
					},
					create: func(_ context.Context, phone *people.GuardianProfilePhoneSnapshot) error {
						calls = append(calls, "create")
						require.Equal(t, guardianID, phone.GuardianProfileID)
						require.Equal(t, "mobile", phone.PhoneType)
						require.Equal(t, "0171 1234567", phone.PhoneNumber)
						if len(calls) == 3 {
							require.True(t, phone.IsPrimary)
							require.Equal(t, 1, phone.Priority)
						} else {
							require.False(t, phone.IsPrimary)
							require.Equal(t, 9, phone.Priority)
						}
						return nil
					},
				},
			}, nil)
			number := "0171 1234567"
			if tc.invalid {
				number = ""
			}
			phone, err := service.AddPhoneNumber(tc.ctx, guardianID, peoplePorts.PhoneNumberCreateRequest{
				CreatedGuardian: tc.created, PhoneNumber: number, PhoneType: "unknown",
			})
			if tc.err != nil {
				require.Error(t, err)
				if errors.Is(tc.err, people.ErrInvalidGuardian) {
					var validation *userscontract.ValidationError
					require.ErrorAs(t, err, &validation)
					require.EqualError(t, validation.Err, "Telefonnummer ist erforderlich")
				} else {
					require.ErrorIs(t, err, tc.err)
				}
				require.Nil(t, phone)
			} else {
				require.NoError(t, err)
				require.NotNil(t, phone)
			}
			require.Equal(t, tc.want, calls)
		})
	}
}

type firstPhoneRepo struct {
	peoplePorts.GuardianPhoneNumberRepository
	count  func(context.Context, int64) (int, int, error)
	create func(context.Context, *people.GuardianProfilePhoneSnapshot) error
}

func (r firstPhoneRepo) CountAndNextPriority(ctx context.Context, id int64) (int, int, error) {
	return r.count(ctx, id)
}

func (r firstPhoneRepo) Create(ctx context.Context, phone *people.GuardianProfilePhoneSnapshot) error {
	return r.create(ctx, phone)
}
