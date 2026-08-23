package users_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestLetterChildStatuses_DerivesFulfilmentFromAcknowledgement pins the core
// mechanism of #2384: nothing about per-child fulfilment is stored. A child
// counts as fulfilled the moment ANY guardian with portal access acknowledges
// the announcement, and the query reports who that was.
func TestLetterChildStatuses_DerivesFulfilmentFromAcknowledgement(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx(t)

	letter := publishedAnnouncement(t, ctx, repo, chain.AccountID, chain.TenantID,
		"Elternbrief", []*usersModels.ParentAnnouncementTarget{
			{TargetType: usersModels.AnnouncementTargetSchoolAll},
		})

	// Before anyone confirms, the child is reached but open.
	children, err := repo.LetterChildStatuses(ctx, chain.TenantID, letter.ID)
	require.NoError(t, err)
	found := childByID(children, chain.StudentID)
	require.NotNil(t, found, "the linked child must appear in the letter audience")
	assert.False(t, found.Fulfilled(), "no acknowledgement yet, so the child is open")

	// A plain READ must not fulfil the letter — only an explicit confirmation does.
	readApplied, err := repo.MarkRead(ctx, chain.TenantID, letter.ID, chain.AccountID, *letter.PublishedAt)
	require.NoError(t, err)
	require.True(t, readApplied)

	children, err = repo.LetterChildStatuses(ctx, chain.TenantID, letter.ID)
	require.NoError(t, err)
	found = childByID(children, chain.StudentID)
	require.NotNil(t, found)
	assert.False(t, found.Fulfilled(), "reading is not confirming")

	// The acknowledgement fulfils it, and the query names the person.
	ackApplied, err := repo.MarkAcknowledged(ctx, chain.TenantID, letter.ID, chain.AccountID, *letter.PublishedAt)
	require.NoError(t, err)
	require.True(t, ackApplied)

	children, err = repo.LetterChildStatuses(ctx, chain.TenantID, letter.ID)
	require.NoError(t, err)
	found = childByID(children, chain.StudentID)
	require.NotNil(t, found)
	assert.True(t, found.Fulfilled(), "an acknowledgement fulfils the letter for the child")
	require.NotNil(t, found.AcknowledgedAt)
	assert.WithinDuration(t, time.Now(), *found.AcknowledgedAt, time.Minute)
	assert.NotEmpty(t, found.AckLastName, "the fulfilling guardian must be named")
}

// TestLetterChildStatuses_TenantIsolation is the acceptance criterion "Tenant-
// Isolation serverseitig getestet": another tenant's id must never surface a
// foreign school's children, even for a real announcement id.
func TestLetterChildStatuses_TenantIsolation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx(t)

	letter := publishedAnnouncement(t, ctx, repo, chain.AccountID, chain.TenantID,
		"Elternbrief", []*usersModels.ParentAnnouncementTarget{
			{TargetType: usersModels.AnnouncementTargetSchoolAll},
		})

	own, err := repo.LetterChildStatuses(ctx, chain.TenantID, letter.ID)
	require.NoError(t, err)
	require.NotEmpty(t, own, "precondition: the letter reaches at least one child")

	foreign, err := repo.LetterChildStatuses(ctx, chain.TenantID+9999, letter.ID)
	require.NoError(t, err)
	assert.Empty(t, foreign, "a foreign tenant must resolve no children for this letter")

	recipients, err := repo.ResolveDeliveryRecipients(ctx, chain.TenantID+9999, letter.ID)
	require.NoError(t, err)
	assert.Empty(t, recipients, "a foreign tenant must resolve no recipients for this letter")
}

// TestResolveDeliveryRecipients_IncludesGuardiansWithoutPortalAccess is the
// difference that makes the recipient matrix useful: unlike ResolveAudienceEmails
// this resolution keeps the people who get nothing, so the school can see and
// repair the gap instead of wondering why a family never heard back.
func TestResolveDeliveryRecipients_IncludesGuardiansWithoutPortalAccess(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx(t)

	letter := publishedAnnouncement(t, ctx, repo, chain.AccountID, chain.TenantID,
		"Elternbrief", []*usersModels.ParentAnnouncementTarget{
			{TargetType: usersModels.AnnouncementTargetSchoolAll},
		})

	before, err := repo.ResolveDeliveryRecipients(ctx, chain.TenantID, letter.ID)
	require.NoError(t, err)
	require.NotEmpty(t, before, "precondition: the linked guardian resolves")
	linked := recipientByProfile(before, chain.GuardianProfileID)
	require.NotNil(t, linked)
	assert.True(t, linked.HasPortalAccess, "the fixture guardian has portal access")

	// Revoke portal access on the link. The classic audience query drops the
	// person entirely; this one must keep them, flagged.
	_, err = db.NewUpdate().
		Table("users.students_guardians").
		Set(`permissions = permissions - 'parent_portal.access'`).
		Where("student_id = ? AND guardian_profile_id = ?", chain.StudentID, chain.GuardianProfileID).
		Exec(ctx)
	require.NoError(t, err)

	after, err := repo.ResolveDeliveryRecipients(ctx, chain.TenantID, letter.ID)
	require.NoError(t, err)
	stillThere := recipientByProfile(after, chain.GuardianProfileID)
	require.NotNil(t, stillThere, "a guardian without portal access must stay visible in the matrix")
	assert.False(t, stillThere.HasPortalAccess, "and must be flagged as unreachable in moto")

	// The classic e-mail audience, by contrast, drops them — that asymmetry is
	// the whole reason this second resolution exists.
	emails, err := repo.ResolveAudienceEmails(ctx, chain.TenantID, letter.ID)
	require.NoError(t, err)
	for _, e := range emails {
		assert.NotEqual(t, chain.Email, e.Email,
			"a guardian without portal access must not be in the classic e-mail audience")
	}
}

func childByID(children []*usersModels.AnnouncementLetterChildStatus, studentID int64) *usersModels.AnnouncementLetterChildStatus {
	for _, c := range children {
		if c.StudentID == studentID {
			return c
		}
	}
	return nil
}

func recipientByProfile(recipients []*usersModels.AnnouncementDeliveryRecipient, profileID int64) *usersModels.AnnouncementDeliveryRecipient {
	for _, r := range recipients {
		if r.GuardianProfileID == profileID {
			return r
		}
	}
	return nil
}

// TestLetterChildStatuses_KeepsChildrenNobodyCanConfirmFor is the regression the
// visual check surfaced: a school-wide letter reaches every child, but only the
// ones with a portal-capable guardian can ever be confirmed. Dropping the rest
// made a letter to 116 children report "6 Kinder" — which a school reads as
// "almost everyone confirmed" rather than "110 children have no portal contact".
func TestLetterChildStatuses_KeepsChildrenNobodyCanConfirmFor(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	repo := usersRepo.NewParentAnnouncementRepository(db)
	ctx := tenantCtx(t)

	letter := publishedAnnouncement(t, ctx, repo, chain.AccountID, chain.TenantID,
		"Elternbrief", []*usersModels.ParentAnnouncementTarget{
			{TargetType: usersModels.AnnouncementTargetSchoolAll},
		})

	before, err := repo.LetterChildStatuses(ctx, chain.TenantID, letter.ID)
	require.NoError(t, err)
	linked := childByID(before, chain.StudentID)
	require.NotNil(t, linked, "precondition: the linked child is reached")
	assert.True(t, linked.CanConfirm, "a guardian with portal access can confirm")
	assert.True(t, linked.Outstanding(), "and the child counts as outstanding")

	// Strip portal access. The child is STILL reached by a school-wide letter and
	// must stay in the list — only now nobody can confirm for it.
	_, err = db.NewUpdate().
		Table("users.students_guardians").
		Set(`permissions = permissions - 'parent_portal.access'`).
		Where("student_id = ?", chain.StudentID).
		Exec(ctx)
	require.NoError(t, err)

	after, err := repo.LetterChildStatuses(ctx, chain.TenantID, letter.ID)
	require.NoError(t, err)
	still := childByID(after, chain.StudentID)
	require.NotNil(t, still, "a child without a portal-capable guardian must stay visible")
	assert.False(t, still.CanConfirm, "and must be marked as unconfirmable")
	assert.False(t, still.Outstanding(), "reminding cannot help, so it is not outstanding")
	assert.False(t, still.Fulfilled())

	// The count must not shrink: losing a guardian permission changes WHO can
	// confirm, never who the letter reached.
	assert.GreaterOrEqual(t, len(after), len(before),
		"revoking portal access must not remove children from the letter's reach")
}
