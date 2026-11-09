package care_test

// A consent to an older wording stays revocable after the school changed the
// text (#6303, Art. 7 Abs. 3 DSGVO). The revocation is recorded against the
// wording the guardian agreed to, and the feed shows that wording. Agreeing
// and declining stay bound to the current, published wording.

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// publishSecondWording is the staff correction: withdraw, change the text,
// publish again. It returns the new current version.
func (s *declarationSetup) publishSecondWording(t *testing.T) *parentportal.DeclarationVersion {
	t.Helper()
	s.unpublishDeclaration(t)
	v2 := freezeTestVersion(t, s.seedCtx, s.repo, s.announcement, 2)
	now := time.Now().Add(-time.Minute)
	require.NoError(t, testpkg.TenantCommand2(t, s.seedCtx, testpkg.SetupTestDB(t), s.repo.SetPublished, s.announcement.ID, &now))
	return v2
}

func (s *declarationSetup) submitOn(accountID, versionID int64, action string) (*parentportal.DeclarationSubmission, bool, error) {
	return s.svc.SubmitDeclaration(s.ctx, accountID, s.announcement.ID, parentportal.DeclarationInput{
		StudentID: s.chain.StudentID, VersionID: versionID, Action: action,
	}, nil)
}

// actionsByVersion maps version id to the account's actions, newest first.
func (s *declarationSetup) actionsByVersion(t *testing.T, accountID int64) map[int64][]string {
	t.Helper()
	out := map[int64][]string{}
	for _, sub := range s.history(t) {
		if sub.AccountID != nil && *sub.AccountID == accountID {
			out[sub.VersionID] = append(out[sub.VersionID], sub.Action)
		}
	}
	return out
}

func TestDeclaration_ConsentToAnOlderWordingStaysRevocable(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(communication.DeclarationSignersAny, false), nil)
	_, _, err := s.submit(s.chain.AccountID, parentportal.DeclarationActionAgreed)
	require.NoError(t, err)

	v2 := s.publishSecondWording(t)

	item := s.feedItem(t, s.chain.AccountID)
	require.NotNil(t, item)
	assert.False(t, item.Ended)
	require.Len(t, item.Declaration.Children, 1)
	child := item.Declaration.Children[0]
	assert.Equal(t, v2.ID, item.Declaration.Version.ID, "the school asks for the new wording")
	assert.ElementsMatch(t, []string{parentportal.DeclarationActionAgreed, parentportal.DeclarationActionDeclined}, child.AllowedActions)
	require.NotNil(t, child.EarlierConsent, "the consent to the old wording is shown")
	assert.Equal(t, s.version.ID, child.EarlierConsent.Version.ID)
	assert.Equal(t, s.version.Body, child.EarlierConsent.Version.Body, "with the wording the guardian agreed to")
	assert.Equal(t, []string{parentportal.DeclarationActionRevoked}, child.EarlierConsent.AllowedActions)

	_, _, err = s.submitOn(s.chain.AccountID, s.version.ID, parentportal.DeclarationActionAgreed)
	require.ErrorIs(t, err, parentportal.ErrDeclarationVersionChanged, "agreeing takes only the current wording")
	revoked, created, err := s.submitOn(s.chain.AccountID, s.version.ID, parentportal.DeclarationActionRevoked)
	require.NoError(t, err, "a consent to an older wording stays revocable")
	assert.True(t, created)
	assert.Equal(t, s.version.ID, revoked.VersionID, "the revocation is bound to the wording agreed to")
	assert.Equal(t, s.version.ContentHash, revoked.ContentHash)
	_, created, err = s.submitOn(s.chain.AccountID, s.version.ID, parentportal.DeclarationActionRevoked)
	require.NoError(t, err)
	assert.False(t, created, "a repeated revocation yields the same proof")

	assert.Equal(t, map[int64][]string{s.version.ID: {parentportal.DeclarationActionRevoked, parentportal.DeclarationActionAgreed}},
		s.actionsByVersion(t, s.chain.AccountID))
	after := s.feedItem(t, s.chain.AccountID).Declaration.Children[0]
	assert.Nil(t, after.EarlierConsent, "nothing is left to revoke")
}

func TestDeclaration_ConsentToAnOlderWordingStaysRevocableWithParentNewsOff(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(communication.DeclarationSignersAny, false), nil)
	_, _, err := s.submit(s.chain.AccountID, parentportal.DeclarationActionAgreed)
	require.NoError(t, err)
	v2 := s.publishSecondWording(t)

	off := s.withParentNewsOff(t)

	item := off.feedItem(t, s.chain.AccountID)
	require.NotNil(t, item, "the guardian finds the consent to revoke it")
	assert.True(t, item.Ended)
	assert.Equal(t, s.version.ID, item.Declaration.Version.ID, "the feed shows the wording agreed to")
	assert.Equal(t, s.version.Body, item.Body)
	require.Len(t, item.Declaration.Children, 1)
	child := item.Declaration.Children[0]
	require.NotNil(t, child.MyAction)
	assert.Equal(t, parentportal.DeclarationActionAgreed, *child.MyAction)
	assert.Equal(t, []string{parentportal.DeclarationActionRevoked}, child.AllowedActions)
	assert.Nil(t, child.EarlierConsent)

	for _, action := range []string{parentportal.DeclarationActionAgreed, parentportal.DeclarationActionDeclined} {
		_, _, err = off.submitOn(s.chain.AccountID, v2.ID, action)
		require.ErrorIs(t, err, parentportal.ErrAnnouncementNotFound, "%s needs parent news switched on", action)
	}
	_, created, err := off.submitOn(s.chain.AccountID, s.version.ID, parentportal.DeclarationActionRevoked)
	require.NoError(t, err, "a consent to an older wording stays revocable with parent news off")
	assert.True(t, created)
	assert.Nil(t, off.feedItem(t, s.chain.AccountID), "with parent news off only a given consent stays in the feed")
}

// TestDeclaration_RevocationEndsEveryOwnConsentToTheDeclaration: a guardian
// who agreed to the first and the second wording revokes once. Both consents
// end, each with a revocation bound to its own wording.
func TestDeclaration_RevocationEndsEveryOwnConsentToTheDeclaration(t *testing.T) {
	t.Parallel()
	for name, target := range map[string]string{"through the current wording": "current", "through the older wording": "older"} {
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			s := newDeclarationSetup(t, consent(communication.DeclarationSignersAny, false), nil)
			_, _, err := s.submit(s.chain.AccountID, parentportal.DeclarationActionAgreed)
			require.NoError(t, err)
			v2 := s.publishSecondWording(t)
			_, _, err = s.submitOn(s.chain.AccountID, v2.ID, parentportal.DeclarationActionAgreed)
			require.NoError(t, err)

			child := s.feedItem(t, s.chain.AccountID).Declaration.Children[0]
			assert.Equal(t, []string{parentportal.DeclarationActionRevoked}, child.AllowedActions)
			require.NotNil(t, child.EarlierConsent, "the older consent still stands")

			versionID := v2.ID
			if target == "older" {
				versionID = s.version.ID
			}
			_, created, err := s.submitOn(s.chain.AccountID, versionID, parentportal.DeclarationActionRevoked)
			require.NoError(t, err)
			assert.True(t, created)

			assert.Equal(t, map[int64][]string{
				s.version.ID: {parentportal.DeclarationActionRevoked, parentportal.DeclarationActionAgreed},
				v2.ID:        {parentportal.DeclarationActionRevoked, parentportal.DeclarationActionAgreed},
			}, s.actionsByVersion(t, s.chain.AccountID), "one revocation ends both consents")
			after := s.feedItem(t, s.chain.AccountID).Declaration.Children[0]
			require.NotNil(t, after.MyAction)
			assert.Equal(t, parentportal.DeclarationActionRevoked, *after.MyAction)
			assert.Nil(t, after.EarlierConsent)
		})
	}
}
