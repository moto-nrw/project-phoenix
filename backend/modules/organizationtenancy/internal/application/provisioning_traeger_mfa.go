package application

import (
	"context"
	"log/slog"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

// minResetReasonLength mirrors the operator's reset of a school account's
// second factor: the audit needs a reason someone can read.
const minResetReasonLength = 3

// ResetTraegerMemberMFA resets the second factor of a Träger-Büro member
// (#4651), the recovery for a person who is locked out or no longer trusts
// the factor. The membership is proven, the reset of Identity & Access
// participates and the operator audit is written in one administrative
// unit; without the audit nothing is reset. The factor belongs to the
// account, so the reset applies to every portal of the person.
func (p *Provisioning) ResetTraegerMemberMFA(ctx context.Context, input organizationtenancy.ResetTraegerMemberMFA) error {
	reason := strings.TrimSpace(input.Reason)
	if len([]rune(reason)) < minResetReasonLength {
		return invalidData("reason must be at least 3 characters")
	}
	_, err := adminValue(ctx, p, func(adminCtx context.Context) (struct{}, error) {
		return struct{}{}, p.resetTraegerMemberMFA(adminCtx, input, reason)
	})
	if err != nil {
		return err
	}
	p.logger.Info("traeger_member_mfa_reset",
		slog.Int64("organization_id", input.OrganizationID),
		slog.Int64("account_id", input.AccountID),
		slog.Int64("operator_id", input.OperatorID))
	return nil
}

// resetTraegerMemberMFA runs the reset on the administrative unit: the
// membership, the participating reset and the operator audit.
func (p *Provisioning) resetTraegerMemberMFA(adminCtx context.Context, input organizationtenancy.ResetTraegerMemberMFA, reason string) error {
	memberships, err := p.organizations.ListTraegerMemberships(adminCtx, input.OrganizationID)
	if err != nil {
		return mapOrganizationError(err, input.OrganizationID)
	}
	if !slices.ContainsFunc(memberships, func(membership organizationtenancy.TraegerMembership) bool {
		return membership.AccountID == input.AccountID
	}) {
		return organizationtenancy.ErrTraegerMemberNotFound
	}
	if err := p.identity.ResetTraegerMFA(adminCtx, input.AccountID); err != nil {
		return err
	}
	accountID := input.AccountID
	return p.recordAction(adminCtx, input.OperatorID, domain.AuditActionMFAReset, domain.AuditResourceTraegerMember, &accountID, input.ClientIP, map[string]any{
		"organization_id": input.OrganizationID,
		"account_id":      input.AccountID,
		"reason":          reason,
	})
}
