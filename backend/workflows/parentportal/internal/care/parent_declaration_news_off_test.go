package care_test

// A given consent stays revocable when the school switches parent news off
// (#6303, Art. 7 Abs. 3 DSGVO). The exception is narrow: the guardian finds
// and revokes only their own consent for their own child; everything else of
// that school stays hidden, and nobody can answer anew.

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// switchParentNewsOff rebuilds the service on the same database with the
// school's parent news turned off.
func (s *declarationSetup) switchParentNewsOff(t *testing.T) {
	t.Helper()
	s.svc, _, _ = buildAnnouncementServiceOn(t, s.db, false)
}

func feedIDs(t *testing.T, s *declarationSetup, accountID int64) []int64 {
	t.Helper()
	feed, err := s.svc.ListAnnouncements(s.ctx, accountID)
	require.NoError(t, err)
	ids := make([]int64, 0, len(feed))
	for _, item := range feed {
		ids = append(ids, item.ID)
	}
	return ids
}

func TestDeclaration_ConsentStaysRevocableWithParentNewsOff(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(communication.DeclarationSignersAny, false), nil)
	_, _, err := s.submit(s.chain.AccountID, parentportal.DeclarationActionAgreed)
	require.NoError(t, err)

	s.switchParentNewsOff(t)

	s.requireEndedWithRevocation(t)
	for _, action := range []string{parentportal.DeclarationActionAgreed, parentportal.DeclarationActionDeclined} {
		_, _, err = s.submit(s.chain.AccountID, action)
		require.ErrorIs(t, err, parentportal.ErrAnnouncementNotFound, "%s needs parent news switched on", action)
	}
	revoked, created, err := s.submit(s.chain.AccountID, parentportal.DeclarationActionRevoked)
	require.NoError(t, err, "a consent stays revocable when the school switched parent news off")
	assert.True(t, created)
	assert.Equal(t, parentportal.DeclarationActionRevoked, revoked.Action)
	_, created, err = s.submit(s.chain.AccountID, parentportal.DeclarationActionRevoked)
	require.NoError(t, err)
	assert.False(t, created, "a repeated revocation yields the same proof")

	history := s.history(t)
	require.Len(t, history, 2, "the consent and its revocation both stay on record")
	assert.Equal(t, parentportal.DeclarationActionRevoked, history[0].Action)
	assert.Equal(t, parentportal.DeclarationActionAgreed, history[1].Action)
	assert.Nil(t, s.feedItem(t, s.chain.AccountID), "with parent news off only a given consent stays in the feed")
	count, err := s.svc.UnreadAnnouncementCount(s.ctx, s.chain.AccountID)
	require.NoError(t, err)
	assert.Zero(t, count)
}

func TestDeclaration_ParentNewsOffShowsOnlyOwnGivenConsents(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(communication.DeclarationSignersAny, false), nil)
	co := testpkg.CreateTestCoGuardianForStudent(t, s.db, s.chain.StudentID, "Anna", "Schneider")
	child := []*api.ParentAnnouncementTargetFixture{{TargetType: "student", TargetRefID: &s.chain.StudentID}}
	declined, declinedVersion := seedDeclaration(t, s.seedCtx, s.repo, s.chain.AccountID, s.chain.TenantID, child,
		consent(communication.DeclarationSignersAny, false), nil)
	unanswered, _ := seedDeclaration(t, s.seedCtx, s.repo, s.chain.AccountID, s.chain.TenantID, child,
		consent(communication.DeclarationSignersAny, false), nil)
	letter := seedPublishedAnnouncement(t, s.seedCtx, s.repo, s.chain.AccountID, s.chain.TenantID, false)
	_, _, err := s.submit(s.chain.AccountID, parentportal.DeclarationActionAgreed)
	require.NoError(t, err)
	_, _, err = s.svc.SubmitDeclaration(s.ctx, s.chain.AccountID, declined.ID, parentportal.DeclarationInput{
		StudentID: s.chain.StudentID, VersionID: declinedVersion.ID, Action: parentportal.DeclarationActionDeclined,
	}, nil)
	require.NoError(t, err)
	require.Subset(t, feedIDs(t, s, s.chain.AccountID), []int64{s.announcement.ID, declined.ID, unanswered.ID, letter.ID})

	s.switchParentNewsOff(t)

	assert.Equal(t, []int64{s.announcement.ID}, feedIDs(t, s, s.chain.AccountID),
		"with parent news off a guardian sees only their own given consents")
	assert.Empty(t, feedIDs(t, s, co.AccountID), "the other guardian gave no consent and sees nothing")
	for _, action := range []string{parentportal.DeclarationActionAgreed, parentportal.DeclarationActionDeclined, parentportal.DeclarationActionRevoked} {
		_, _, err = s.submit(co.AccountID, action)
		require.ErrorIs(t, err, parentportal.ErrAnnouncementNotFound, "%s by a guardian without an own consent", action)
	}
	_, _, err = s.svc.SubmitDeclaration(s.ctx, s.chain.AccountID, declined.ID, parentportal.DeclarationInput{
		StudentID: s.chain.StudentID, VersionID: declinedVersion.ID, Action: parentportal.DeclarationActionRevoked,
	}, nil)
	require.ErrorIs(t, err, parentportal.ErrAnnouncementNotFound, "a refusal is no consent to revoke")
	assert.Len(t, s.history(t), 1)
}

// TestDeclaration_EveryConsentIsRevocable: no school setting makes a
// consent final; the guardian withdraws it while live and after it ended.
func TestDeclaration_EveryConsentIsRevocable(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(communication.DeclarationSignersAny, false), nil)
	_, _, err := s.submit(s.chain.AccountID, parentportal.DeclarationActionAgreed)
	require.NoError(t, err)
	assert.Equal(t, []string{parentportal.DeclarationActionRevoked}, s.feedDeclaration(t, s.chain.AccountID).Children[0].AllowedActions)

	s.unpublishDeclaration(t)

	s.requireEndedWithRevocation(t)
	_, created, err := s.submit(s.chain.AccountID, parentportal.DeclarationActionRevoked)
	require.NoError(t, err)
	assert.True(t, created)
}
