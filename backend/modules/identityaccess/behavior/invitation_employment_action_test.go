package behavior_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Drive the persisted invitation through the real owner compositions: the
// choice belongs to the school and the person survives either choice.
func TestInvitationEmploymentChoice(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		action        identityaccess.EmploymentAction
		alreadyActive bool
		numberTaken   bool
		operator      bool
	}{
		{name: "legacy default"},
		{name: "resume", action: identityaccess.EmploymentActionResume},
		{name: "new", action: identityaccess.EmploymentActionNew},
		{name: "resume after activation", action: identityaccess.EmploymentActionResume, alreadyActive: true},
		{name: "new after activation", action: identityaccess.EmploymentActionNew, alreadyActive: true},
		{name: "resume with reassigned number", action: identityaccess.EmploymentActionResume, numberTaken: true},
		{name: "operator always resumes", action: identityaccess.EmploymentActionNew, operator: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			action := tc.action
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			ctx := testpkg.Ctx(t)
			env := newInvitationEnv(t, db)
			former, account := testpkg.CreateTestStaffWithAccount(t, db, "Returning", "Member")
			_, err := db.ExecContext(ctx, `UPDATE users.staff_employment_profiles SET personnel_number = 'RETURN-01', staff_notes = 'earlier notes' WHERE membership_id = ?`, former.ID)
			require.NoError(t, err)
			_, err = db.ExecContext(ctx, `UPDATE users.staff_school_memberships SET deleted_at = now() WHERE id = ?`, former.ID)
			require.NoError(t, err)
			_, err = db.ExecContext(ctx, `UPDATE auth.accounts SET active = FALSE WHERE id = ?`, account.ID)
			require.NoError(t, err)
			_, err = db.ExecContext(ctx, `UPDATE auth.account_tenants SET status = 'inactive', deactivated_at = now() WHERE account_id = ? AND tenant_id = ?`, account.ID, testpkg.Tenant(t))
			require.NoError(t, err)
			role := testpkg.CreateTestRole(t, db, "returning-member")
			creator := testpkg.CreateTestAccount(t, db, "return-inviter")
			invitation, err := env.service.CreateSchoolInvitation(ctx, identityaccess.SchoolInvitationRequest{
				Email: account.Email, RoleID: role.ID, CreatedBy: creator.ID,
				FirstName: testpkg.StrPtr("Returning"), LastName: testpkg.StrPtr("Member"),
				OperatorGrant: tc.operator, EmploymentAction: action, ActorPermissions: []string{usersManagePermission},
			})
			require.NoError(t, err)
			expected := action
			if expected == "" || tc.operator {
				expected = identityaccess.EmploymentActionResume
			}
			require.Equal(t, expected, invitation.EmploymentAction)
			pending, err := env.service.ListPendingSchoolInvitations(ctx)
			require.NoError(t, err)
			require.Len(t, pending, 1)
			require.Equal(t, expected, pending[0].EmploymentAction)
			// A resend must not forget the school's decision.
			require.NoError(t, env.service.ResendSchoolInvitation(ctx, invitation.ID, creator.ID))
			if tc.alreadyActive {
				_, err = db.ExecContext(ctx, `UPDATE users.staff_school_memberships SET deleted_at = NULL WHERE id = ?`, former.ID)
				require.NoError(t, err)
			}
			if tc.numberTaken {
				other := testpkg.CreateTestStaff(t, db, "Other", "Member")
				_, err = db.ExecContext(ctx, `UPDATE users.staff_employment_profiles SET personnel_number = 'RETURN-01' WHERE membership_id = ?`, other.ID)
				require.NoError(t, err)
			}
			accepted, err := env.service.AcceptSchoolInvitation(context.Background(), invitation.Token, identityaccess.InvitationRegistration{
				Password: invitationPassword, ConfirmPassword: invitationPassword,
			})
			require.NoError(t, err)
			require.Equal(t, account.ID, accepted.ID)
			var memberships []struct {
				ID        int64      `bun:"id"`
				PersonID  int64      `bun:"person_id"`
				DeletedAt *time.Time `bun:"deleted_at"`
			}
			require.NoError(t, db.NewRaw(`SELECT id, person_id, deleted_at FROM users.staff_school_memberships WHERE tenant_id = ? AND person_id = ? ORDER BY id`, testpkg.Tenant(t), former.PersonID).Scan(ctx, &memberships))
			if expected == identityaccess.EmploymentActionNew && !tc.alreadyActive {
				require.Len(t, memberships, 2)
				require.Equal(t, former.ID, memberships[0].ID)
				require.NotNil(t, memberships[0].DeletedAt, "the earlier employment stays ended")
				require.Nil(t, memberships[1].DeletedAt)
			} else {
				require.Len(t, memberships, 1)
				require.Equal(t, former.ID, memberships[0].ID, "the same employment continues")
				require.Nil(t, memberships[0].DeletedAt)
			}
			var profile struct {
				PersonnelNumber *string `bun:"personnel_number"`
				StaffNotes      string  `bun:"staff_notes"`
			}
			current := memberships[len(memberships)-1]
			require.Equal(t, former.PersonID, current.PersonID)
			require.NoError(t, db.NewRaw(`SELECT personnel_number, staff_notes FROM users.staff_employment_profiles WHERE membership_id = ?`, current.ID).Scan(ctx, &profile))
			if expected == identityaccess.EmploymentActionNew && !tc.alreadyActive {
				require.Nil(t, profile.PersonnelNumber)
				require.Empty(t, profile.StaffNotes, "a new employment starts with its own profile")
			} else {
				require.Equal(t, "earlier notes", profile.StaffNotes)
				if tc.numberTaken {
					require.Nil(t, profile.PersonnelNumber)
				} else {
					require.NotNil(t, profile.PersonnelNumber)
					require.Equal(t, "RETURN-01", *profile.PersonnelNumber)
				}
			}
			var persons int
			require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.persons WHERE tenant_id = ? AND account_id = ?`, testpkg.Tenant(t), account.ID).Scan(ctx, &persons))
			require.Equal(t, 1, persons, "both choices reuse the person")
		})
	}
}
