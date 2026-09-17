package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// The admin side of account MFA (#3331): disabling a user's second factor
// and the force_on / force_off overrides, from the school admin surface and
// from the operator surface.
//
// `auth.accounts` has no tenant column and no row-level security, so every
// path here proves the target belongs to the acting school before it writes.
// Without that check a school admin holding users:manage could disable MFA
// on any account anywhere by guessing a primary key. Both accepted and
// refused attempts produce an audit row, so scanning is visible.

// adminPermission is the permission a school admin needs to manage another
// account's MFA.
const adminPermission = "users:manage"

// actorTypeAccount and actorTypeOperator tag the audit metadata so the
// ledger tells the two surfaces apart on the same account.
const (
	actorTypeAccount  = "account"
	actorTypeOperator = "operator"
)

// AdminDisable wipes the target's enrollment and trusted devices from the
// school admin surface.
func (f *AccountMFAFlows) AdminDisable(ctx context.Context, actorID, actorTenantID, targetAccountID int64, reason string, actorPermissions []string) error {
	if err := f.requireAdminPermission(ctx, actorPermissions, adminOverrideFailure{
		ActorType: actorTypeAccount, ActorID: actorID, TargetAccountID: targetAccountID,
		Action: "disable", Reason: reason,
	}); err != nil {
		return err
	}
	if err := f.requireSchoolMembership(ctx, actorTypeAccount, actorID, actorTenantID, targetAccountID, "disable"); err != nil {
		return err
	}
	// The membership check just proved the target sits in the actor's
	// school, so that school is also the one the audit row is filed under.
	return f.adminDisable(ctx, actorTypeAccount, actorID, actorTenantID, targetAccountID, reason)
}

// OperatorAdminDisable is the operator variant. It skips the users:manage
// check — operator routes carry their own platform gate — and still proves
// the school membership, so a future non-HTTP caller cannot act across
// schools.
func (f *AccountMFAFlows) OperatorAdminDisable(ctx context.Context, operatorID, schoolID, targetAccountID int64, reason string) error {
	if err := f.requireSchoolMembership(ctx, actorTypeOperator, operatorID, schoolID, targetAccountID, "disable"); err != nil {
		return err
	}
	// Operator routes run outside the tenant middleware, so the school is
	// passed explicitly or the audit row would lose its tenant and be
	// dropped.
	return f.adminDisable(ctx, actorTypeOperator, operatorID, schoolID, targetAccountID, reason)
}

// adminDisable is the cascade both surfaces share.
func (f *AccountMFAFlows) adminDisable(ctx context.Context, actorType string, actorID, targetTenantID, targetAccountID int64, reason string) error {
	reason = strings.TrimSpace(reason)
	failure := adminOverrideFailure{
		ActorType: actorType, ActorID: actorID, TargetTenantID: targetTenantID,
		TargetAccountID: targetAccountID, Action: "disable",
	}
	if reason == "" {
		failure.ErrMsg = domain.ErrMFAReasonRequired.Error()
		f.recordAdminFailure(ctx, failure)
		return domain.ErrMFAReasonRequired
	}
	if err := f.Disable(ctx, targetAccountID); err != nil {
		failure.Reason, failure.ErrMsg = reason, err.Error()
		f.recordAdminFailure(ctx, failure)
		return err
	}
	f.recordAuthEvent(ctx, domain.AuthEvent{
		AccountID: targetAccountID, TenantID: targetTenantID, Type: domain.AuthEventMFAAdminOverride,
		Success: true, IPAddress: domain.MFAAuditFallbackIP,
		MFA: &domain.MFAEvidence{Admin: &domain.MFAAdminEvidence{
			ActorType: actorType, ActorAccountID: actorID, Action: "disable", Reason: reason,
		}},
	})
	return nil
}

// SetMFAOverride writes a school-scoped override from the school admin
// surface. "none" deletes that school's row and never touches the
// platform-wide one, which only the operator can write.
func (f *AccountMFAFlows) SetMFAOverride(ctx context.Context, actorID, actorTenantID, targetAccountID int64, override, reason string, actorPermissions []string) error {
	if err := f.requireAdminPermission(ctx, actorPermissions, adminOverrideFailure{
		ActorType: actorTypeAccount, ActorID: actorID, TargetAccountID: targetAccountID,
		Action: "set_override", Override: override, Reason: reason,
	}); err != nil {
		return err
	}
	if err := f.requireSchoolMembership(ctx, actorTypeAccount, actorID, actorTenantID, targetAccountID, "set_override"); err != nil {
		return err
	}
	return f.setTenantOverride(ctx, actorTypeAccount, actorID, actorTenantID, targetAccountID, override, reason)
}

// OperatorSetMFAOverride is the operator variant of SetMFAOverride. It also
// writes a school-scoped row, not a platform-wide one.
func (f *AccountMFAFlows) OperatorSetMFAOverride(ctx context.Context, operatorID, schoolID, targetAccountID int64, override, reason string) error {
	if err := f.requireSchoolMembership(ctx, actorTypeOperator, operatorID, schoolID, targetAccountID, "set_override"); err != nil {
		return err
	}
	return f.setTenantOverride(ctx, actorTypeOperator, operatorID, schoolID, targetAccountID, override, reason)
}

// setTenantOverride writes the row and revokes the school's trusted devices
// in one transaction: a failed revoke must roll the flip back, or a
// force_off would leave the account flagged "no MFA" at this school while
// its remember-device cookies still verify.
func (f *AccountMFAFlows) setTenantOverride(ctx context.Context, actorType string, actorID, targetTenantID, targetAccountID int64, override, reason string) error {
	failure := adminOverrideFailure{
		ActorType: actorType, ActorID: actorID, TargetTenantID: targetTenantID,
		TargetAccountID: targetAccountID, Action: "set_override", Override: override, Reason: reason,
	}
	if !domain.IsValidMFAAdminOverride(override) {
		failure.ErrMsg = domain.ErrMFAInvalidOverride.Error()
		f.recordAdminFailure(ctx, failure)
		return domain.ErrMFAInvalidOverride
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		failure.Reason, failure.ErrMsg = "", domain.ErrMFAReasonRequired.Error()
		f.recordAdminFailure(ctx, failure)
		return domain.ErrMFAReasonRequired
	}
	failure.Reason = reason
	if targetTenantID == 0 {
		// A zero school would silently target the platform-wide slot, which
		// only the operator's account-wide path may write.
		return domain.ErrMFATargetTenantRequired
	}

	previous := domain.MFAAdminOverrideNone
	txErr := f.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		if err := f.records.LockAccountForOverrideWrite(txCtx, targetAccountID); err != nil {
			return err
		}
		existing, found, err := f.records.FindTenantOverride(txCtx, targetAccountID, targetTenantID)
		if err != nil {
			return fmt.Errorf("load existing tenant override: %w", err)
		}
		if found {
			previous = existing.Override
		}
		if override == domain.MFAAdminOverrideNone {
			if err := f.records.DeleteTenantOverride(txCtx, targetAccountID, targetTenantID); err != nil {
				return fmt.Errorf("clear tenant override: %w", err)
			}
			return nil
		}
		tenantID := targetTenantID
		if err := f.records.UpsertTenantOverride(txCtx, domain.AccountMFAOverride{
			AccountID: targetAccountID, TenantID: &tenantID, Override: override,
			SetBy: actorID, SetByType: actorType, Reason: reason,
		}); err != nil {
			return fmt.Errorf("persist tenant mfa override: %w", err)
		}
		// Force-off revokes this school's devices only: the same account may
		// legitimately keep trust at schools whose admin has not flipped it.
		if override == domain.MFAAdminOverrideForceOff {
			if err := f.records.RevokeTenantTrustedDevices(txCtx, targetAccountID, targetTenantID, f.now()); err != nil {
				return fmt.Errorf("revoke tenant-scoped trusted devices: %w", err)
			}
		}
		return nil
	})
	if txErr != nil {
		failure.ErrMsg = txErr.Error()
		f.recordAdminFailure(ctx, failure)
		return txErr
	}

	f.recordAuthEvent(ctx, domain.AuthEvent{
		AccountID: targetAccountID, TenantID: targetTenantID, Type: domain.AuthEventMFAAdminOverride,
		Success: true, IPAddress: domain.MFAAuditFallbackIP,
		MFA: &domain.MFAEvidence{Admin: &domain.MFAAdminEvidence{
			ActorType: actorType, ActorAccountID: actorID, Action: "set_override", Written: true,
			Scope: "tenant", Override: override, PreviousOverride: previous, Reason: reason,
		}},
	})
	return nil
}

// OperatorSetGlobalMFAOverride writes or clears the platform-wide row: the
// account-wide emergency switch for a user who lost mailbox access at every
// school they belong to. School admins cannot reach it. A force_off here is
// the only override path that revokes trust across every school.
func (f *AccountMFAFlows) OperatorSetGlobalMFAOverride(ctx context.Context, operatorID, targetAccountID int64, override, reason string) error {
	failure := adminOverrideFailure{
		ActorType: domain.MFAOverrideSetByTypeOperator, ActorID: operatorID,
		TargetAccountID: targetAccountID, Action: "set_global_override", Override: override, Reason: reason,
	}
	if !domain.IsValidMFAAdminOverride(override) {
		failure.ErrMsg = domain.ErrMFAInvalidOverride.Error()
		f.recordAdminFailure(ctx, failure)
		return domain.ErrMFAInvalidOverride
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		failure.Reason, failure.ErrMsg = "", domain.ErrMFAGlobalReasonRequired.Error()
		f.recordAdminFailure(ctx, failure)
		return domain.ErrMFAGlobalReasonRequired
	}
	failure.Reason = reason

	previous := domain.MFAAdminOverrideNone
	txErr := f.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		if err := f.records.LockAccountForOverrideWrite(txCtx, targetAccountID); err != nil {
			return err
		}
		existing, found, err := f.records.FindGlobalOverride(txCtx, targetAccountID)
		if err != nil {
			return fmt.Errorf("load existing global override: %w", err)
		}
		if found {
			previous = existing.Override
		}
		if override == domain.MFAAdminOverrideNone {
			if err := f.records.DeleteGlobalOverride(txCtx, targetAccountID); err != nil {
				return fmt.Errorf("clear global mfa override: %w", err)
			}
			return nil
		}
		if err := f.records.UpsertGlobalOverride(txCtx, domain.AccountMFAOverride{
			AccountID: targetAccountID, Override: override, SetBy: operatorID,
			SetByType: domain.MFAOverrideSetByTypeOperator, Reason: reason,
		}); err != nil {
			return fmt.Errorf("persist global mfa override: %w", err)
		}
		if override == domain.MFAAdminOverrideForceOff {
			if err := f.records.RevokeAllTrustedDevices(txCtx, targetAccountID, f.now()); err != nil {
				return fmt.Errorf("revoke trusted devices: %w", err)
			}
		}
		return nil
	})
	if txErr != nil {
		failure.ErrMsg = txErr.Error()
		f.recordAdminFailure(ctx, failure)
		return txErr
	}

	// An account-wide override is a platform action with no single school.
	// The authentication ledger is school-scoped and would drop the row, so
	// this one goes to the operator action log instead — the same table the
	// operator's own MFA actions use.
	target := targetAccountID
	f.audit.RecordOperatorAction(domain.OperatorAuditEntry{
		OperatorID: operatorID, Action: domain.OperatorAuditActionMFAAdminOverride,
		ResourceType: domain.OperatorAuditResourceAccount, ResourceID: &target,
		MFA: &domain.OperatorMFAEvidence{Override: &domain.OperatorMFAOverrideEvidence{
			Action: "set_global_override", Scope: "platform", Override: override,
			PreviousOverride: previous, Reason: reason,
		}},
	})
	return nil
}

// GetTenantMFAOverride returns the school-scoped value for the pair, or
// "none". The operator's platform-wide row stays hidden on this path so a
// school admin cannot learn whether the emergency switch is open.
func (f *AccountMFAFlows) GetTenantMFAOverride(ctx context.Context, accountID, tenantID int64) (string, error) {
	if tenantID == 0 {
		return domain.MFAAdminOverrideNone, nil
	}
	override, found, err := f.records.FindTenantOverride(ctx, accountID, tenantID)
	if err != nil {
		return "", fmt.Errorf("load tenant mfa override: %w", err)
	}
	if !found {
		return domain.MFAAdminOverrideNone, nil
	}
	return override.Override, nil
}

// GetGlobalMFAOverride returns the platform-wide value, or "none". Operator
// surface only.
func (f *AccountMFAFlows) GetGlobalMFAOverride(ctx context.Context, accountID int64) (string, error) {
	override, found, err := f.records.FindGlobalOverride(ctx, accountID)
	if err != nil {
		return "", fmt.Errorf("load global mfa override: %w", err)
	}
	if !found {
		return domain.MFAAdminOverrideNone, nil
	}
	return override.Override, nil
}

// GetAdminState is the read the admin modal needs, behind the same two gates
// the writes pass. Without them the read path would be the soft underbelly:
// a school admin with users:manage could enumerate which accounts elsewhere
// are enrolled and which carry which override by probing integer ids. A
// cross-school probe is indistinguishable from "no such account" and lands a
// failure row.
func (f *AccountMFAFlows) GetAdminState(ctx context.Context, actorID, actorTenantID, targetAccountID int64, actorPermissions []string) (domain.MFAAdminState, error) {
	if err := f.requireAdminPermission(ctx, actorPermissions, adminOverrideFailure{
		ActorType: actorTypeAccount, ActorID: actorID, TargetTenantID: actorTenantID,
		TargetAccountID: targetAccountID, Action: "get_state",
	}); err != nil {
		return domain.MFAAdminState{}, err
	}
	if err := f.requireSchoolMembership(ctx, actorTypeAccount, actorID, actorTenantID, targetAccountID, "get_state"); err != nil {
		return domain.MFAAdminState{}, err
	}
	enrolled, err := f.HasEnrollment(ctx, targetAccountID)
	if err != nil {
		return domain.MFAAdminState{}, err
	}
	override, err := f.GetTenantMFAOverride(ctx, targetAccountID, actorTenantID)
	if err != nil {
		return domain.MFAAdminState{}, err
	}
	return domain.MFAAdminState{Enrolled: enrolled, Override: override}, nil
}

// ===== Gates and audit =====

func (f *AccountMFAFlows) requireAdminPermission(ctx context.Context, actorPermissions []string, failure adminOverrideFailure) error {
	if domain.HasPermissionMatch(actorPermissions, adminPermission) {
		return nil
	}
	failure.ErrMsg = "permission denied"
	f.recordAdminFailure(ctx, failure)
	return domain.ErrMFAPermissionDenied
}

// requireSchoolMembership mirrors the route-level check inside the flow, so
// a CLI, a scheduler or any other future caller cannot reach across schools.
func (f *AccountMFAFlows) requireSchoolMembership(ctx context.Context, actorType string, actorID, schoolID, targetAccountID int64, action string) error {
	failure := adminOverrideFailure{
		ActorType: actorType, ActorID: actorID, TargetAccountID: targetAccountID, Action: action,
	}
	if schoolID == 0 {
		failure.ErrMsg = "missing school_id"
		f.recordAdminFailure(ctx, failure)
		return domain.ErrMFAPermissionDenied
	}
	exists, err := f.records.AccountBelongsToTenant(ctx, targetAccountID, schoolID)
	if err != nil {
		failure.ErrMsg = "membership lookup failed: " + err.Error()
		f.recordAdminFailure(ctx, failure)
		return fmt.Errorf("verify account membership: %w", err)
	}
	if !exists {
		failure.ErrMsg = "account is not a member of school"
		f.recordAdminFailure(ctx, failure)
		return domain.ErrMFAPermissionDenied
	}
	return nil
}

// adminOverrideFailure is the variable shape of a rejected admin attempt.
type adminOverrideFailure struct {
	ActorType       string
	ActorID         int64
	TargetAccountID int64
	// TargetTenantID is the school the row is filed under. The operator
	// paths run outside the tenant middleware, where the row would otherwise
	// be dropped.
	TargetTenantID int64
	Action         string
	Override       string
	Reason         string
	ErrMsg         string
}

func (f *AccountMFAFlows) recordAdminFailure(ctx context.Context, failure adminOverrideFailure) {
	f.recordAuthEvent(ctx, domain.AuthEvent{
		AccountID: failure.TargetAccountID, TenantID: failure.TargetTenantID,
		Type: domain.AuthEventMFAAdminOverride, IPAddress: domain.MFAAuditFallbackIP,
		ErrorMessage: failure.ErrMsg,
		MFA: &domain.MFAEvidence{Admin: &domain.MFAAdminEvidence{
			ActorType: failure.ActorType, ActorAccountID: failure.ActorID,
			Action: failure.Action, Override: failure.Override, Reason: failure.Reason,
		}},
	})
}
