package behavior_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// #6294: a guardian invitation belongs to the address it was mailed to. A
// link issued before the profile address changed creates no account, and its
// preview shows nothing of the profile; a resend or an approval issues a new
// link for the address on file.

func accountCountByEmail(t *testing.T, db bun.IDB, emails ...string) int {
	t.Helper()
	lowered := make([]string, 0, len(emails))
	for _, email := range emails {
		lowered = append(lowered, strings.ToLower(strings.TrimSpace(email)))
	}
	count, err := db.NewSelect().TableExpr("auth.accounts").
		Where("LOWER(email) IN (?)", bun.List(lowered)).
		Count(context.Background())
	require.NoError(t, err)
	return int(count)
}

// changeGuardianEmail rewrites the profile address through the People
// Directory repository, the writer behind both contact edit paths.
func (env *guardianTestEnv) changeGuardianEmail(t *testing.T, profileID int64, email string) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	profile, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.GuardianProfile.FindByID, profileID)
	require.NoError(t, err)
	profile.Email = &email
	profile.FirstName = "Korrigiert"
	profile.LastName = "Neuname"
	require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.GuardianProfile.Update, profile))
}

func mailedToken(t *testing.T, outbox *api.CapturingOutbox, recipient string) string {
	t.Helper()
	requests := outbox.Requests()
	for i := len(requests) - 1; i >= 0; i-- {
		url, hasLink := requests[i].Payload["invitation_url"]
		if requests[i].Payload["recipient_email"] != recipient || !hasLink || requests[i].Payload["existing_account"] == true {
			continue
		}
		// The latest invitation mail to the recipient decides; a link without
		// a token is a broken mail, not a reason to fall back to an older one.
		match := mailedLinkTokenPattern.FindStringSubmatch(fmt.Sprint(url))
		require.Len(t, match, 2, "the latest invitation mail to %s carries no token: %v", recipient, url)
		return match[1]
	}
	t.Fatalf("no invitation link mailed to %s", recipient)
	return ""
}

func TestGuardianInvitationAccept_OldLinkCreatesNoAccountAfterEmailChange(t *testing.T) {
	t.Parallel()
	env := setupGuardianInvitationTest(t)
	profile := testpkg.CreateTestGuardianProfile(t, env.db, "address-binding-old")
	creatorID := env.inviterAccountID(t)
	ctx := testpkg.Ctx(t)
	invitation, err := env.service.CreateGuardianInvitation(ctx, identityaccess.GuardianInvitationRequest{
		GuardianProfileID: profile.ID, CreatedBy: creatorID,
	})
	require.NoError(t, err)
	oldEmail := *profile.Email
	newEmail := fmt.Sprintf("corrected-%d@example.test", time.Now().UnixNano())
	env.changeGuardianEmail(t, profile.ID, newEmail)

	_, err = env.service.AcceptGuardianInvitation(context.Background(), invitation.Token, identityaccess.GuardianRegistration{
		Password: strongTestPassword, ConfirmPassword: strongTestPassword,
	})
	require.ErrorIs(t, err, identityaccess.ErrGuardianInvitationAddressChanged)
	assert.Zero(t, accountCountByEmail(t, env.db, oldEmail, newEmail), "the old link must create no account")
	stored := testpkg.GuardianInvitationByID(t, env.db, invitation.ID)
	assert.Nil(t, stored.AcceptedAt, "the refused link stays unspent")
	reread, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.GuardianProfile.FindByID, profile.ID)
	require.NoError(t, err)
	assert.Nil(t, reread.AccountID, "the profile stays without an account")
}

func TestGuardianInvitationValidate_OldLinkShowsNothingAfterEmailChange(t *testing.T) {
	t.Parallel()
	env := setupGuardianInvitationTest(t)
	profile := testpkg.CreateTestGuardianProfile(t, env.db, "address-binding-preview")
	creatorID := env.inviterAccountID(t)
	invitation, err := env.service.CreateGuardianInvitation(testpkg.Ctx(t), identityaccess.GuardianInvitationRequest{
		GuardianProfileID: profile.ID, CreatedBy: creatorID,
	})
	require.NoError(t, err)
	newEmail := fmt.Sprintf("corrected-preview-%d@example.test", time.Now().UnixNano())
	env.changeGuardianEmail(t, profile.ID, newEmail)

	preview, err := env.service.ValidateGuardianInvitation(context.Background(), invitation.Token)
	require.ErrorIs(t, err, identityaccess.ErrGuardianInvitationAddressChanged)
	assert.Equal(t, identityaccess.GuardianInvitationPreview{}, preview, "the preview reveals no profile data")
}

func TestGuardianInvitationAccept_KeepsLinkWhenOnlyTheCaseChanges(t *testing.T) {
	t.Parallel()
	env := setupGuardianInvitationTest(t)
	profile := testpkg.CreateTestGuardianProfile(t, env.db, "address-binding-case")
	creatorID := env.inviterAccountID(t)
	invitation, err := env.service.CreateGuardianInvitation(testpkg.Ctx(t), identityaccess.GuardianInvitationRequest{
		GuardianProfileID: profile.ID, CreatedBy: creatorID,
	})
	require.NoError(t, err)
	// Written past the People Directory, which would normalize the address
	// itself: legacy data or another writer may keep case and blanks, and
	// only the comparison may treat them as the same address.
	spelled := " \t" + strings.ToUpper(*profile.Email) + " "
	_, err = env.db.NewRaw(`UPDATE users.guardian_profiles SET email = ? WHERE id = ?`, spelled, profile.ID).Exec(context.Background())
	require.NoError(t, err)
	var stored string
	require.NoError(t, env.db.NewRaw(`SELECT email FROM users.guardian_profiles WHERE id = ?`, profile.ID).Scan(context.Background(), &stored))
	require.Equal(t, spelled, stored, "the profile keeps the other spelling")

	account, err := env.service.AcceptGuardianInvitation(context.Background(), invitation.Token, identityaccess.GuardianRegistration{
		Password: strongTestPassword, ConfirmPassword: strongTestPassword,
	})
	require.NoError(t, err, "the same address in other letters is the same address")
	testpkg.OwnTestAccount(t, env.db, account.ID)
	assert.Equal(t, strings.ToLower(*profile.Email), account.Email)
}

func TestGuardianInvitationResend_IssuesLinkForTheCorrectedAddress(t *testing.T) {
	t.Parallel()
	outbox := api.NewCapturingOutbox()
	env := setupGuardianInvitationTest(t, func(_ *bun.DB, cfg *api.GuardianInvitationTestConfig) { cfg.Outbox = outbox })
	profile := testpkg.CreateTestGuardianProfile(t, env.db, "address-binding-resend")
	creatorID := env.inviterAccountID(t)
	ctx := testpkg.Ctx(t)
	invitation, err := env.service.CreateGuardianInvitation(ctx, identityaccess.GuardianInvitationRequest{
		GuardianProfileID: profile.ID, CreatedBy: creatorID,
	})
	require.NoError(t, err)
	oldEmail := *profile.Email
	newEmail := fmt.Sprintf("corrected-resend-%d@example.test", time.Now().UnixNano())
	env.changeGuardianEmail(t, profile.ID, newEmail)

	require.NoError(t, env.service.ResendGuardianInvitation(ctx, invitation.ID, creatorID))
	token := mailedToken(t, outbox, newEmail)
	account, err := env.service.AcceptGuardianInvitation(context.Background(), token, identityaccess.GuardianRegistration{
		Password: strongTestPassword, ConfirmPassword: strongTestPassword,
	})
	require.NoError(t, err, "the resent link belongs to the corrected address")
	testpkg.OwnTestAccount(t, env.db, account.ID)
	assert.Equal(t, newEmail, account.Email)
	assert.Zero(t, accountCountByEmail(t, env.db, oldEmail))
}

func TestGuardianInvitationApproval_IssuesLinkForTheCorrectedAddress(t *testing.T) {
	t.Parallel()
	outbox := api.NewCapturingOutbox()
	env := setupGuardianInvitationTest(t, func(_ *bun.DB, cfg *api.GuardianInvitationTestConfig) { cfg.Outbox = outbox })
	ctx := testpkg.Ctx(t)
	creatorID := env.inviterAccountID(t)
	student := testpkg.CreateTestStudent(t, env.db, "Approval", "Address", "2b")
	oldEmail := fmt.Sprintf("pending-typo-%d@example.test", time.Now().UnixNano())
	pending, err := env.service.InviteToStudent(ctx, identityaccess.InviteToStudentRequest{
		StudentID: student.ID, Email: oldEmail, CreatedBy: creatorID,
		RequestedByParentAccountID: &creatorID, RequireApproval: true,
	})
	require.NoError(t, err)
	require.NotNil(t, pending.InvitationID)
	newEmail := fmt.Sprintf("pending-corrected-%d@example.test", time.Now().UnixNano())
	env.changeGuardianEmail(t, pending.GuardianProfileID, newEmail)

	require.NoError(t, env.service.ApproveInvitation(ctx, *pending.InvitationID, creatorID))
	token := mailedToken(t, outbox, newEmail)
	account, err := env.service.AcceptGuardianInvitation(context.Background(), token, identityaccess.GuardianRegistration{
		Password: strongTestPassword, ConfirmPassword: strongTestPassword,
	})
	require.NoError(t, err, "the released link belongs to the address on file")
	testpkg.OwnTestAccount(t, env.db, account.ID)
	assert.Equal(t, newEmail, account.Email)
}

// A sibling invitation supersedes the first link. Repeating the original
// child request must still mail a usable new link, never an empty token.
func TestInviteToStudent_RepeatedInviteMailsRedeemableLink(t *testing.T) {
	t.Parallel()
	outbox := api.NewCapturingOutbox()
	env := setupGuardianInvitationTest(t, func(_ *bun.DB, cfg *api.GuardianInvitationTestConfig) { cfg.Outbox = outbox })
	ctx := testpkg.Ctx(t)
	creatorID := env.inviterAccountID(t)
	student := testpkg.CreateTestStudent(t, env.db, "Repeat", "Link", "3c")
	sibling := testpkg.CreateTestStudent(t, env.db, "Repeat", "Sibling", "3d")
	email := fmt.Sprintf("repeat-link-%d@example.test", time.Now().UnixNano())
	request := identityaccess.InviteToStudentRequest{StudentID: student.ID, Email: email, CreatedBy: creatorID}

	first, err := env.service.InviteToStudent(ctx, request)
	require.NoError(t, err)
	siblingRequest := request
	siblingRequest.StudentID = sibling.ID
	_, err = env.service.InviteToStudent(ctx, siblingRequest)
	require.NoError(t, err)
	second, err := env.service.InviteToStudent(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, second.InvitationID)
	assert.NotEqual(t, *first.InvitationID, *second.InvitationID, "the sibling invitation superseded the first link; the repeated request issues a new one")

	token := mailedToken(t, outbox, email)
	preview, err := env.service.ValidateGuardianInvitation(context.Background(), token)
	require.NoError(t, err, "the repeated mail carries a usable link")
	assert.Equal(t, email, preview.Email)
}

// Every link of a profile ends with its address: the per-child invitations
// of a guardian invited to two children both stop working.
func TestInviteToStudent_EmailChangeEndsTheLinksOfEveryChild(t *testing.T) {
	t.Parallel()
	outbox := api.NewCapturingOutbox()
	env := setupGuardianInvitationTest(t, func(_ *bun.DB, cfg *api.GuardianInvitationTestConfig) { cfg.Outbox = outbox })
	ctx := testpkg.Ctx(t)
	creatorID := env.inviterAccountID(t)
	email := fmt.Sprintf("two-children-%d@example.test", time.Now().UnixNano())
	var tokens []string
	var profileID int64
	for _, class := range []string{"5a", "5b"} {
		student := testpkg.CreateTestStudent(t, env.db, "Two", "Children", class)
		result, err := env.service.InviteToStudent(ctx, identityaccess.InviteToStudentRequest{
			StudentID: student.ID, Email: email, CreatedBy: creatorID,
		})
		require.NoError(t, err)
		profileID = result.GuardianProfileID
		tokens = append(tokens, mailedToken(t, outbox, email))
	}
	env.changeGuardianEmail(t, profileID, fmt.Sprintf("two-children-corrected-%d@example.test", time.Now().UnixNano()))

	for i, token := range tokens {
		_, err := env.service.AcceptGuardianInvitation(context.Background(), token, identityaccess.GuardianRegistration{
			Password: strongTestPassword, ConfirmPassword: strongTestPassword,
		})
		if i == 0 {
			require.ErrorIs(t, err, identityaccess.ErrInvitationExpired, "the sibling invitation already ended the first link")
		} else {
			require.ErrorIs(t, err, identityaccess.ErrGuardianInvitationAddressChanged)
		}
	}
	assert.Zero(t, accountCountByEmail(t, env.db, email))
}

// Accept and an address change serialize on the profile row. The change
// holds the row first; the acceptance must wait for it and then refuse the
// link, instead of checking the old address and linking the account after
// the change committed.
func TestGuardianInvitationAccept_SerializesWithConcurrentEmailChange(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, err := api.NewGuardianInvitationServiceForTests(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.GuardianInvitationTestConfig{
		Outbox: api.NewCapturingOutbox(),
	})
	require.NoError(t, err)
	_, creator := testpkg.CreateTestPersonWithAccount(t, db, "Inviter", "Race")
	profile := testpkg.CreateTestGuardianProfile(t, db, "address-binding-race")
	invitation, err := service.CreateGuardianInvitation(testpkg.Ctx(t), identityaccess.GuardianInvitationRequest{
		GuardianProfileID: profile.ID, CreatedBy: creator.ID,
	})
	require.NoError(t, err)
	oldEmail := *profile.Email
	newEmail := fmt.Sprintf("race-corrected-%d@example.test", time.Now().UnixNano())

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	change, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = change.Rollback() }()
	_, err = change.NewRaw(`UPDATE users.guardian_profiles SET email = ? WHERE id = ?`, newEmail, profile.ID).Exec(ctx)
	require.NoError(t, err)

	results := make(chan error, 1)
	go func() {
		account, acceptErr := service.AcceptGuardianInvitation(ctx, invitation.Token, identityaccess.GuardianRegistration{
			Password: strongTestPassword, ConfirmPassword: strongTestPassword,
		})
		if acceptErr == nil {
			testpkg.OwnTestAccount(t, db, account.ID)
		}
		results <- acceptErr
	}()
	require.Eventually(t, func() bool {
		var waiting int
		if scanErr := db.NewRaw(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock' AND query ILIKE '%guardian_profiles%'`).
			Scan(ctx, &waiting); scanErr != nil {
			return false
		}
		return waiting > 0
	}, 10*time.Second, 10*time.Millisecond, "the acceptance must wait for the uncommitted address change")
	require.NoError(t, change.Commit())

	select {
	case acceptErr := <-results:
		require.ErrorIs(t, acceptErr, identityaccess.ErrGuardianInvitationAddressChanged)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	assert.Zero(t, accountCountByEmail(t, db, oldEmail, newEmail))
	assert.Nil(t, testpkg.GuardianInvitationByID(t, db, invitation.ID).AcceptedAt)
}

// Account-linking flows hold the account before locking the profile. While
// acceptance waits for that account, it must leave the profile available so
// the linking transaction can finish instead of forming a deadlock cycle.
func TestGuardianInvitationAccept_LocksAccountBeforeProfile(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, err := api.NewGuardianInvitationServiceForTests(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.GuardianInvitationTestConfig{
		Outbox: api.NewCapturingOutbox(),
	})
	require.NoError(t, err)
	_, creator := testpkg.CreateTestPersonWithAccount(t, db, "Inviter", "LockOrder")
	profile := testpkg.CreateTestGuardianProfile(t, db, "account-before-profile")
	account := testpkg.CreateTestAccountWithPassword(t, db, *profile.Email, strongTestPassword)
	invitation, err := service.CreateGuardianInvitation(testpkg.Ctx(t), identityaccess.GuardianInvitationRequest{
		GuardianProfileID: profile.ID, CreatedBy: creator.ID,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	linking, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = linking.Rollback() }()
	_, err = linking.NewRaw(`SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE`, account.ID).Exec(ctx)
	require.NoError(t, err)

	type acceptance struct {
		account identityaccess.Account
		err     error
	}
	results := make(chan acceptance, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		accepted, acceptErr := service.AcceptGuardianInvitation(ctx, invitation.Token, identityaccess.GuardianRegistration{
			Password: strongTestPassword, ConfirmPassword: strongTestPassword,
		})
		results <- acceptance{account: accepted, err: acceptErr}
	}()
	defer func() {
		cancel()
		_ = linking.Rollback()
		<-finished
	}()
	require.Eventually(t, func() bool {
		var waiting int
		if scanErr := db.NewRaw(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock' AND query ILIKE '%auth.accounts%'`).
			Scan(ctx, &waiting); scanErr != nil {
			return false
		}
		return waiting > 0
	}, 10*time.Second, 10*time.Millisecond, "acceptance must wait for the account held by the linking transaction")
	_, err = linking.NewRaw(`SELECT id FROM users.guardian_profiles WHERE id = ? FOR UPDATE NOWAIT`, profile.ID).Exec(ctx)
	require.NoError(t, err, "acceptance must not hold the profile while waiting for the account")
	require.NoError(t, linking.Commit())

	select {
	case result := <-results:
		require.NoError(t, result.err)
		assert.Equal(t, account.ID, result.account.ID)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	assert.NotNil(t, testpkg.GuardianInvitationByID(t, db, invitation.ID).AcceptedAt)
}

// Accept holds the account before it locks the profile. Creating an
// invitation must take the same order: holding the profile while it waits
// for the account would close a cycle with an acceptance and end one of
// them with a deadlock (40P01). The other transaction stands in for the
// acceptance: account first, then the profile.
func TestCreateGuardianInvitation_LocksAccountBeforeProfile(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, err := api.NewGuardianInvitationServiceForTests(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.GuardianInvitationTestConfig{
		Outbox: api.NewCapturingOutbox(),
	})
	require.NoError(t, err)
	_, creator := testpkg.CreateTestPersonWithAccount(t, db, "Inviter", "CreateOrder")
	profile := testpkg.CreateTestGuardianProfile(t, db, "create-account-before-profile")
	account := testpkg.CreateTestAccountWithPassword(t, db, *profile.Email, strongTestPassword)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	acceptance, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = acceptance.Rollback() }()
	_, err = acceptance.NewRaw(`SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE`, account.ID).Exec(ctx)
	require.NoError(t, err)

	created := make(chan error, 1)
	go func() {
		_, createErr := service.CreateGuardianInvitation(testpkg.TenantContext(testpkg.Tenant(t)), identityaccess.GuardianInvitationRequest{
			GuardianProfileID: profile.ID, CreatedBy: creator.ID,
		})
		created <- createErr
	}()
	require.Eventually(t, func() bool {
		var waiting int
		if scanErr := db.NewRaw(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock' AND query ILIKE '%auth.accounts%'`).
			Scan(ctx, &waiting); scanErr != nil {
			return false
		}
		return waiting > 0
	}, 10*time.Second, 10*time.Millisecond, "creating the invitation must wait for the account the acceptance holds")

	_, err = acceptance.NewRaw(`SELECT id FROM users.guardian_profiles WHERE id = ? FOR UPDATE`, profile.ID).Exec(ctx)
	require.NoError(t, err, "the acceptance must get the profile: creating may not hold it while it waits for the account")
	require.NoError(t, acceptance.Rollback())

	select {
	case createErr := <-created:
		require.NoError(t, createErr)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

// Locks follow one order: account, then school, then profile. Accept took the
// shared school lock before the account, so a transaction that holds the
// account and then locks the school (as a school-wide account flow does)
// closed a cycle with it and one of them ended with 40P01. The other
// transaction stands in for that flow.
func TestGuardianInvitationAccept_LocksAccountBeforeSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, err := api.NewGuardianInvitationServiceForTests(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.GuardianInvitationTestConfig{
		Outbox: api.NewCapturingOutbox(),
	})
	require.NoError(t, err)
	_, creator := testpkg.CreateTestPersonWithAccount(t, db, "Inviter", "SchoolOrder")
	profile := testpkg.CreateTestGuardianProfile(t, db, "account-before-school")
	account := testpkg.CreateTestAccountWithPassword(t, db, *profile.Email, strongTestPassword)
	invitation, err := service.CreateGuardianInvitation(testpkg.Ctx(t), identityaccess.GuardianInvitationRequest{
		GuardianProfileID: profile.ID, CreatedBy: creator.ID,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	other, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = other.Rollback() }()
	_, err = other.NewRaw(`SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE`, account.ID).Exec(ctx)
	require.NoError(t, err)

	results := make(chan error, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		_, acceptErr := service.AcceptGuardianInvitation(ctx, invitation.Token, identityaccess.GuardianRegistration{
			Password: strongTestPassword, ConfirmPassword: strongTestPassword,
		})
		results <- acceptErr
	}()
	defer func() {
		cancel()
		_ = other.Rollback()
		<-finished
	}()
	require.Eventually(t, func() bool {
		var waiting int
		if scanErr := db.NewRaw(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock' AND query ILIKE '%auth.accounts%'`).
			Scan(ctx, &waiting); scanErr != nil {
			return false
		}
		return waiting > 0
	}, 10*time.Second, 10*time.Millisecond, "the acceptance must wait for the account the other transaction holds")

	_, err = other.NewRaw(`SELECT id FROM platform.schools WHERE id = ? FOR UPDATE`, testpkg.Tenant(t)).Exec(ctx)
	require.NoError(t, err, "the other transaction must get the school: acceptance may not hold it while it waits for the account")
	require.NoError(t, other.Rollback())

	select {
	case acceptErr := <-results:
		require.NoError(t, acceptErr)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	assert.NotNil(t, testpkg.GuardianInvitationByID(t, db, invitation.ID).AcceptedAt)
}

// The school check runs after the account is provisioned (lock order). A
// deleted school still refuses the link, and the account it provisioned
// rolls back with the acceptance.
func TestGuardianInvitationAccept_RefusesDeletedSchoolWithoutAccount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, err := api.NewGuardianInvitationServiceForTests(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.GuardianInvitationTestConfig{
		Outbox: api.NewCapturingOutbox(),
	})
	require.NoError(t, err)
	_, creator := testpkg.CreateTestPersonWithAccount(t, db, "Inviter", "DeletedSchool")
	profile := testpkg.CreateTestGuardianProfile(t, db, "deleted-school-accept")
	invitation, err := service.CreateGuardianInvitation(testpkg.Ctx(t), identityaccess.GuardianInvitationRequest{
		GuardianProfileID: profile.ID, CreatedBy: creator.ID,
	})
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, testpkg.Tenant(t)).Exec(context.Background())
	require.NoError(t, err)

	_, err = service.AcceptGuardianInvitation(context.Background(), invitation.Token, identityaccess.GuardianRegistration{
		Password: strongTestPassword, ConfirmPassword: strongTestPassword,
	})
	require.ErrorIs(t, err, identityaccess.ErrInvitationTenantDeleted)
	assert.Zero(t, accountCountByEmail(t, db, *profile.Email), "the provisioned account rolls back")
	assert.Nil(t, testpkg.GuardianInvitationByID(t, db, invitation.ID).AcceptedAt)
}

func TestGuardianInvitationNewIssueEndsOlderLinks(t *testing.T) {
	t.Parallel()
	env := setupGuardianInvitationTest(t)
	profile := testpkg.CreateTestGuardianProfile(t, env.db, "superseded-link")
	creator := env.inviterAccountID(t)
	request := identityaccess.GuardianInvitationRequest{GuardianProfileID: profile.ID, CreatedBy: creator}
	first, err := env.service.CreateGuardianInvitation(testpkg.Ctx(t), request)
	require.NoError(t, err)
	second, err := env.service.CreateGuardianInvitation(testpkg.Ctx(t), request)
	require.NoError(t, err)
	_, err = env.service.ValidateGuardianInvitation(context.Background(), first.Token)
	require.Error(t, err, "only the latest invitation may be redeemed")
	_, err = env.service.ValidateGuardianInvitation(context.Background(), second.Token)
	require.NoError(t, err)
}
