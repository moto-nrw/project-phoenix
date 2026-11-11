package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Parent session at a Träger (ADR 0216 point 7). An account set up through
// the link of a holiday registration has no school, no guardian profile and
// no child; it holds parents-portal access at the Träger
// (auth.traeger_parent_access). The parent sign-in opens its session only
// when no guardian school that books Eltern-App opens the ordinary one. The
// session names the Träger and no school, like the Träger session (ADR 0084),
// carries no rights, follows no school's module booking and no second factor.
// Every mint, refresh and validation reads the access and the Träger again;
// without either the session ends.

var traegerParentPolicy = sessionPolicy{
	before: func(_ context.Context, s *AccountAuthentication, b sessionBinding) error {
		if b.tenantID != 0 || b.orgID <= 0 {
			return b.refuse(s, "traeger_parent_token_shape", failed("refresh Träger parent session", identityaccess.ErrInvalidToken))
		}
		return nil
	},
	facts: func(ctx context.Context, s *AccountAuthentication, b sessionBinding, r factReads) (heldFacts, error) {
		return heldFacts{}, s.requireTraegerParentAccess(ctx, b, r)
	},
	claims: func(_ context.Context, _ *AccountAuthentication, b sessionBinding, account domain.LoginAccount, _ heldFacts) (*domain.AccountClaimsPayload, error) {
		claims := buildParentMetadata(account)
		// The access token names the Träger: validation recognises the
		// kind by it (domain.SessionClaims.SessionKind).
		claims.OrgID = b.orgID
		return claims, nil
	},
	keptAt: func(*AccountAuthentication, sessionBinding) successorTenant {
		return func(context.Context, int64, domain.AccountSession) (int64, error) { return 0, nil }
	},
}

// requireTraegerParentAccess: the account holds parents-portal access at the
// Träger, and the Träger is active and not deleted. Mint and refresh pin
// both rows in shared mode after the account lock: the access first, then
// the Träger, so neither a removal nor a deactivation commits under them. A
// renewal answers their loss with ErrTenantNotFound, as a parent session
// whose guardian school is gone.
func (s *AccountAuthentication) requireTraegerParentAccess(ctx context.Context, b sessionBinding, r factReads) error {
	if s.traeger == nil {
		return errTraegerMembershipsUnavailable
	}
	held, _, err := s.store.HasTraegerParentAccess(ctx, b.accountID, b.orgID, r.locked)
	if err != nil {
		return fmt.Errorf("re-check parents-portal access of account %d at Träger %d: %w", b.accountID, b.orgID, err)
	}
	live := false
	if held {
		live, err = s.liveTraeger(ctx, b.orgID, r)
		if err != nil {
			return fmt.Errorf("re-check Träger %d of a parent session: %w", b.orgID, err)
		}
	}
	if live {
		return nil
	}
	if b.renewal == mintSession {
		return failed(b.op, identityaccess.ErrAccountNoGuardianRole)
	}
	return b.refuse(s, "traeger_parent_access_lost", failed("refresh Träger parent session", identityaccess.ErrTenantNotFound))
}

func (s *AccountAuthentication) liveTraeger(ctx context.Context, organizationID int64, r factReads) (bool, error) {
	live, err := s.traeger.LiveTraegers(ctx, []int64{organizationID}, r.locked)
	return len(live) > 0, err
}

// traegerParentOrganizations returns the active Träger the account holds
// parents-portal access at, oldest access first, read inside an
// administrative transaction.
func (s *AccountAuthentication) traegerParentOrganizations(ctx context.Context, accountID int64) ([]identityaccess.TraegerOrganization, error) {
	if s.traeger == nil {
		return nil, nil
	}
	var organizations []identityaccess.TraegerOrganization
	err := globalIdentityRead(ctx, s.runtime, func(txCtx context.Context) error {
		organizationIDs, _, err := s.store.ListTraegerParentAccess(txCtx, accountID)
		if err != nil || len(organizationIDs) == 0 {
			return err
		}
		organizations, err = s.traeger.LiveTraegers(txCtx, organizationIDs, false)
		return err
	})
	return organizations, err
}

// issueTraegerParentTokens mints the parent session at the first active
// Träger the account holds access at; found is false when there is none.
func (s *AccountAuthentication) issueTraegerParentTokens(ctx context.Context, account domain.LoginAccount, email, ipAddress, userAgent string) (string, string, bool, error) {
	organizations, err := s.traegerParentOrganizations(ctx, account.ID)
	if err != nil {
		return "", "", false, failed("parent login: read Träger access", err)
	}
	if len(organizations) == 0 {
		return "", "", false, nil
	}
	minted, err := s.mintAccountSession(ctx, mintRequest{
		portal: domain.PortalParent, account: account, email: email, orgID: organizations[0].ID,
		op:    "parent login",
		audit: s.schoollessAudit(ctx, account.ID, domain.AuthEventLogin, ipAddress, userAgent),
	})
	return minted.accessToken, minted.refreshToken, true, err
}

// hasTraegerParentAccess reports whether the parents portal serves the
// account at an active Träger; the password reset sends it the parents link.
func (s *AccountAuthentication) hasTraegerParentAccess(ctx context.Context, accountID int64) (bool, error) {
	organizations, err := s.traegerParentOrganizations(ctx, accountID)
	return len(organizations) > 0, err
}

// schoollessAudit files the event of a session without a school at the
// account's default school; the authentication ledger is school-scoped, and
// an account set up for a holiday registration may have none.
func (s *AccountAuthentication) schoollessAudit(ctx context.Context, accountID int64, event, ipAddress, userAgent string) *mintAudit {
	if _, _, err := s.resolveAccountTenantDefault(ctx, accountID); err != nil {
		s.logger.Info("parent session at a Träger without a school ledger",
			slog.Int64("account_id", accountID),
			slog.String("event_type", event))
		return nil
	}
	return &mintAudit{event: event, ipAddress: ipAddress, userAgent: userAgent}
}
