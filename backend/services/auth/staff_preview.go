package auth

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// StaffPreviewSession is the result of starting an admin staff-view preview
// (#2893): a short-lived, access-only JWT that carries the TARGET account's
// identity, roles, and permissions plus the read_only/acting_admin_id claims.
// No refresh token is ever minted in the target's name — the preview lives
// only as long as the admin session that keeps re-minting it.
type StaffPreviewSession struct {
	AccessToken     string
	ExpiresIn       int64 // seconds until the access token expires
	TargetAccountID int64
	TargetName      string
}

// StaffPreviewCandidate is one selectable staff member for the preview
// picker: an active account with an active mapping and at least one
// tenant-portal role at the admin's school.
type StaffPreviewCandidate struct {
	AccountID int64    `json:"account_id"`
	FirstName string   `json:"first_name"`
	LastName  string   `json:"last_name"`
	Email     string   `json:"email"`
	Roles     []string `json:"roles"`
}

// StartStaffPreview mints a read-only preview token for targetAccountID at
// tenantID (#2893). The route layer restricts callers to effective admins;
// this method validates the TARGET: active account, active mapping at the
// admin's school, and a tenant-portal surface (not guardian-only, not
// Lehrkraft-only). The audit event is written against the ADMIN's account
// with the target in the metadata — never in the target's name.
//
// Re-minting is the same call: the frontend repeats it with a fresh admin
// token when the preview token nears expiry, so a deactivated target ends
// the preview at the next mint.
func (s *Service) StartStaffPreview(ctx context.Context, adminAccountID, tenantID, targetAccountID int64, ipAddress, userAgent string) (*StaffPreviewSession, error) {
	const op = "start staff preview"

	if tenantID <= 0 {
		return nil, &AuthError{Op: op, Err: ErrTenantNotFound}
	}
	if targetAccountID == adminAccountID {
		return nil, &AuthError{Op: op, Err: ErrPreviewSelf}
	}

	account, err := s.repos.Account.FindByID(ctx, targetAccountID)
	if err != nil {
		// Only a genuine miss is a 404. A database or infrastructure error
		// must stay a 5xx so the admin retries instead of being told the
		// colleague does not exist.
		if errors.Is(err, modelBase.ErrNotFound) {
			return nil, &AuthError{Op: op, Err: ErrAccountNotFound}
		}
		return nil, &AuthError{Op: op, Err: err}
	}
	if account == nil {
		return nil, &AuthError{Op: op, Err: ErrAccountNotFound}
	}
	if !account.Active {
		return nil, &AuthError{Op: op, Err: ErrAccountInactive}
	}

	// Membership check + tenant-scoped metadata in ONE admin transaction —
	// the same BYPASSRLS shape as the login/switch flows, because no tenant
	// transaction is open on this route (mirrors switch-tenant).
	var metadata *accountMetadata
	err = tenant.WithAdminTx(s.withTenantRuntime(ctx), s.db, func(ctx context.Context, _ bun.Tx) error {
		mappings, err := s.repos.AccountTenant.FindActiveByAccountID(ctx, targetAccountID)
		if err != nil {
			return &AuthError{Op: op, Err: err}
		}
		mapped := false
		for _, mapping := range mappings {
			if mapping.TenantID == tenantID {
				mapped = true
				break
			}
		}
		if !mapped {
			return &AuthError{Op: op, Err: ErrTenantAccessDenied}
		}

		metadata, err = s.loadAccountMetadataForTenantInTx(ctx, account, tenantID)
		return err
	})
	if err != nil {
		return nil, err
	}

	// The preview shows the OGS tenant portal. Accounts without a surface
	// there cannot be previewed meaningfully: guardians live in the parents
	// portal, Lehrkraft-only accounts in moto schule, and an account with no
	// role at this school could not even log in.
	if len(metadata.roleNames) == 0 || IsGuardianOnlyForTenant(metadata.roleNames) {
		return nil, &AuthError{Op: op, Err: ErrPreviewTargetNotStaff}
	}
	if IsSchoolPortalOnlyForTenant(account.Roles) {
		return nil, &AuthError{Op: op, Err: ErrMustUseSchoolPortal}
	}

	claims := jwt.AppClaims{
		ID:            int(account.ID),
		Sub:           account.Email,
		Username:      metadata.username,
		FirstName:     metadata.firstName,
		LastName:      metadata.lastName,
		Roles:         metadata.roleNames,
		Permissions:   metadata.permissionStrs,
		IsAdmin:       metadata.isAdmin,
		Scope:         metadata.scope,
		TenantID:      tenantID,
		OrgID:         metadata.orgID,
		ReadOnly:      true,
		ActingAdminID: adminAccountID,
	}
	accessToken, err := s.tokenAuth.CreateJWT(claims)
	if err != nil {
		return nil, &AuthError{Op: op, Err: err}
	}

	s.logAuthEventWithMetadata(ctx, adminAccountID, audit.EventTypeStaffPreviewStarted, true, ipAddress, userAgent, "",
		map[string]interface{}{"target_account_id": targetAccountID})
	s.getLogger().Info("staff preview started",
		slog.Int64("admin_account_id", adminAccountID),
		slog.Int64("target_account_id", targetAccountID),
		slog.Int64("tenant_id", tenantID),
	)

	return &StaffPreviewSession{
		AccessToken:     accessToken,
		ExpiresIn:       int64(s.tokenAuth.JwtExpiry.Seconds()),
		TargetAccountID: targetAccountID,
		TargetName:      staffPreviewDisplayName(metadata, account.Email),
	}, nil
}

// EndStaffPreview records that the admin left the preview. Called with the
// RESTORED admin session — the preview token itself cannot reach this route
// (POST, blocked by the read-only middleware), so it travels in the body as
// PROOF instead: the previewed account is read from the signed token, never
// from a client-supplied number. Only a token this admin was actually handed
// at this school is accepted, so nobody can stamp the audit trail with a
// preview that never happened. Purely an audit affair otherwise — the preview
// token simply expires, nothing is revoked.
//
// Returns the previewed account id for the caller's response and logs.
func (s *Service) EndStaffPreview(ctx context.Context, adminAccountID, tenantID int64, previewToken, ipAddress, userAgent string) (int64, error) {
	const op = "end staff preview"

	claims, err := s.tokenAuth.ParseAccessJWT(previewToken)
	if err != nil {
		s.getLogger().Warn("staff preview end: token not parseable",
			slog.Int64("admin_account_id", adminAccountID),
			slog.String("error", err.Error()),
		)
		return 0, &AuthError{Op: op, Err: ErrPreviewTokenInvalid}
	}
	if !claims.IsReadOnlyPreview() || claims.ActingAdminID != adminAccountID ||
		claims.TenantID != tenantID || claims.ID <= 0 {
		s.getLogger().Warn("staff preview end: token does not belong to this session",
			slog.Int64("admin_account_id", adminAccountID),
			slog.Int64("tenant_id", tenantID),
		)
		return 0, &AuthError{Op: op, Err: ErrPreviewTokenInvalid}
	}
	targetAccountID := int64(claims.ID)

	s.logAuthEventWithMetadata(ctx, adminAccountID, audit.EventTypeStaffPreviewEnded, true, ipAddress, userAgent, "",
		map[string]interface{}{"target_account_id": targetAccountID})
	s.getLogger().Info("staff preview ended",
		slog.Int64("admin_account_id", adminAccountID),
		slog.Int64("target_account_id", targetAccountID),
	)
	return targetAccountID, nil
}

// ListStaffPreviewCandidates returns the staff members an admin can preview
// at tenantID, excluding the caller. Runs inside the route's tenant
// transaction (RLS-scoped).
//
// The listing query returns aggregated role NAMES, and a name alone cannot
// tell the lehrkraft SYSTEM role from a school's own custom role carrying the
// same label. So the name check is only a prefilter: a candidate it would drop
// gets its actual role assignments loaded and decided by the same
// IsSchoolPortalOnlyForTenant the start path uses — the picker and
// StartStaffPreview never disagree.
func (s *Service) ListStaffPreviewCandidates(ctx context.Context, tenantID, excludeAccountID int64) ([]StaffPreviewCandidate, error) {
	infos, err := s.repos.AccountTenant.ListAccountsByTenantID(ctx, tenantID)
	if err != nil {
		return nil, &AuthError{Op: "list staff preview candidates", Err: err}
	}

	candidates := make([]StaffPreviewCandidate, 0, len(infos))
	for _, info := range infos {
		if info.AccountID == 0 || info.AccountID == excludeAccountID {
			continue
		}
		if !info.Active || info.Status != authModels.AccountTenantStatusActive {
			continue
		}
		roles := splitAggregatedRoleNames(info.RoleName)
		if len(roles) == 0 || IsGuardianOnlyForTenant(roles) {
			continue
		}
		if isLehrkraftOnlyByName(roles) {
			schoolPortalOnly, err := s.isSchoolPortalOnlyAtTenant(ctx, info.AccountID, tenantID)
			if err != nil {
				return nil, &AuthError{Op: "list staff preview candidates", Err: err}
			}
			if schoolPortalOnly {
				continue
			}
		}
		candidates = append(candidates, StaffPreviewCandidate{
			AccountID: info.AccountID,
			FirstName: info.FirstName,
			LastName:  info.LastName,
			Email:     info.Email,
			Roles:     roles,
		})
	}
	return candidates, nil
}

func splitAggregatedRoleNames(aggregated string) []string {
	if strings.TrimSpace(aggregated) == "" {
		return nil
	}
	parts := strings.Split(aggregated, ",")
	names := make([]string, 0, len(parts))
	for _, part := range parts {
		if name := strings.TrimSpace(part); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// isSchoolPortalOnlyAtTenant answers the start path's question for a listing
// row: does this account hold ONLY school-portal roles at the tenant? Loads
// the assignments with their role objects (the listing query carries names
// only) and decides with IsSchoolPortalOnlyForTenant. An account with no
// assignment row is not school-portal-only — the caller already knows it has
// roles, so that case belongs to the start call, not to a silent drop.
func (s *Service) isSchoolPortalOnlyAtTenant(ctx context.Context, accountID, tenantID int64) (bool, error) {
	accountRoles, err := s.repos.AccountRole.FindByAccountIDForTenant(ctx, accountID, tenantID)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	roles := make([]*authModels.Role, 0, len(accountRoles))
	for _, accountRole := range accountRoles {
		if accountRole.Role != nil {
			roles = append(roles, accountRole.Role)
		}
	}
	return IsSchoolPortalOnlyForTenant(roles), nil
}

// isLehrkraftOnlyByName is the cheap name-based prefilter in front of
// isSchoolPortalOnlyAtTenant: it decides nothing on its own, it only marks
// the rows worth one extra role lookup.
func isLehrkraftOnlyByName(roleNames []string) bool {
	if len(roleNames) == 0 {
		return false
	}
	for _, name := range roleNames {
		if !strings.EqualFold(name, lehrkraftRoleName) {
			return false
		}
	}
	return true
}

func staffPreviewDisplayName(metadata *accountMetadata, email string) string {
	name := strings.TrimSpace(strings.TrimSpace(metadata.firstName) + " " + strings.TrimSpace(metadata.lastName))
	if name != "" {
		return name
	}
	if metadata.username != "" {
		return metadata.username
	}
	return email
}
