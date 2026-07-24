package enrollment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// --- stubs -----------------------------------------------------------------

type stubAudiencePhaseRepo struct {
	enrollmentModels.PhaseRepository
	phase *enrollmentModels.Phase
}

func (s stubAudiencePhaseRepo) FindByID(context.Context, int64) (*enrollmentModels.Phase, error) {
	return s.phase, nil
}

type stubAudienceLateInviteRepo struct {
	enrollmentModels.LateInviteRepository
	invite *enrollmentModels.LateInvite
	err    error
}

func (s stubAudienceLateInviteRepo) FindUsableByTokenHash(context.Context, string, int64, time.Time) (*enrollmentModels.LateInvite, error) {
	return s.invite, s.err
}

// enrollmentEnabledSettings answers just enough for isEnrollmentEnabled.
type enrollmentEnabledSettings struct{}

func (enrollmentEnabledSettings) HasTenantOverride(context.Context, string) (bool, error) {
	return true, nil
}

func (enrollmentEnabledSettings) ResolveBool(_ context.Context, key string) (bool, error) {
	return key == configModel.KeyEnrollmentEnabled, nil
}

func (enrollmentEnabledSettings) ResolveString(context.Context, string) (string, error) {
	return "", nil
}

func (enrollmentEnabledSettings) ResolveInt(context.Context, string) (int, error) {
	return 0, nil
}

func linkedParentsPhaseOpenWindow() *enrollmentModels.Phase {
	// No EnrollmentOpenAt/CloseAt bounds → window always open, so these
	// tests isolate the audience gate from the window check.
	return &enrollmentModels.Phase{
		Audience: enrollmentModels.PhaseAudienceLinkedParents,
		IsActive: true,
	}
}

func audienceTestService(invite *enrollmentModels.LateInvite, inviteErr error, phase *enrollmentModels.Phase) *requestService {
	return &requestService{RequestServiceConfig: RequestServiceConfig{
		Settings:       enrollmentEnabledSettings{},
		PhaseRepo:      stubAudiencePhaseRepo{phase: phase},
		LateInviteRepo: stubAudienceLateInviteRepo{invite: invite, err: inviteErr},
	}}
}

// --- tests -----------------------------------------------------------------

// A valid late invite lifts the public audience gate: an admin who mints an
// invite for a linked_parents phase generates the tenant's public URL, and
// the recipient must be able to load the form (#1663).
func TestLoadPublicPhaseWithLateInvite_ValidInviteBypassesAudienceGate(t *testing.T) {
	svc := audienceTestService(&enrollmentModels.LateInvite{}, nil, linkedParentsPhaseOpenWindow())

	phase, err := svc.LoadPublicPhaseWithLateInvite(context.Background(), 4242, time.Now(), "valid-token")
	require.NoError(t, err)
	require.NotNil(t, phase)
	require.Equal(t, enrollmentModels.PhaseAudienceLinkedParents, phase.Audience)
}

// Without a late-invite token the public gate still refuses a linked_parents
// phase, so a guessed public URL cannot load a hidden phase.
func TestLoadPublicPhaseWithLateInvite_NoInviteStillRestricted(t *testing.T) {
	svc := audienceTestService(nil, errors.New("no token"), linkedParentsPhaseOpenWindow())

	_, err := svc.LoadPublicPhaseWithLateInvite(context.Background(), 4242, time.Now(), "")
	require.ErrorIs(t, err, ErrPhaseAudienceRestricted)
}

// An invalid/expired invite token does not grant access to a restricted
// phase — the audience gate still applies.
func TestLoadPublicPhaseWithLateInvite_InvalidInviteStaysRestricted(t *testing.T) {
	svc := audienceTestService(nil, errors.New("expired"), linkedParentsPhaseOpenWindow())

	_, err := svc.LoadPublicPhaseWithLateInvite(context.Background(), 4242, time.Now(), "bad-token")
	require.ErrorIs(t, err, ErrPhaseAudienceRestricted)
}

// A valid invite also opens a closed window for a restricted phase, so the
// invited recipient can load the form after the window has closed.
func TestLoadPublicPhaseWithLateInvite_ValidInviteOpensClosedRestrictedPhase(t *testing.T) {
	phase := linkedParentsPhaseOpenWindow()
	past := time.Now().Add(-time.Hour)
	phase.EnrollmentCloseAt = &past // window is closed

	svc := audienceTestService(&enrollmentModels.LateInvite{}, nil, phase)

	loaded, err := svc.LoadPublicPhaseWithLateInvite(context.Background(), 4242, time.Now(), "valid-token")
	require.NoError(t, err)
	require.NotNil(t, loaded)
}

// The authenticated enrollee gate keeps loading restricted phases without any
// invite — the guardian is already authorized.
func TestLoadEnrolleePhaseWithLateInvite_AllowsRestrictedWithoutInvite(t *testing.T) {
	svc := audienceTestService(nil, errors.New("no token"), linkedParentsPhaseOpenWindow())

	phase, err := svc.LoadEnrolleePhaseWithLateInvite(context.Background(), 4242, time.Now(), "")
	require.NoError(t, err)
	require.NotNil(t, phase)
}
