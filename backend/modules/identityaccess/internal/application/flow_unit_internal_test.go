package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

type flowUnitKey struct{}

// TestFlowUnitBindsTranslatesAndGuards pins what the spelled-out wrappers did
// before ADR 0177: fn runs under the attached unit of work, its error is
// translated, and a nil receiver reports its sentinel untranslated without
// running fn.
func TestFlowUnitBindsTranslatesAndGuards(t *testing.T) {
	t.Parallel()
	inner := errors.New("inner")
	public := errors.New("public")
	unit := flowUnit{
		attach: func(ctx context.Context) context.Context { return context.WithValue(ctx, flowUnitKey{}, "bound") },
		translate: func(err error) error {
			if err == nil {
				return nil
			}
			return errors.Join(public, err)
		},
	}

	value, err := runFlow(context.Background(), unit, func(ctx context.Context) (string, error) {
		return ctx.Value(flowUnitKey{}).(string), inner
	})
	require.Equal(t, "bound", value, "the value survives the translation")
	require.ErrorIs(t, err, public)
	require.ErrorIs(t, err, inner)

	first, second, err := runFlow2(context.Background(), unit, func(context.Context) (int, bool, error) { return 7, true, nil })
	require.NoError(t, err)
	require.Equal(t, 7, first)
	require.True(t, second)

	require.ErrorIs(t, runFlowErr(context.Background(), unit, func(context.Context) error { return inner }), public)

	sentinel := errors.New("unavailable")
	guarded := flowUnit{unavailable: sentinel, translate: unit.translate}
	called := false
	value, err = runFlow(context.Background(), guarded, func(context.Context) (string, error) { called = true; return "x", nil })
	requireSameError(t, sentinel, err)
	require.Empty(t, value)
	first, second, err = runFlow2(context.Background(), guarded, func(context.Context) (int, bool, error) { called = true; return 1, true, nil })
	requireSameError(t, sentinel, err)
	require.Zero(t, first)
	require.False(t, second)
	requireSameError(t, sentinel, runFlowErr(context.Background(), guarded, func(context.Context) error { called = true; return nil }))
	require.False(t, called, "a nil receiver runs no flow")
}

// TestNilReceiverFlowsReportTheirSentinel covers the flow() units: each nil
// receiver yields its own unavailable sentinel, untranslated.
func TestNilReceiverFlowsReportTheirSentinel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var authentication *AccountAuthentication
	_, err := authentication.LoginWithMFAGate(ctx, "", "", "", "", "", "")
	requireSameError(t, errAccountAuthenticationUnavailable, err)
	var administration *AccountAdministration
	requireSameError(t, errAccountAdministrationUnavailable, administration.ChangeAccountPassword(ctx, 1, "", ""))
	var lifecycle *AccountLifecycle
	_, err = lifecycle.PreviewStaffOffboarding(ctx, 1)
	requireSameError(t, errAccountLifecycleUnavailable, err)
	var operators *OperatorProvisioning
	_, err = operators.ChangeOperatorStatus(ctx, identityaccess.OperatorStatusChange{})
	requireSameError(t, errOperatorProvisioningUnavailable, err)
	var reset *PasswordReset
	_, err = reset.DeleteSpentPasswordResetTokens(ctx)
	requireSameError(t, errPasswordResetUnavailable, err)
	var roles *RoleAdministration
	requireSameError(t, errRoleAdministrationUnavailable, roles.DeleteRole(ctx, 1))
	var passkeys *AccountPasskeyFlows
	requireSameError(t, identityaccess.ErrPasskeyFlowsUnavailable, passkeys.RevokeAccountPasskeyCredential(ctx, 1, 1))
	var invitations *SchoolInvitation
	requireSameError(t, errSchoolInvitationUnavailable, invitations.ResendSchoolInvitation(ctx, 1, 1))
	var operatorAuth *OperatorAuthentication
	requireSameError(t, errOperatorAuthenticationUnavailable, operatorAuth.ChangeOperatorPassword(ctx, 1, "", ""))
	var access *OperatorAccountAccess
	_, err = access.ListAccountTenantAccess(ctx, 1)
	requireSameError(t, errOperatorAuthenticationUnavailable, err)
}

// requireSameError asserts the very sentinel, not a translation of it.
func requireSameError(t *testing.T, want, got error) {
	t.Helper()
	require.Truef(t, want == got, "want the untranslated %v, got %v", want, got) //nolint:errorlint // identity is the contract
}
