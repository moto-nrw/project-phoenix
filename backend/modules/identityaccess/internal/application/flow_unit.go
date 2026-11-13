package application

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
)

// flowUnit is what a public flow of one receiver shares: the unit of work
// the composition attaches, the translation onto the public contract, and
// the sentinel a nil receiver reports. Each receiver builds it in flow();
// a nil receiver yields only its sentinel (ADR 0177).
type flowUnit struct {
	attach      func(context.Context) context.Context
	translate   func(error) error
	unavailable error
}

// runFlow runs fn under the composed unit of work and translates its error.
// A nil receiver returns the zero value and its untranslated sentinel
// without calling fn, as the spelled-out nil guards did.
func runFlow[T any](ctx context.Context, unit flowUnit, fn func(context.Context) (T, error)) (T, error) {
	if unit.unavailable != nil {
		var zero T
		return zero, unit.unavailable
	}
	value, err := fn(bindRuntime(unit.attach, ctx))
	return value, unit.translate(err)
}

// runFlowErr is runFlow for a flow without a result.
func runFlowErr(ctx context.Context, unit flowUnit, fn func(context.Context) error) error {
	if unit.unavailable != nil {
		return unit.unavailable
	}
	return unit.translate(fn(bindRuntime(unit.attach, ctx)))
}

// runFlow2 is runFlow for a flow with two results.
func runFlow2[A, B any](ctx context.Context, unit flowUnit, fn func(context.Context) (A, B, error)) (A, B, error) {
	if unit.unavailable != nil {
		var first A
		var second B
		return first, second, unit.unavailable
	}
	first, second, err := fn(bindRuntime(unit.attach, ctx))
	return first, second, unit.translate(err)
}

// The units of the receivers whose public flows run through the helpers.

func (s *AccountAuthentication) flow() flowUnit {
	if s == nil {
		return flowUnit{unavailable: errAccountAuthenticationUnavailable}
	}
	return flowUnit{attach: s.attach, translate: authenticationError}
}

func (a *AccountAdministration) flow() flowUnit {
	if a == nil {
		return flowUnit{unavailable: errAccountAdministrationUnavailable}
	}
	return flowUnit{attach: a.auth.attach, translate: administrationError}
}

func (l *AccountLifecycle) flow() flowUnit {
	if l == nil {
		return flowUnit{unavailable: errAccountLifecycleUnavailable}
	}
	return flowUnit{attach: l.auth.attach, translate: lifecycleError}
}

func (p *OperatorProvisioning) flow() flowUnit {
	if p == nil {
		return flowUnit{unavailable: errOperatorProvisioningUnavailable}
	}
	return flowUnit{attach: p.attach, translate: operatorProvisioningError}
}

func (r *PasswordReset) flow() flowUnit {
	if r == nil {
		return flowUnit{unavailable: errPasswordResetUnavailable}
	}
	return flowUnit{attach: r.auth.attach, translate: passwordResetError}
}

func (r *RoleAdministration) flow() flowUnit {
	if r == nil {
		return flowUnit{unavailable: errRoleAdministrationUnavailable}
	}
	return flowUnit{attach: r.attach, translate: roleError}
}

func (f *AccountPasskeyFlows) flow() flowUnit {
	if f == nil {
		return flowUnit{unavailable: identityaccess.ErrPasskeyFlowsUnavailable}
	}
	return flowUnit{attach: f.attach, translate: passkeyError}
}

func (s *SchoolInvitation) flow() flowUnit {
	if s == nil {
		return flowUnit{unavailable: errSchoolInvitationUnavailable}
	}
	return flowUnit{attach: s.attach, translate: invitationError}
}

func (s *OperatorAuthentication) flow() flowUnit {
	if s == nil {
		return flowUnit{unavailable: errOperatorAuthenticationUnavailable}
	}
	return flowUnit{attach: s.attach, translate: operatorError}
}

func (s *OperatorAccountAccess) flow() flowUnit {
	if s == nil {
		return flowUnit{unavailable: errOperatorAuthenticationUnavailable}
	}
	return flowUnit{attach: s.attach, translate: operatorError}
}
