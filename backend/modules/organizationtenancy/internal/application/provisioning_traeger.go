package application

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

// The operator assigns the first person of a Träger-Büro (ADR 0084, spec
// #4611 user stories 1 and 2). The membership belongs to this owner, the
// Träger role to Identity & Access; both commit in one administrative unit,
// the role command participating in it (ADR 0067).

// ListTraegerMembers lists the people of the Träger-Büro with their roles.
func (p *Provisioning) ListTraegerMembers(ctx context.Context, organizationID int64) ([]organizationtenancy.TraegerMember, error) {
	return adminValue(ctx, p, func(adminCtx context.Context) ([]organizationtenancy.TraegerMember, error) {
		memberships, err := p.organizations.ListTraegerMemberships(adminCtx, organizationID)
		if err != nil {
			return nil, mapOrganizationError(err, organizationID)
		}
		accountIDs := make([]int64, 0, len(memberships))
		for _, membership := range memberships {
			accountIDs = append(accountIDs, membership.AccountID)
		}
		accounts, err := p.identity.ListTraegerAccounts(adminCtx, organizationID, accountIDs)
		if err != nil {
			return nil, err
		}
		return traegerMembers(memberships, accounts), nil
	})
}

// AssignTraegerMember activates the membership of an existing account and
// grants it one or more Träger roles (#5144). The operator audit records the
// grant in the same unit; without the audit nothing is granted.
func (p *Provisioning) AssignTraegerMember(ctx context.Context, input organizationtenancy.AssignTraegerMember) (organizationtenancy.TraegerMember, error) {
	email := strings.TrimSpace(strings.ToLower(input.Email))
	if email == "" {
		return organizationtenancy.TraegerMember{}, invalidData("email is required")
	}
	roles, err := traegerRoleSet(input.Roles)
	if err != nil {
		return organizationtenancy.TraegerMember{}, err
	}
	input.Roles = roles
	member, err := adminValue(ctx, p, func(adminCtx context.Context) (organizationtenancy.TraegerMember, error) {
		return p.assignTraegerMember(adminCtx, input, email)
	})
	if err != nil {
		return organizationtenancy.TraegerMember{}, err
	}
	p.logger.Info("traeger_member_assigned",
		slog.Int64("organization_id", input.OrganizationID),
		slog.Int64("account_id", member.AccountID),
		slog.Int64("operator_id", input.OperatorID))
	return member, nil
}

// assignTraegerMember runs the grant on the administrative unit: the account
// behind the address, the role, the membership and the operator audit.
func (p *Provisioning) assignTraegerMember(adminCtx context.Context, input organizationtenancy.AssignTraegerMember, email string) (organizationtenancy.TraegerMember, error) {
	accountID, found, err := p.identity.FindAccountIDByEmail(adminCtx, email)
	if err != nil {
		return organizationtenancy.TraegerMember{}, err
	}
	if !found {
		return organizationtenancy.TraegerMember{}, organizationtenancy.ErrTraegerAccountNotFound
	}
	if _, err := p.organizations.FindOrganization(adminCtx, input.OrganizationID); err != nil {
		return organizationtenancy.TraegerMember{}, mapOrganizationError(err, input.OrganizationID)
	}
	// The role grant locks the live account first, before the membership
	// insert takes its foreign-key lock: two grants of the same account then
	// queue on that lock instead of deadlocking on each other's key share.
	for _, role := range input.Roles {
		if err := p.identity.AssignTraegerRole(adminCtx, input.OrganizationID, accountID, role); err != nil {
			if errors.Is(err, organizationtenancy.ErrInvalidTraegerRole) {
				return organizationtenancy.TraegerMember{}, &organizationtenancy.InvalidProvisioningDataError{Err: err}
			}
			return organizationtenancy.TraegerMember{}, err
		}
	}
	membership, err := p.organizations.ActivateTraegerMembership(adminCtx, input.OrganizationID, accountID)
	if err != nil {
		return organizationtenancy.TraegerMember{}, mapOrganizationError(err, input.OrganizationID)
	}
	if err := p.recordAction(adminCtx, input.OperatorID, domain.AuditActionGrant, domain.AuditResourceTraegerMember, &accountID, input.ClientIP, map[string]any{
		"organization_id": input.OrganizationID,
		"account_id":      accountID,
		"roles":           input.Roles,
	}); err != nil {
		return organizationtenancy.TraegerMember{}, err
	}
	accounts, err := p.identity.ListTraegerAccounts(adminCtx, input.OrganizationID, []int64{accountID})
	if err != nil {
		return organizationtenancy.TraegerMember{}, err
	}
	members := traegerMembers([]organizationtenancy.TraegerMembership{membership}, accounts)
	return members[0], nil
}

// InviteTraegerMember invites a person into the Träger-Büro with the
// Träger-Büro's own invitation (#5144): Identity & Access stores it and
// queues the mail, the operator audit records it in the same unit. Without
// the audit nothing is sent.
func (p *Provisioning) InviteTraegerMember(ctx context.Context, input organizationtenancy.InviteTraegerMember) (organizationtenancy.TraegerInvitation, error) {
	email := strings.TrimSpace(strings.ToLower(input.Email))
	if email == "" {
		return organizationtenancy.TraegerInvitation{}, invalidData("email is required")
	}
	roles, err := traegerRoleSet(input.Roles)
	if err != nil {
		return organizationtenancy.TraegerInvitation{}, err
	}
	invitation, err := adminValue(ctx, p, func(adminCtx context.Context) (organizationtenancy.TraegerInvitation, error) {
		organization, err := p.organizations.FindOrganization(adminCtx, input.OrganizationID)
		if err != nil {
			return organizationtenancy.TraegerInvitation{}, mapOrganizationError(err, input.OrganizationID)
		}
		if !organization.Active || organization.IsDeleted() {
			return organizationtenancy.TraegerInvitation{}, organizationtenancy.ErrOrganizationInactive
		}
		invitation, err := p.identity.InviteTraegerAccount(adminCtx, input.OrganizationID, email, roles)
		if err != nil {
			return organizationtenancy.TraegerInvitation{}, traegerInvitationError(err)
		}
		return invitation, p.recordAction(adminCtx, input.OperatorID, domain.AuditActionCreate, domain.AuditResourceTraegerInvitation, &invitation.ID, input.ClientIP, map[string]any{
			"organization_id": input.OrganizationID,
			"email":           invitation.Email,
			"roles":           invitation.Roles,
		})
	})
	if err != nil {
		return organizationtenancy.TraegerInvitation{}, err
	}
	p.logger.Info("traeger_member_invited",
		slog.Int64("organization_id", input.OrganizationID),
		slog.Int64("invitation_id", invitation.ID),
		slog.Int64("operator_id", input.OperatorID))
	return invitation, nil
}

// traegerRoleSet deduplicates the requested roles and refuses an empty set;
// Identity & Access validates each role.
func traegerRoleSet(roles []string) ([]string, error) {
	result := make([]string, 0, len(roles))
	for _, role := range roles {
		role = strings.TrimSpace(role)
		if role != "" && !slices.Contains(result, role) {
			result = append(result, role)
		}
	}
	if len(result) == 0 {
		return nil, &organizationtenancy.InvalidProvisioningDataError{Err: organizationtenancy.ErrInvalidTraegerRole}
	}
	return result, nil
}

// traegerInvitationError turns the invitation's input refusals into invalid
// operator input; the other refusals keep their sentinel.
func traegerInvitationError(err error) error {
	if errors.Is(err, organizationtenancy.ErrInvalidTraegerRole) {
		return &organizationtenancy.InvalidProvisioningDataError{Err: err}
	}
	return err
}

// traegerMembers joins the memberships with the identity facts in membership
// order. A membership whose account Identity no longer reports keeps its row
// without an address.
func traegerMembers(memberships []organizationtenancy.TraegerMembership, accounts []organizationtenancy.TraegerAccount) []organizationtenancy.TraegerMember {
	byID := make(map[int64]organizationtenancy.TraegerAccount, len(accounts))
	for _, account := range accounts {
		byID[account.AccountID] = account
	}
	members := make([]organizationtenancy.TraegerMember, 0, len(memberships))
	for _, membership := range memberships {
		account := byID[membership.AccountID]
		roles := account.Roles
		if roles == nil {
			roles = []string{}
		}
		members = append(members, organizationtenancy.TraegerMember{
			AccountID: membership.AccountID, Email: account.Email, Status: membership.Status,
			Roles: roles, ActivatedAt: membership.ActivatedAt,
		})
	}
	return members
}
