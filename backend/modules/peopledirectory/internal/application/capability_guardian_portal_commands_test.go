package application

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGuardianPortalCommandsRefuseIncompleteInputBeforeTheEngine(t *testing.T) {
	t.Parallel()
	engine := &recordingEngine{}
	module := newValidationService(engine)
	ctx := context.Background()
	record := peopledirectory.GuardianContactRecord{FirstName: "A", LastName: "B"}
	grantedAt := time.Date(2026, 9, 21, 8, 0, 0, 0, time.UTC)

	require.ErrorIs(t, module.UpdateGuardianContact(ctx, 0, record), peopledirectory.ErrInvalidGuardian)
	require.ErrorIs(t, module.ReplaceGuardianPhones(ctx, 0, nil), peopledirectory.ErrInvalidGuardian)
	_, err := module.AddGuardianPhoneRecord(ctx, -1, peopledirectory.GuardianPhoneRecord{})
	require.ErrorIs(t, err, peopledirectory.ErrInvalidGuardian)
	require.ErrorIs(t, module.SetGuardianPhoneNumber(ctx, 0, "0170 1234567"), peopledirectory.ErrInvalidGuardian)
	require.ErrorIs(t, module.DeleteGuardianPhoneRecord(ctx, 0), peopledirectory.ErrInvalidGuardian)
	_, _, err = module.LinkGuardianContact(ctx, peopledirectory.GuardianContactLink{StudentID: 1})
	require.ErrorIs(t, err, peopledirectory.ErrInvalidGuardian)
	_, err = module.PatchGuardianLinkPickup(ctx, peopledirectory.GuardianLinkPickupPatch{})
	require.ErrorIs(t, err, peopledirectory.ErrInvalidGuardian)
	_, err = module.PatchGuardianLinkPickup(ctx, peopledirectory.GuardianLinkPickupPatch{LinkID: 1})
	require.ErrorIs(t, err, peopledirectory.ErrInvalidGuardian, "a patch without a column is refused")
	_, err = module.SetGuardianPortalLocale(ctx, 0, "de")
	require.ErrorIs(t, err, peopledirectory.ErrInvalidGuardian)
	require.ErrorIs(t, module.SetStudentPhotoConsent(ctx, peopledirectory.StudentPhotoState{}), peopledirectory.ErrInvalidStudent)
	require.ErrorIs(t, module.SetStudentPhotoConsent(ctx, peopledirectory.StudentPhotoState{StudentID: 1, PhotoConsentGivenAt: &grantedAt}),
		peopledirectory.ErrInvalidStudent)
	assert.Zero(t, engine.calls, "invalid input never reaches the engine")

	_, err = module.PatchGuardianLinkPickup(ctx, peopledirectory.GuardianLinkPickupPatch{LinkID: 1, SetPickupNotes: true})
	require.NoError(t, err, "clearing the notes alone is a valid patch")
	_, err = module.SetGuardianPortalLocale(ctx, 1, "")
	require.NoError(t, err, "the caller validates the locale, the owner writes it as given")
	assert.Equal(t, 2, engine.calls)
}
