package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// errDecisionGuardianAccessRequired reports a decision flow composed without
// the Identity & Access capability. The old repositories were optional and
// their absence silently skipped the account attach, which left an approved
// child invisible to a parent with a working portal login; the capability is
// a mandatory collaborator of the acceptance workflow.
var errDecisionGuardianAccessRequired = errors.New("decision: guardian access capability is not configured")

// submitterOwnsEmail reports whether the submitted guardian email is the
// authenticated submitter's OWN account address. It is the ownership proof
// resolveGuardianProfile requires before letting an authenticated approval
// claim an unlinked guardian profile.
//
// An account deleted between submission and decision answers true without a
// comparison, deliberately: the by-id attach already falls back to the
// email-owner lookup, which can only bind the profile to whoever owns THAT
// address, never to the caller. Any other lookup failure fails the approval so
// it can be retried; ownership is never assumed on an outage.
func (d *Decisions) submitterOwnsEmail(ctx context.Context, accountID int64, email string) (bool, error) {
	if d.deps.GuardianAccess == nil {
		return false, errDecisionGuardianAccessRequired
	}
	account, err := d.deps.GuardianAccess.FindAccount(ctx, accountID)
	if err != nil {
		if errors.Is(err, identityaccess.ErrAccountNotFound) {
			d.logger().Warn("decision: submitting account no longer resolvable, skipping email ownership check",
				slog.Int64("guardian_account_id", accountID),
			)
			return true, nil
		}
		return false, fmt.Errorf("decision: load submitting account %d: %w", accountID, err)
	}
	return strings.EqualFold(strings.TrimSpace(account.Email), email), nil
}

// attachExistingAccountIfPresent looks up the parent email among the
// platform accounts (no tenant - emails are unique platform-wide). If an
// account exists, it grants that account guardian access to this school and
// links the per-tenant guardian profile to it. Returns true when the
// attachment happened so the caller can skip enqueueing an invitation.
//
// Without this step, an admin approving the same parent at a second school
// would queue another guardian-invitation email, and the accept flow
// overwrites the existing password hash ("I just got accepted at school B and
// now my school A password no longer works"). Linking directly here keeps the
// parent silent and on the same credentials.
//
// Not-found is the common case (parent has no portal account yet) and lets
// the invitation flow run. Any other lookup failure fails the approval: a
// swallowed outage would invite a parent who already has a password.
func (d *Decisions) attachExistingAccountIfPresent(ctx context.Context, guardian *GuardianProfile) (bool, error) {
	if d.deps.GuardianAccess == nil {
		return false, errDecisionGuardianAccessRequired
	}
	if guardian.Email == nil || strings.TrimSpace(*guardian.Email) == "" {
		return false, nil
	}
	email := strings.TrimSpace(strings.ToLower(*guardian.Email))
	account, err := d.deps.GuardianAccess.FindAccountByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, identityaccess.ErrAccountNotFound) {
			d.logger().Debug("decision: no platform account for guardian email, invitation flow stays responsible",
				slog.Int64("guardian_profile_id", guardian.ID),
			)
			return false, nil
		}
		return false, fmt.Errorf("attach: account lookup: %w", err)
	}
	return d.attachAccountToGuardian(ctx, guardian, account.ID, "attach")
}

// attachAccountToGuardian runs the shared attach tail for both account
// resolution paths: guardian access for this tenant (active mapping plus
// guardian role, idempotent) and the account link on the per-tenant profile.
// errPrefix keeps the historical per-path error wording ("attach" /
// "attach by id").
func (d *Decisions) attachAccountToGuardian(ctx context.Context, guardian *GuardianProfile, accountID int64, errPrefix string) (bool, error) {
	if err := d.ensureGuardianTenantAccess(ctx, accountID, errPrefix); err != nil {
		return false, err
	}
	// Linking also flips has_account=true so future approvals for the same
	// profile see the linked state.
	if err := d.deps.People.GuardianProfiles.LinkGuardianAccount(ctx, guardian.ID, accountID); err != nil {
		return false, fmt.Errorf("%s: link profile: %w", errPrefix, err)
	}
	guardian.AccountID = &accountID
	guardian.HasAccount = true
	return true, nil
}

// attachExistingAccountByID links the guardian profile to the account
// identified by accountID directly, bypassing the email lookup that
// attachExistingAccountIfPresent uses. Called when the enrollment request
// was submitted by an authenticated parent - the JWT-derived account_id is
// more authoritative than the email field (which the parent could have typed
// differently in the form).
//
// An account deleted between submission and decision falls back to the email
// lookup so the approval still goes through; any other lookup failure fails
// the approval.
func (d *Decisions) attachExistingAccountByID(ctx context.Context, guardian *GuardianProfile, accountID int64) (bool, error) {
	if d.deps.GuardianAccess == nil {
		return false, errDecisionGuardianAccessRequired
	}
	account, err := d.deps.GuardianAccess.FindAccount(ctx, accountID)
	if err != nil {
		if !errors.Is(err, identityaccess.ErrAccountNotFound) {
			return false, fmt.Errorf("attach by id: account lookup: %w", err)
		}
		d.logger().Warn("decision: request guardian_account_id no longer resolvable, falling back to email",
			slog.Int64("guardian_account_id", accountID),
		)
		if guardian.Email != nil && strings.TrimSpace(*guardian.Email) != "" {
			return d.attachExistingAccountIfPresent(ctx, guardian)
		}
		return false, nil
	}
	return d.attachAccountToGuardian(ctx, guardian, account.ID, "attach by id")
}

// ensureGuardianTenantAccess makes an account's guardian membership in the
// CURRENT tenant usable through the Identity & Access capability: the
// school mapping is created OR REACTIVATED, and the guardian base role is
// assigned once for this tenant.
//
// Reactivation (not create-if-missing) is deliberate: a mapping left inactive
// by a previous offboarding would otherwise survive an approval untouched —
// mapping present, status 'inactive', parent locked out of the school they
// were just approved for.
func (d *Decisions) ensureGuardianTenantAccess(ctx context.Context, accountID int64, errPrefix string) error {
	if d.deps.GuardianAccess == nil {
		return errDecisionGuardianAccessRequired
	}
	if d.deps.Runtime.TenantID(ctx) == 0 {
		return fmt.Errorf("%s: tenant not in context", errPrefix)
	}
	granted, err := d.deps.GuardianAccess.GrantGuardianTenantAccess(ctx, accountID)
	if err != nil {
		return fmt.Errorf("%s: guardian tenant access: %w", errPrefix, err)
	}
	if granted.RoleAssigned {
		d.logger().Debug("decision: guardian role assigned for tenant",
			slog.Int64("account_id", accountID),
			slog.Int64("tenant_id", granted.TenantID),
		)
	}
	return nil
}
