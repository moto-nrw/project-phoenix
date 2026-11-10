package enrollmenthttp_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Renewal terms (#6456): a renewal carries no acceptance of the earlier
// phase; the parents accept the required blocks of the new phase's form on
// the status page, and only that is evidence for the new contract.

// renewalTerms reads the terms the status page shows for a token.
func renewalTerms(t *testing.T, ctx context.Context, svc enrollmentAPI.RequestService, token string) *capability.RenewalTerms {
	t.Helper()
	var page capability.StatusPage
	require.NoError(t, fixturePublicTokenRoot(ctx, func(txCtx context.Context) error {
		req, children, err := svc.GetByStatusToken(txCtx, token)
		if err != nil {
			return err
		}
		page, err = svc.StatusPage(txCtx, req, children)
		return err
	}))
	return page.RenewalTerms
}

// acceptedRenewalTerms ticks every block the status page shows, for the
// form version it shows.
func acceptedRenewalTerms(t *testing.T, ctx context.Context, svc enrollmentAPI.RequestService, token string) capability.RenewalConfirmation {
	t.Helper()
	terms := renewalTerms(t, ctx, svc, token)
	require.NotNil(t, terms, "a renewal shows its terms")
	flags := map[string]any{}
	for _, block := range terms.Blocks {
		flags[block.Key] = true
	}
	return capability.RenewalConfirmation{ConsentFlags: testFormValues(flags), DisplayedFormSchemaID: terms.FormSchemaID}
}

// rolledRequest rolls the source phase over and returns the one carried
// request with its child.
func rolledRequest(t *testing.T, env *rolloverTestEnv, mode string) (*capability.Request, *capability.RequestChild) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, validRolloverRequest(env, mode, true))
	require.NoError(t, err)
	require.Equal(t, 1, result.RolledCount)
	children, err := testpkg.TenantQuery2(t, ctx, env.db, env.repos.Enrollment().ChildrenByPhaseStatuses, result.Phase.ID,
		[]string{capability.ChildStatusPendingRenewal, capability.ChildStatusAutoRenewed})
	require.NoError(t, err)
	require.Len(t, children, 1)
	request, err := enrollmentAPI.ReadOwnerRequestForTest(ctx, env.repos.Enrollment(), children[0].RequestID)
	require.NoError(t, err)
	return request, children[0]
}

func requireInputCode(t *testing.T, err error, code, field string) {
	t.Helper()
	var rejection *capability.InvalidInputError
	require.True(t, errors.As(err, &rejection), "want an input rejection, got %v", err)
	assert.Equal(t, code, rejection.Code)
	assert.Equal(t, field, rejection.Field)
}

// The source's evidence and consent flags stay with the source request: a
// copy would show the old acceptance as one for the new phase and would
// stamp the student's consents again at the approval.
func TestRolloverCarriesNoConsentEvidenceForward(t *testing.T) {
	t.Parallel()
	env := setupRolloverTest(t)
	ctx := testpkg.Ctx(t)
	source := seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Nachweis", "anna.nachweis@example.com", "Lina", "Nachweis", 1)
	sourceRequest, err := enrollmentAPI.ReadOwnerRequestForTest(ctx, env.repos.Enrollment(), source.RequestID)
	require.NoError(t, err)
	require.NotEmpty(t, sourceRequest.LegalBlocksSnapshot, "the source submission holds its evidence")
	require.False(t, sourceRequest.ConsentFlags.IsZero(), "the source submission holds its consent flags")

	request, _ := rolledRequest(t, env, capability.PhaseRolloverModeOptOut)
	assert.Empty(t, request.LegalBlocksSnapshot, "no evidence is copied into the new phase")
	assert.True(t, request.ConsentFlags.IsZero(), "no consent flag is copied into the new phase")

	terms := renewalTerms(t, ctx, env.requestSvc, request.StatusToken)
	require.NotNil(t, terms)
	assert.Nil(t, terms.ConfirmedAt, "silence accepts nothing")
	assert.NotEmpty(t, terms.Blocks)
	for _, block := range terms.Blocks {
		assert.True(t, block.Required, "the page asks for the required blocks only")
	}
	mails := env.outbox.ByKind(capability.MailKindRolloverOptOut)
	require.Len(t, mails, 1)
	assert.Equal(t, true, mails[0].Payload[capability.EnrollmentPayloadRenewalTermsRequired], "the mail asks for the confirmation")
}

func TestConfirmRenewalRefusesMissingTicksAndAnOutdatedForm(t *testing.T) {
	t.Parallel()
	env := setupRolloverTest(t)
	ctx := testpkg.Ctx(t)
	_ = seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Haken", "anna.haken@example.com", "Lina", "Haken", 1)
	request, child := rolledRequest(t, env, capability.PhaseRolloverModeOptIn)

	accepted := acceptedRenewalTerms(t, ctx, env.requestSvc, request.StatusToken)
	require.False(t, accepted.ConsentFlags.IsZero())
	for key := range accepted.ConsentFlags.All() {
		ticks := map[string]any{}
		for other := range accepted.ConsentFlags.All() {
			if other != key {
				ticks[other] = true
			}
		}
		missing := capability.RenewalConfirmation{ConsentFlags: testFormValues(ticks), DisplayedFormSchemaID: accepted.DisplayedFormSchemaID}
		_, err := env.requestSvc.ConfirmRenewal(ctx, request.StatusToken, missing)
		requireInputCode(t, err, capability.CodeRenewalTermsRequired, "consent_flags."+key)
	}

	// The school publishes a new form version while the page is open: the
	// ticks for the old version accept nothing.
	outdated := accepted
	outdated.DisplayedFormSchemaID = accepted.DisplayedFormSchemaID + 1
	_, err := env.requestSvc.ConfirmRenewal(ctx, request.StatusToken, outdated)
	requireInputCode(t, err, capability.CodeRenewalTermsChanged, "form_schema_id")

	stored, err := enrollmentAPI.ReadOwnerRequestForTest(ctx, env.repos.Enrollment(), request.ID)
	require.NoError(t, err)
	assert.Empty(t, stored.LegalBlocksSnapshot, "a refused confirmation records nothing")
	unchanged, err := enrollmentAPI.ReadOwnerChildForTest(ctx, env.repos.Enrollment(), child.ID)
	require.NoError(t, err)
	assert.Equal(t, capability.ChildStatusPendingRenewal, unchanged.Status)
}

func TestConfirmRenewalRecordsTheAcceptanceOfAnOptOutRenewal(t *testing.T) {
	t.Parallel()
	env := setupRolloverTest(t)
	ctx := testpkg.Ctx(t)
	_ = seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Weiter", "anna.weiter@example.com", "Lina", "Weiter", 1)
	request, child := rolledRequest(t, env, capability.PhaseRolloverModeOptOut)

	confirmation := acceptedRenewalTerms(t, ctx, env.requestSvc, request.StatusToken)
	confirmed, err := env.requestSvc.ConfirmRenewal(ctx, request.StatusToken, confirmation)
	require.NoError(t, err)
	assert.Zero(t, confirmed, "an opt-out renewal moves no child")

	kept, err := enrollmentAPI.ReadOwnerChildForTest(ctx, env.repos.Enrollment(), child.ID)
	require.NoError(t, err)
	assert.Equal(t, capability.ChildStatusAutoRenewed, kept.Status, "the care runs on as before")

	stored, err := enrollmentAPI.ReadOwnerRequestForTest(ctx, env.repos.Enrollment(), request.ID)
	require.NoError(t, err)
	require.Len(t, stored.LegalBlocksSnapshot, 1, "the acceptance is a new evidence entry")
	entry := stored.LegalBlocksSnapshot[0]
	assert.Equal(t, confirmation.DisplayedFormSchemaID, entry.FormSchemaID)
	assert.Equal(t, confirmation.DisplayedFormSchemaID, entry.DisplayedFormSchemaID)
	assert.False(t, entry.SnapshotAt.IsZero())
	for _, block := range entry.Blocks {
		assert.NotEmpty(t, block.TextSHA256, "block %s carries its checksum", block.Key)
	}
	for key := range confirmation.ConsentFlags.All() {
		assert.Equal(t, true, testFormValue(stored.ConsentFlags, key), "the request holds the accepted %s", key)
	}

	terms := renewalTerms(t, ctx, env.requestSvc, request.StatusToken)
	require.NotNil(t, terms)
	require.NotNil(t, terms.ConfirmedAt, "the page shows the acceptance")
	assert.True(t, terms.ConfirmedAt.Equal(entry.SnapshotAt))
}

// An acceptance after the school's decision proves nothing for the
// contract; the confirmation then records nothing.
func TestConfirmRenewalAfterTheDecisionRecordsNothing(t *testing.T) {
	t.Parallel()
	env := setupRolloverTest(t)
	ctx := testpkg.Ctx(t)
	_ = seedApprovedChild(t, env, env.sourcePhase.ID, "Anna", "Spaet", "anna.spaet@example.com", "Lina", "Spaet", 1)
	request, child := rolledRequest(t, env, capability.PhaseRolloverModeOptOut)
	confirmation := acceptedRenewalTerms(t, ctx, env.requestSvc, request.StatusToken)
	require.NoError(t, testpkg.TenantCommand4(t, ctx, env.db, env.repos.Enrollment().UpdateChildStatus, child.ID, capability.ChildStatusApproved, nil, env.creatorID))

	confirmed, err := env.requestSvc.ConfirmRenewal(ctx, request.StatusToken, confirmation)
	require.NoError(t, err)
	assert.Zero(t, confirmed)
	stored, err := enrollmentAPI.ReadOwnerRequestForTest(ctx, env.repos.Enrollment(), request.ID)
	require.NoError(t, err)
	assert.Empty(t, stored.LegalBlocksSnapshot)
	assert.Nil(t, renewalTerms(t, ctx, env.requestSvc, request.StatusToken), "no child waits for its renewal")
}
