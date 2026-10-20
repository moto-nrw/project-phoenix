package behavior_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestInvitationEmploymentResets(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                                                                     string
		master, qualifications, work, active, numberTaken, failAudit, oldOpening, futureVacation bool
	}{
		{name: "unchanged"}, {name: "master data", master: true}, {name: "qualifications", qualifications: true},
		{name: "work time after closed month", work: true}, {name: "all", master: true, qualifications: true, work: true},
		{name: "already active", master: true, qualifications: true, work: true, active: true},
		{name: "reassigned number audited", master: true, numberTaken: true},
		{name: "rollback and retry", master: true, qualifications: true, work: true, failAudit: true},
		{name: "previous vacation opening stays in audit", work: true, oldOpening: true},
		{name: "future vacation spends the new entitlement", work: true, futureVacation: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			var db *bun.DB
			if tc.failAudit {
				db = testpkg.SetupIsolatedTestDB(t)
			} else {
				db = testpkg.SetupTestDB(t)
			}
			ctx := testpkg.Ctx(t)
			now := calendar.NewDate(2026, time.October, 1).BerlinMidnight().Add(10 * time.Hour)
			module, err := api.NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.WithInvitationResetClock(func() time.Time { return now }), api.WithAuthTestMailer(testpkg.NewCapturingMailer()))
			require.NoError(t, err)
			former, account := testpkg.CreateTestStaffWithAccount(t, db, "Return", "Reset")
			seedReturningEmployment(t, db, former.ID)
			if tc.futureVacation {
				_, err := db.ExecContext(ctx, `INSERT INTO active.staff_absences (tenant_id, staff_id, absence_type, date_start, date_end, status, created_by, working_days) VALUES (?, ?, 'vacation', '2026-11-02', '2026-11-05', 'approved', ?, 4)`, testpkg.Tenant(t), former.ID, former.ID)
				require.NoError(t, err)
			}
			if tc.oldOpening {
				_, err := db.ExecContext(ctx, `INSERT INTO active.staff_vacation_openings (tenant_id, staff_id, year, effective_date, taken_before_days, entered_remaining_days, note, decided_by) VALUES (?, ?, 2026, '2026-02-01', 3, 31, 'Frühere Übernahme', ?)`, testpkg.Tenant(t), former.ID, former.ID)
				require.NoError(t, err)
			}
			_, err = db.ExecContext(ctx, `UPDATE users.staff_school_memberships SET deleted_at = now() WHERE id = ?`, former.ID)
			require.NoError(t, err)
			_, err = db.ExecContext(ctx, `UPDATE auth.accounts SET active = FALSE WHERE id = ?`, account.ID)
			require.NoError(t, err)
			_, err = db.ExecContext(ctx, `UPDATE auth.account_tenants SET status = 'inactive', deactivated_at = now() WHERE account_id = ? AND tenant_id = ?`, account.ID, testpkg.Tenant(t))
			require.NoError(t, err)
			role := testpkg.CreateTestRole(t, db, "return-reset")
			creator := testpkg.CreateTestAccount(t, db, "reset-inviter")
			invitation, err := module.Invitation.CreateSchoolInvitation(ctx, identityaccess.SchoolInvitationRequest{
				Email: account.Email, RoleID: role.ID, CreatedBy: creator.ID, ActorPermissions: []string{usersManagePermission},
				FirstName: testpkg.StrPtr("Return"), LastName: testpkg.StrPtr("Reset"),
				EmploymentAction: identityaccess.EmploymentActionResume, ResetMasterData: tc.master, ResetQualifications: tc.qualifications, ResetWorkTime: tc.work,
			})
			require.NoError(t, err)
			pending, err := module.Invitation.ListPendingSchoolInvitations(ctx)
			require.NoError(t, err)
			require.Len(t, pending, 1)
			require.Equal(t, tc.master, pending[0].ResetMasterData)
			require.Equal(t, tc.qualifications, pending[0].ResetQualifications)
			require.Equal(t, tc.work, pending[0].ResetWorkTime)
			require.NoError(t, module.Invitation.ResendSchoolInvitation(ctx, invitation.ID, creator.ID))
			if tc.active {
				_, err = db.ExecContext(ctx, `UPDATE users.staff_school_memberships SET deleted_at = NULL WHERE id = ?`, former.ID)
				require.NoError(t, err)
			}
			if tc.numberTaken {
				other := testpkg.CreateTestStaff(t, db, "Other", "Number")
				_, err = db.ExecContext(ctx, `UPDATE users.staff_employment_profiles SET personnel_number = 'RESET-01' WHERE membership_id = ?`, other.ID)
				require.NoError(t, err)
			}
			if tc.failAudit {
				_, err = db.ExecContext(ctx, `CREATE FUNCTION public.refuse_invitation_reset() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected reset audit failure'; END $$`)
				require.NoError(t, err)
				_, err = db.ExecContext(ctx, `CREATE TRIGGER refuse_invitation_reset BEFORE INSERT ON audit.staff_master_data_changes FOR EACH ROW EXECUTE FUNCTION public.refuse_invitation_reset()`)
				require.NoError(t, err)
				_, err = module.Invitation.AcceptSchoolInvitation(context.Background(), invitation.Token, identityaccess.InvitationRegistration{Password: invitationPassword, ConfirmPassword: invitationPassword})
				require.ErrorContains(t, err, "injected reset audit failure")
				assertInvitationResetRolledBack(t, db, former.ID, account.ID, invitation.ID)
				_, err = db.ExecContext(ctx, `DROP TRIGGER refuse_invitation_reset ON audit.staff_master_data_changes`)
				require.NoError(t, err)
			}
			_, err = module.Invitation.AcceptSchoolInvitation(context.Background(), invitation.Token, identityaccess.InvitationRegistration{Password: invitationPassword, ConfirmPassword: invitationPassword})
			require.NoError(t, err)
			resetMaster, resetQualifications, resetWork := tc.master && !tc.active, tc.qualifications && !tc.active, tc.work && !tc.active
			var profile struct {
				PersonnelNumber *string
				WorkTimeModelID *int64
			}
			require.NoError(t, db.NewRaw(`SELECT personnel_number, work_time_model_id FROM users.staff_employment_profiles WHERE membership_id = ?`, former.ID).Scan(ctx, &profile))
			require.Equal(t, resetMaster, profile.PersonnelNumber == nil)
			require.Equal(t, resetWork, profile.WorkTimeModelID == nil)
			var city *string
			require.NoError(t, db.NewRaw(`SELECT address_city FROM users.staff_master_data WHERE staff_id = ?`, former.ID).Scan(ctx, &city))
			if resetMaster {
				require.Nil(t, city)
			} else {
				require.Equal(t, "Köln", *city)
			}
			var retired bool
			require.NoError(t, db.NewRaw(`SELECT deleted_at IS NOT NULL FROM users.staff_qualifications WHERE staff_id = ?`, former.ID).Scan(ctx, &retired))
			require.Equal(t, resetQualifications, retired)
			var resets int
			require.NoError(t, db.NewRaw(`SELECT count(*) FROM active.staff_balance_adjustments WHERE staff_id = ? AND type = 'reset' AND effective_date = '2026-10-01' AND minutes_delta = -300`, former.ID).Scan(ctx, &resets))
			if resetWork {
				require.Equal(t, 1, resets)
			} else {
				require.Zero(t, resets)
			}
			var carryover float64
			require.NoError(t, db.NewRaw(`SELECT carryover_days FROM active.staff_vacation_quota WHERE staff_id = ? AND year = 2026`, former.ID).Scan(ctx, &carryover))
			if resetWork {
				require.Zero(t, carryover)
			} else {
				require.Equal(t, 4.0, carryover)
			}
			var oldBalance int
			require.NoError(t, db.NewRaw(`SELECT closing_balance_minutes FROM active.staff_month_balance_snapshots WHERE staff_id = ?`, former.ID).Scan(ctx, &oldBalance))
			require.Equal(t, 300, oldBalance, "closed history must remain unchanged")
			if resetMaster {
				var audit int
				require.NoError(t, db.NewRaw(`SELECT count(*) FROM audit.personnel_number_changes WHERE staff_id = ? AND old_value = 'RESET-01' AND new_value = ''`, former.ID).Scan(ctx, &audit))
				require.Equal(t, 1, audit, "also preserves a number cleared by conflict handling")
				require.NoError(t, db.NewRaw(`SELECT count(*) FROM audit.staff_master_data_changes WHERE staff_id = ? AND field_name = 'address_city' AND old_value = 'Köln'`, former.ID).Scan(ctx, &audit))
				require.Equal(t, 1, audit)
			}
			if resetWork {
				var opening struct {
					TakenBeforeDays      float64
					EnteredRemainingDays float64
				}
				require.NoError(t, db.NewRaw(`SELECT taken_before_days, entered_remaining_days FROM active.staff_vacation_openings WHERE staff_id = ?`, former.ID).Scan(ctx, &opening))
				require.Equal(t, -5.0, opening.TakenBeforeDays)
				require.Equal(t, 30.0, opening.EnteredRemainingDays)
				var absences int
				require.NoError(t, db.NewRaw(`SELECT count(*) FROM active.staff_absences WHERE staff_id = ?`, former.ID).Scan(ctx, &absences))
				expectedAbsences := 1
				if tc.futureVacation {
					expectedAbsences++
				}
				require.Equal(t, expectedAbsences, absences, "vacations remain evidence")
				var modelAudit int
				require.NoError(t, db.NewRaw(`SELECT count(*) FROM audit.staff_master_data_changes WHERE staff_id = ? AND field_name IN ('work_time_model_id', 'rotation_anchor_date') AND old_value <> '' AND new_value = ''`, former.ID).Scan(ctx, &modelAudit))
				require.Equal(t, 2, modelAudit)
				if tc.oldOpening {
					var tombstones int
					require.NoError(t, db.NewRaw(`SELECT count(*) FROM audit.time_tracking_deletions WHERE staff_id = ? AND source = 'vacation_opening'`, former.ID).Scan(ctx, &tombstones))
					require.Equal(t, 1, tombstones, "the earlier opening must remain as evidence")
				}
			}
		})
	}
}

func seedReturningEmployment(t *testing.T, db *bun.DB, staffID int64) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	var modelID int64
	require.NoError(t, db.NewRaw(`INSERT INTO config.work_time_models (tenant_id, name, rotation_anchor_date) VALUES (?, 'Returning model', '2026-01-01') RETURNING id`, tenantID).Scan(ctx, &modelID))
	_, err := db.ExecContext(ctx, `UPDATE users.staff_employment_profiles SET personnel_number = 'RESET-01', work_time_model_id = ?, rotation_anchor_date = '2026-01-01' WHERE membership_id = ?`, modelID, staffID)
	require.NoError(t, err)
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO users.staff_master_data (tenant_id, staff_id, address_city, weekly_hours) VALUES (?, ?, 'Köln', 20)`, []any{tenantID, staffID}},
		{`INSERT INTO users.staff_qualifications (tenant_id, staff_id, name, sort_order) VALUES (?, ?, 'Erste Hilfe', 0)`, []any{tenantID, staffID}},
		{`INSERT INTO active.staff_vacation_quota (tenant_id, staff_id, year, entitled_days, carryover_days) VALUES (?, ?, 2026, 30, 4)`, []any{tenantID, staffID}},
		{`INSERT INTO active.staff_absences (tenant_id, staff_id, absence_type, date_start, date_end, status, created_by, working_days) VALUES (?, ?, 'vacation', '2026-08-03', '2026-08-07', 'approved', ?, 5)`, []any{tenantID, staffID, staffID}},
		{`INSERT INTO active.staff_month_balance_snapshots (tenant_id, staff_id, year, month, closing_balance_minutes, closed_by) VALUES (?, ?, 2026, 9, 300, ?)`, []any{tenantID, staffID, staffID}},
	} {
		_, err := db.ExecContext(ctx, statement.query, statement.args...)
		require.NoError(t, err)
	}
}

func assertInvitationResetRolledBack(t *testing.T, db *bun.DB, staffID, accountID, invitationID int64) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	var retired, used, active bool
	require.NoError(t, db.NewRaw(`SELECT deleted_at IS NOT NULL FROM users.staff_school_memberships WHERE id = ?`, staffID).Scan(ctx, &retired))
	require.True(t, retired)
	require.NoError(t, db.NewRaw(`SELECT used_at IS NOT NULL FROM auth.invitation_tokens WHERE id = ?`, invitationID).Scan(ctx, &used))
	require.False(t, used, "failure must leave the invitation usable")
	require.NoError(t, db.NewRaw(`SELECT active FROM auth.accounts WHERE id = ?`, accountID).Scan(ctx, &active))
	require.False(t, active)
	var resets int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM active.staff_balance_adjustments WHERE staff_id = ? AND type = 'reset'`, staffID).Scan(ctx, &resets))
	require.Zero(t, resets, "the reset booking rolls back with the failed audit")
	var carryover float64
	require.NoError(t, db.NewRaw(`SELECT carryover_days FROM active.staff_vacation_quota WHERE staff_id = ? AND year = 2026`, staffID).Scan(ctx, &carryover))
	require.Equal(t, 4.0, carryover, "the vacation reset rolls back too")
}
