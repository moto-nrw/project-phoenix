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
	t.Parallel()

	svc := audienceTestService(&enrollmentModels.LateInvite{}, nil, linkedParentsPhaseOpenWindow())

	phase, err := svc.LoadPublicPhaseWithLateInvite(context.Background(), 4242, time.Now(), "valid-token")
	require.NoError(t, err)
	require.NotNil(t, phase)
	require.Equal(t, enrollmentModels.PhaseAudienceLinkedParents, phase.Audience)
}

// Without a late-invite token the public gate still refuses a linked_parents
// phase, so a guessed public URL cannot load a hidden phase.
func TestLoadPublicPhaseWithLateInvite_NoInviteStillRestricted(t *testing.T) {
	t.Parallel()

	svc := audienceTestService(nil, enrollmentModels.ErrLateInviteNotFound, linkedParentsPhaseOpenWindow())

	_, err := svc.LoadPublicPhaseWithLateInvite(context.Background(), 4242, time.Now(), "")
	require.ErrorIs(t, err, ErrPhaseAudienceRestricted)
}

// An invalid/expired invite token does not grant access to a restricted
// phase — the audience gate still applies.
func TestLoadPublicPhaseWithLateInvite_InvalidInviteStaysRestricted(t *testing.T) {
	t.Parallel()

	svc := audienceTestService(nil, enrollmentModels.ErrLateInviteNotFound, linkedParentsPhaseOpenWindow())

	_, err := svc.LoadPublicPhaseWithLateInvite(context.Background(), 4242, time.Now(), "bad-token")
	require.ErrorIs(t, err, ErrPhaseAudienceRestricted)
}

// A valid invite also opens a closed window for a restricted phase, so the
// invited recipient can load the form after the window has closed.
func TestLoadPublicPhaseWithLateInvite_ValidInviteOpensClosedRestrictedPhase(t *testing.T) {
	t.Parallel()

	phase := linkedParentsPhaseOpenWindow()
	past := time.Now().Add(-time.Hour)
	phase.EnrollmentCloseAt = &past // window is closed

	svc := audienceTestService(&enrollmentModels.LateInvite{}, nil, phase)

	loaded, err := svc.LoadPublicPhaseWithLateInvite(context.Background(), 4242, time.Now(), "valid-token")
	require.NoError(t, err)
	require.NotNil(t, loaded)
}

// A database/driver failure during the invite lookup leaves the invite status
// unknown. Downgrading it to "no invite" would render an outage as a 404 and
// tell a legitimate recipient their link is invalid, so the error propagates
// (the handler turns it into a 500) instead of becoming a gate error (#1663).
func TestLoadPublicPhaseWithLateInvite_LookupFailurePropagates(t *testing.T) {
	t.Parallel()

	lookupErr := errors.New("connection reset by peer")
	svc := audienceTestService(nil, lookupErr, linkedParentsPhaseOpenWindow())

	_, err := svc.LoadPublicPhaseWithLateInvite(context.Background(), 4242, time.Now(), "some-token")
	require.ErrorIs(t, err, lookupErr)
	require.NotErrorIs(t, err, ErrPhaseAudienceRestricted)
	require.NotErrorIs(t, err, ErrLateInviteInvalid)
}

// Same for a closed window on an unrestricted phase: a failed lookup must not
// masquerade as an invalid invite.
func TestLoadPublicPhaseWithLateInvite_LookupFailurePropagatesOnClosedPhase(t *testing.T) {
	t.Parallel()

	phase := linkedParentsPhaseOpenWindow()
	phase.Audience = enrollmentModels.PhaseAudienceOpen
	past := time.Now().Add(-time.Hour)
	phase.EnrollmentCloseAt = &past // window is closed

	lookupErr := errors.New("connection reset by peer")
	svc := audienceTestService(nil, lookupErr, phase)

	_, err := svc.LoadPublicPhaseWithLateInvite(context.Background(), 4242, time.Now(), "some-token")
	require.ErrorIs(t, err, lookupErr)
	require.NotErrorIs(t, err, ErrLateInviteInvalid)
	require.NotErrorIs(t, err, ErrEnrollmentWindowClosed)
}

// A valid late invite is a per-recipient eligibility override: Submit skips
// validateChildGradeEligibility for it via AllowClosedPhase, so an admin can
// invite a child from outside the restricted grade. The form-load response must
// therefore drop the restriction too — otherwise the grade select offers only
// the phase's grades and the recipient cannot declare the very grade the invite
// exists for. Mirrors the class-list override, which keeps the full offered list
// for a late invite (#1663).
func TestLoadPublicPhaseWithLateInvite_ValidInviteClearsGradeRestriction(t *testing.T) {
	t.Parallel()

	phase := linkedParentsPhaseOpenWindow()
	phase.Audience = enrollmentModels.PhaseAudienceOpen
	phase.EligibleGradeLevels = []int{3}

	svc := audienceTestService(&enrollmentModels.LateInvite{}, nil, phase)

	loaded, err := svc.LoadPublicPhaseWithLateInvite(context.Background(), 4242, time.Now(), "valid-token")
	require.NoError(t, err)
	require.Empty(t, loaded.EligibleGradeLevels,
		"a late-invite recipient must be able to pick any grade the submit path would accept")
}

// The enrollee (parents-portal) gate runs through the same override, so the
// embedded form behaves identically.
func TestLoadEnrolleePhaseWithLateInvite_ValidInviteClearsGradeRestriction(t *testing.T) {
	t.Parallel()

	phase := linkedParentsPhaseOpenWindow()
	phase.EligibleGradeLevels = []int{3}

	svc := audienceTestService(&enrollmentModels.LateInvite{}, nil, phase)

	loaded, err := svc.LoadEnrolleePhaseWithLateInvite(context.Background(), 4242, time.Now(), "valid-token",
		EnrolleeAudienceAccess{LinkedParents: true})
	require.NoError(t, err)
	require.Empty(t, loaded.EligibleGradeLevels)
}

// Without a valid invite the restriction stays: a normal self-service load must
// keep narrowing the grade select to the grades submit will accept.
func TestLoadPublicPhaseWithLateInvite_NoInviteKeepsGradeRestriction(t *testing.T) {
	t.Parallel()

	phase := linkedParentsPhaseOpenWindow()
	phase.Audience = enrollmentModels.PhaseAudienceOpen
	phase.EligibleGradeLevels = []int{3}

	svc := audienceTestService(nil, enrollmentModels.ErrLateInviteNotFound, phase)

	loaded, err := svc.LoadPublicPhaseWithLateInvite(context.Background(), 4242, time.Now(), "")
	require.NoError(t, err)
	require.Equal(t, []int{3}, loaded.EligibleGradeLevels)
}

// An invalid/expired token is not an override either — the restriction must
// survive it exactly as the audience and window gates do.
func TestLoadPublicPhaseWithLateInvite_InvalidInviteKeepsGradeRestriction(t *testing.T) {
	t.Parallel()

	phase := linkedParentsPhaseOpenWindow()
	phase.Audience = enrollmentModels.PhaseAudienceOpen
	phase.EligibleGradeLevels = []int{3}

	svc := audienceTestService(nil, enrollmentModels.ErrLateInviteNotFound, phase)

	loaded, err := svc.LoadPublicPhaseWithLateInvite(context.Background(), 4242, time.Now(), "bad-token")
	require.NoError(t, err)
	require.Equal(t, []int{3}, loaded.EligibleGradeLevels)
}

// The authenticated enrollee gate keeps loading restricted phases without any
// invite — the guardian is already authorized for that audience.
func TestLoadEnrolleePhaseWithLateInvite_AllowsRestrictedWithoutInvite(t *testing.T) {
	t.Parallel()

	svc := audienceTestService(nil, enrollmentModels.ErrLateInviteNotFound, linkedParentsPhaseOpenWindow())

	phase, err := svc.LoadEnrolleePhaseWithLateInvite(context.Background(), 4242, time.Now(), "",
		EnrolleeAudienceAccess{LinkedParents: true})
	require.NoError(t, err)
	require.NotNil(t, phase)
}

// Access is resolved PER audience (#1663): the linked_parents fact (any
// submit-permitted guardian relationship) must NOT open an existing_students
// phase, which needs a still-enrolled child. Otherwise the parents portal
// serves a form for a phase its own picker hides and whose submit can only
// fail with ErrChildNotEnrolled.
func TestLoadEnrolleePhaseWithLateInvite_LinkedParentsAccessDoesNotOpenExistingStudents(t *testing.T) {
	t.Parallel()

	phase := linkedParentsPhaseOpenWindow()
	phase.Audience = enrollmentModels.PhaseAudienceExistingStudents

	svc := audienceTestService(nil, enrollmentModels.ErrLateInviteNotFound, phase)

	_, err := svc.LoadEnrolleePhaseWithLateInvite(context.Background(), 4242, time.Now(), "",
		EnrolleeAudienceAccess{LinkedParents: true})
	require.ErrorIs(t, err, ErrPhaseAudienceRestricted)
}

// The matching fact does open it.
func TestLoadEnrolleePhaseWithLateInvite_EnrolledAccessOpensExistingStudents(t *testing.T) {
	t.Parallel()

	phase := linkedParentsPhaseOpenWindow()
	phase.Audience = enrollmentModels.PhaseAudienceExistingStudents

	svc := audienceTestService(nil, enrollmentModels.ErrLateInviteNotFound, phase)

	loaded, err := svc.LoadEnrolleePhaseWithLateInvite(context.Background(), 4242, time.Now(), "",
		EnrolleeAudienceAccess{ExistingStudents: true})
	require.NoError(t, err)
	require.NotNil(t, loaded)
}

// An existing_students-only account must not reach a linked_parents phase
// either — the per-audience mapping is not a hierarchy.
func TestLoadEnrolleePhaseWithLateInvite_ExistingStudentsAccessDoesNotOpenLinkedParents(t *testing.T) {
	t.Parallel()

	svc := audienceTestService(nil, enrollmentModels.ErrLateInviteNotFound, linkedParentsPhaseOpenWindow())

	_, err := svc.LoadEnrolleePhaseWithLateInvite(context.Background(), 4242, time.Now(), "",
		EnrolleeAudienceAccess{ExistingStudents: true})
	require.ErrorIs(t, err, ErrPhaseAudienceRestricted)
}
