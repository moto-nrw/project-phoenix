package care_test

// Erklärungen the school no longer shows (#6290): withdrawn (unpublished),
// deactivated or expired. Withdrawing a consent must stay as easy as giving
// it (Art. 7 Abs. 3 DSGVO), so a guardian keeps the revocation of their own
// consent, and only that, until they use it.

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unpublishDeclaration is the staff "Zurückziehen": published_at goes NULL.
func (s *declarationSetup) unpublishDeclaration(t *testing.T) {
	t.Helper()
	require.NoError(t, testpkg.TenantCommand2(t, s.seedCtx, testpkg.SetupTestDB(t), s.repo.SetPublished, s.announcement.ID, nil))
}

// updateAnnouncementRow changes the stored announcement row the way staff
// edits or a future deactivation would.
func (s *declarationSetup) updateAnnouncementRow(t *testing.T, set string, args ...any) {
	t.Helper()
	_, err := s.db.NewUpdate().TableExpr("users.parent_announcements").
		Set(set, args...).
		Where("id = ?", s.announcement.ID).
		Exec(testpkg.WithPackageTenantRuntime(context.Background()))
	require.NoError(t, err)
}

// feedItem returns the account's feed entry for the declaration, or nil.
func (s *declarationSetup) feedItem(t *testing.T, accountID int64) *parentportal.AnnouncementFeedItem {
	t.Helper()
	feed, err := s.svc.ListAnnouncements(s.ctx, accountID)
	require.NoError(t, err)
	for _, item := range feed {
		if item.ID == s.announcement.ID {
			return item
		}
	}
	return nil
}

// requireEndedWithRevocation checks that the guardian still finds the ended
// declaration with the wording they answered and only the revocation.
func (s *declarationSetup) requireEndedWithRevocation(t *testing.T) {
	t.Helper()
	item := s.feedItem(t, s.chain.AccountID)
	require.NotNil(t, item, "the guardian must find the declaration to revoke the consent")
	require.NotNil(t, item.Declaration)
	assert.True(t, item.Ended)
	assert.True(t, item.Declaration.Ended)
	assert.True(t, item.Declaration.Closed)
	assert.Equal(t, s.version.Title, item.Title)
	assert.Equal(t, s.version.Body, item.Body)
	require.Len(t, item.Declaration.Children, 1)
	assert.Equal(t, []string{parentportal.DeclarationActionRevoked}, item.Declaration.Children[0].AllowedActions)
	assert.Equal(t, communication.DeclarationStateAgreed, item.Declaration.Children[0].State)
}

func TestDeclaration_WithdrawnDeclarationKeepsOwnConsentRevocable(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(communication.DeclarationSignersAny, true, false), nil)
	_, created, err := s.submit(s.chain.AccountID, parentportal.DeclarationActionAgreed)
	require.NoError(t, err)
	require.True(t, created)
	live := s.feedItem(t, s.chain.AccountID)
	require.NotNil(t, live)
	assert.False(t, live.Ended)
	assert.False(t, live.Declaration.Ended)

	s.unpublishDeclaration(t)
	// Staff edit the withdrawn declaration before a correction: the guardian
	// keeps seeing the wording they answered, not the draft.
	s.updateAnnouncementRow(t, "title = ?, body = ?, link_url = ?", "Entwurf", "Noch nicht freigegeben", "https://example.org/entwurf")
	s.requireEndedWithRevocation(t)
	assert.Nil(t, s.feedItem(t, s.chain.AccountID).LinkURL)

	_, _, err = s.submit(s.chain.AccountID, parentportal.DeclarationActionDeclined)
	require.ErrorIs(t, err, parentportal.ErrAnnouncementNotFound, "a withdrawn declaration takes no new answers")
	revoked, created, err := s.submit(s.chain.AccountID, parentportal.DeclarationActionRevoked)
	require.NoError(t, err, "a consent stays revocable after the school withdrew the declaration")
	assert.True(t, created)
	assert.Equal(t, parentportal.DeclarationActionRevoked, revoked.Action)

	history := s.history(t)
	require.Len(t, history, 2, "the consent and its revocation both stay on record")
	assert.Equal(t, parentportal.DeclarationActionRevoked, history[0].Action)
	assert.Equal(t, parentportal.DeclarationActionAgreed, history[1].Action)

	_, created, err = s.submit(s.chain.AccountID, parentportal.DeclarationActionRevoked)
	require.NoError(t, err)
	assert.False(t, created, "a repeated revocation yields the same proof")
	_, _, err = s.submit(s.chain.AccountID, parentportal.DeclarationActionAgreed)
	require.ErrorIs(t, err, parentportal.ErrAnnouncementNotFound, "consenting again needs a published declaration")
	assert.Len(t, s.history(t), 2)

	item := s.feedItem(t, s.chain.AccountID)
	require.NotNil(t, item, "the revoked consent stays findable with its proof")
	assert.Empty(t, item.Declaration.Children[0].AllowedActions)
	assert.Equal(t, communication.DeclarationStateRevoked, item.Declaration.Children[0].State)
	count, err := s.svc.UnreadAnnouncementCount(s.ctx, s.chain.AccountID)
	require.NoError(t, err)
	assert.Zero(t, count, "an ended declaration owes nothing")
}

func TestDeclaration_ExpiredDeclarationStaysFindableForRevocation(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(communication.DeclarationSignersAny, true, false), nil)
	_, _, err := s.submit(s.chain.AccountID, parentportal.DeclarationActionAgreed)
	require.NoError(t, err)

	s.updateAnnouncementRow(t, "created_at = created_at - interval '2 hours', expires_at = created_at - interval '1 hour'")

	s.requireEndedWithRevocation(t)
}

func TestDeclaration_DeactivatedDeclarationKeepsOwnConsentRevocable(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(communication.DeclarationSignersAny, true, false), nil)
	_, _, err := s.submit(s.chain.AccountID, parentportal.DeclarationActionAgreed)
	require.NoError(t, err)

	s.updateAnnouncementRow(t, "active = false")
	s.requireEndedWithRevocation(t)

	_, _, err = s.submit(s.chain.AccountID, parentportal.DeclarationActionDeclined)
	require.ErrorIs(t, err, parentportal.ErrAnnouncementNotFound)
	_, created, err := s.submit(s.chain.AccountID, parentportal.DeclarationActionRevoked)
	require.NoError(t, err, "a consent stays revocable after the declaration was deactivated")
	assert.True(t, created)
}

func TestDeclaration_WithdrawnDeclarationRefusesNewAnswers(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(communication.DeclarationSignersAny, true, false), nil)
	co := testpkg.CreateTestCoGuardianForStudent(t, s.db, s.chain.StudentID, "Anna", "Schneider")
	_, _, err := s.submit(s.chain.AccountID, parentportal.DeclarationActionAgreed)
	require.NoError(t, err)

	s.unpublishDeclaration(t)

	for _, action := range []string{parentportal.DeclarationActionAgreed, parentportal.DeclarationActionDeclined} {
		_, _, err = s.submit(co.AccountID, action)
		require.ErrorIs(t, err, parentportal.ErrAnnouncementNotFound, action)
	}
	_, _, err = s.submit(co.AccountID, parentportal.DeclarationActionRevoked)
	require.ErrorIs(t, err, parentportal.ErrAnnouncementNotFound,
		"a guardian without an own answer learns nothing about a withdrawn declaration")
	assert.Nil(t, s.feedItem(t, co.AccountID), "the withdrawn declaration leaves the feed of those who did not answer")
	assert.Len(t, s.history(t), 1)
}

func TestDeclaration_WithdrawnDeclarationWithoutAnswersStaysHidden(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(communication.DeclarationSignersAny, true, false), nil)

	s.unpublishDeclaration(t)

	_, _, err := s.submit(s.chain.AccountID, parentportal.DeclarationActionRevoked)
	require.ErrorIs(t, err, parentportal.ErrAnnouncementNotFound)
	assert.Nil(t, s.feedItem(t, s.chain.AccountID))
	assert.Empty(t, s.history(t))
}

func TestDeclaration_WithdrawnDeclarationRevocationNeedsPortalAccess(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(communication.DeclarationSignersAny, true, false), nil)
	_, _, err := s.submit(s.chain.AccountID, parentportal.DeclarationActionAgreed)
	require.NoError(t, err)

	s.unpublishDeclaration(t)
	_, err = s.db.NewUpdate().TableExpr("auth.guardian_student_access").
		Set("permissions = permissions - 'parent_portal.access'").
		Where("account_id = ?", s.chain.AccountID).
		Exec(testpkg.WithPackageTenantRuntime(context.Background()))
	require.NoError(t, err)

	_, _, err = s.submit(s.chain.AccountID, parentportal.DeclarationActionRevoked)
	require.ErrorIs(t, err, parentportal.ErrAnnouncementNotFound, "without portal access the child is not the guardian's to answer for")
	assert.Nil(t, s.feedItem(t, s.chain.AccountID))
	assert.Len(t, s.history(t), 1)
}

func TestDeclaration_EndedDeclarationShowsOnlyAnsweredChildren(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(communication.DeclarationSignersAny, true, false), nil)
	sibling := testpkg.CreateTestStudent(t, s.db, "Mia", "Schneider", "2b")
	link := &testpkg.StudentGuardian{
		StudentID: sibling.ID, GuardianProfileID: s.chain.GuardianProfileID, RelationshipType: "parent",
	}
	testpkg.ApplyStudentGuardianRole(link, securityruntime.GuardianRolePrimaryGuardian)
	link.TenantID = s.chain.TenantID
	require.NoError(t, testpkg.InsertTestStudentGuardian(context.Background(), s.db, link))
	// Targets change only on a draft: retract, widen to the sibling, publish again.
	s.unpublishDeclaration(t)
	require.NoError(t, testpkg.TenantCommand3(t, s.seedCtx, testpkg.SetupTestDB(t), s.repo.ReplaceTargets, s.chain.TenantID, s.announcement.ID,
		[]*api.ParentAnnouncementTargetFixture{
			{TargetType: "student", TargetRefID: &s.chain.StudentID},
			{TargetType: "student", TargetRefID: &sibling.ID},
		}))
	published := time.Now().Add(-time.Minute)
	require.NoError(t, testpkg.TenantCommand2(t, s.seedCtx, testpkg.SetupTestDB(t), s.repo.SetPublished, s.announcement.ID, &published))
	require.Len(t, s.feedDeclaration(t, s.chain.AccountID).Children, 2)
	_, _, err := s.submit(s.chain.AccountID, parentportal.DeclarationActionAgreed)
	require.NoError(t, err)

	s.unpublishDeclaration(t)

	s.requireEndedWithRevocation(t)
	assert.Equal(t, s.chain.StudentID, s.feedItem(t, s.chain.AccountID).Declaration.Children[0].StudentID,
		"an ended declaration asks nothing for the child nobody answered for")
}
