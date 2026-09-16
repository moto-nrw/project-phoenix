package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Admin staff-view preview (#2893): a read-only, access-only token that sees
// the tenant portal exactly as the target staff member. Start mints (and
// re-mints) the token, End records the audit trail, the candidate list feeds
// the picker. The route layer restricts all three to effective admins.

// StartStaffPreview mints a read-only preview token for targetAccountID at
// tenantID. It validates the TARGET: active account, active mapping at the
// admin's school, and a tenant-portal surface (not guardian-only, not
// Lehrkraft-only). The audit event is written against the ADMIN's account
// with the target in the metadata, never in the target's name.
//
// Re-minting is the same call: the frontend repeats it with a fresh admin
// token when the preview token nears expiry. It passes the token it currently
// holds as previousToken; when that token proves the SAME preview instance,
// the new token inherits its preview id and NO second started event is
// written.
func (l *AccountLifecycle) StartStaffPreview(ctx context.Context, adminAccountID, tenantID, targetAccountID int64, previousToken, ipAddress, userAgent string) (*domain.StaffPreviewSession, error) {
	const op = "start staff preview"

	if tenantID <= 0 {
		return nil, failed(op, domain.ErrTenantNotFound)
	}
	if targetAccountID == adminAccountID {
		return nil, failed(op, domain.ErrPreviewSelf)
	}

	// Everything that decides whether this person may be previewed and the
	// minting of the token happen in ONE admin transaction, with the same row
	// locks the login/refresh path uses. Under READ COMMITTED a plain read
	// would only prove the account was active at statement time. The FOR
	// UPDATE on the account and the FOR SHARE on the mapping make the
	// revoking UPDATE wait for this transaction, so a minted token is provably
	// backed by an active account, an active membership, and the roles and
	// permissions that still existed when the token was written.
	var (
		account     domain.LoginAccount
		claims      *domain.AccountClaimsPayload
		accessToken string
		previewID   string
		isRemint    bool
	)
	err := l.runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		var found bool
		var err error
		account, found, _, err = l.logins.FindLoginAccount(txCtx, targetAccountID, true)
		if err != nil {
			return failed(op, err)
		}
		if !found {
			// Only a genuine miss is a 404; a database failure stays a 5xx.
			return failed(op, domain.ErrAccountNotFound)
		}
		if !account.Active {
			return failed(op, domain.ErrAccountInactive)
		}

		mapped, _, err := l.logins.LockActiveTenantMappingShared(txCtx, targetAccountID, tenantID)
		if err != nil {
			return failed(op, err)
		}
		if !mapped {
			return failed(op, domain.ErrTenantAccessDenied)
		}

		// Pin what the token is BUILT from, in the same account -> roles ->
		// permissions order every revocation path walks, so the claims read
		// below are provably the ones still in force at commit time.
		if _, _, err := l.logins.ListAccountRolesAtTenant(txCtx, targetAccountID, tenantID, true); err != nil {
			return failed(op, err)
		}
		if _, err := l.logins.LockAccountPermissionSources(txCtx, targetAccountID, tenantID); err != nil {
			return failed(op, err)
		}

		claims, err = l.auth.LoadAccountClaims(txCtx, targetAccountID, tenantID)
		if err != nil {
			return err
		}

		// The preview shows the OGS tenant portal. Accounts without a surface
		// there cannot be previewed meaningfully: guardians live in the
		// parents portal, Lehrkraft-only accounts in moto schule, and an
		// account with no role at this school could not even log in.
		if len(claims.RoleNames) == 0 || domain.IsGuardianOnly(claims.RoleNames) {
			return failed(op, domain.ErrPreviewTargetNotStaff)
		}
		if domain.IsSchoolPortalOnly(claims.Roles) {
			return failed(op, domain.ErrMustUseSchoolPortal)
		}

		previewID, isRemint, err = l.continuedPreviewID(txCtx, previousToken, adminAccountID, tenantID, targetAccountID)
		if err != nil {
			return failed(op, err)
		}
		if !isRemint {
			newID, err := newPreviewID()
			if err != nil {
				return failed(op, err)
			}
			previewID = newID
		}

		accessToken, err = l.codec.IssueAccessToken(domain.SessionClaims{
			AccountID: account.ID, Email: account.Email, Username: claims.Username,
			FirstName: claims.FirstName, LastName: claims.LastName,
			Roles: claims.RoleNames, Permissions: claims.Permissions, IsAdmin: claims.IsAdmin,
			Scope: claims.Scope, TenantID: tenantID, OrgID: claims.OrgID,
			ReadOnly: true, ActingAdminID: adminAccountID, PreviewID: previewID,
		})
		if err != nil {
			return failed(op, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if isRemint {
		l.logger.Debug("staff preview token re-minted",
			slog.Int64("admin_account_id", adminAccountID),
			slog.Int64("target_account_id", targetAccountID),
			slog.Int64("tenant_id", tenantID),
		)
	} else {
		// Written synchronously, BEFORE the token leaves this method: if the
		// start row cannot land, the caller gets an error and never holds a
		// preview token. The mint transaction persisted nothing (a JWT is
		// stateless), so there is nothing to unwind on failure.
		if err := l.recordPreviewStart(ctx, domain.StaffPreviewEvent{
			AdminAccountID: adminAccountID, TenantID: tenantID, TargetAccountID: targetAccountID,
			PreviewID: previewID, IPAddress: ipAddress, UserAgent: userAgent,
		}); err != nil {
			return nil, failed(op, err)
		}
		l.logger.Info("staff preview started",
			slog.Int64("admin_account_id", adminAccountID),
			slog.Int64("target_account_id", targetAccountID),
			slog.Int64("tenant_id", tenantID),
		)
	}

	return &domain.StaffPreviewSession{
		AccessToken:     accessToken,
		ExpiresIn:       int64(l.codec.AccessExpiry().Seconds()),
		TargetAccountID: targetAccountID,
		TargetName:      staffPreviewDisplayName(claims, account.Email),
	}, nil
}

// EndStaffPreview records that the admin left the preview. The signed preview
// token IS the credential: admin, school, target and preview instance are all
// read from its claims, never from anything the client picks. Expiry is
// deliberately not checked: a preview left open past the access expiry must
// still end with an audit row. Ending is one-shot per preview instance, and
// the database is what makes it so (partial unique index over account and
// preview id for this event type).
//
// Returns the previewed account id for the caller's response and logs.
func (l *AccountLifecycle) EndStaffPreview(ctx context.Context, previewToken, ipAddress, userAgent string) (int64, error) {
	const op = "end staff preview"

	claims, err := l.codec.ParseAccessTokenAllowExpired(previewToken)
	if err != nil {
		l.logger.Warn("staff preview end: token not parseable",
			slog.String("error", err.Error()),
		)
		return 0, failed(op, domain.ErrPreviewTokenInvalid)
	}
	if !claims.ReadOnly || claims.ActingAdminID <= 0 ||
		claims.TenantID <= 0 || claims.AccountID <= 0 || claims.PreviewID == "" {
		l.logger.Warn("staff preview end: token is not a preview token")
		return 0, failed(op, domain.ErrPreviewTokenInvalid)
	}
	event := domain.StaffPreviewEvent{
		AdminAccountID: claims.ActingAdminID, TenantID: claims.TenantID, TargetAccountID: claims.AccountID,
		PreviewID: claims.PreviewID, IPAddress: ipAddress, UserAgent: userAgent,
	}

	recorded, err := l.recordPreviewEnd(ctx, event)
	if err != nil {
		return 0, failed(op, err)
	}
	if !recorded {
		// Not an error for the caller: the preview IS over. It simply does
		// not get a second audit row.
		l.logger.Debug("staff preview end: already recorded",
			slog.Int64("admin_account_id", event.AdminAccountID),
			slog.Int64("target_account_id", event.TargetAccountID),
		)
		return event.TargetAccountID, nil
	}

	l.logger.Info("staff preview ended",
		slog.Int64("admin_account_id", event.AdminAccountID),
		slog.Int64("target_account_id", event.TargetAccountID),
	)
	return event.TargetAccountID, nil
}

// continuedPreviewID reads the preview id out of the token the client is
// renewing. It returns ok only when the token is a signed preview token of
// exactly this admin, school, and target; a foreign or forged value must
// never suppress a start event. An expired token is fine. A token of a
// preview that has ALREADY ENDED is not: such a token starts a fresh preview
// with its own start event. "Already ended" is decided under the preview's
// advisory lock, which the caller's transaction keeps until the replacement
// token is minted and committed.
func (l *AccountLifecycle) continuedPreviewID(ctx context.Context, previousToken string, adminAccountID, tenantID, targetAccountID int64) (string, bool, error) {
	if strings.TrimSpace(previousToken) == "" {
		return "", false, nil
	}
	claims, err := l.codec.ParseAccessTokenAllowExpired(previousToken)
	if err != nil {
		return "", false, nil
	}
	if !claims.ReadOnly || claims.PreviewID == "" ||
		claims.ActingAdminID != adminAccountID || claims.TenantID != tenantID ||
		claims.AccountID != targetAccountID {
		return "", false, nil
	}

	if err := l.audit.LockStaffPreview(ctx, adminAccountID, claims.PreviewID); err != nil {
		return "", false, err
	}
	ended, err := l.audit.StaffPreviewEnded(ctx, adminAccountID, claims.PreviewID)
	if err != nil {
		return "", false, err
	}
	if ended {
		return "", false, nil
	}
	return claims.PreviewID, true, nil
}

// recordPreviewStart writes the "preview started" audit row for a NEW
// preview instance, synchronously, in the admin's tenant transaction (the
// start route runs without one).
func (l *AccountLifecycle) recordPreviewStart(ctx context.Context, event domain.StaffPreviewEvent) error {
	return l.runtime.WithTenantTx(ctx, event.TenantID, func(txCtx context.Context) error {
		return l.audit.RecordStaffPreviewStart(txCtx, event)
	})
}

// recordPreviewEnd writes the "preview ended" audit row for this preview
// instance and reports whether THIS call wrote it. The advisory lock is taken
// before the insert so a renewal in flight either finishes before this end
// is visible, or waits for it and then sees a closed instance.
func (l *AccountLifecycle) recordPreviewEnd(ctx context.Context, event domain.StaffPreviewEvent) (bool, error) {
	var recorded bool
	err := l.runtime.WithTenantTx(ctx, event.TenantID, func(txCtx context.Context) error {
		if err := l.audit.LockStaffPreview(txCtx, event.AdminAccountID, event.PreviewID); err != nil {
			return err
		}
		inserted, err := l.audit.RecordStaffPreviewEndOnce(txCtx, event)
		if err != nil {
			return err
		}
		recorded = inserted
		return nil
	})
	return recorded, err
}

// newPreviewID returns the identifier for one preview instance: 128 bits of
// cryptographic randomness, so a client can never guess a foreign id.
func newPreviewID() (string, error) {
	buf := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// ListStaffPreviewCandidates returns the staff members an admin can preview
// at tenantID, excluding the caller. Runs inside the route's tenant
// transaction (RLS-scoped).
//
// The listing carries aggregated role NAMES, and a name alone cannot tell the
// lehrkraft SYSTEM role from a school's own custom role carrying the same
// label. So the name check is only a prefilter: a candidate it would drop
// gets its actual role assignments loaded and decided by the same rule the
// start path uses, so the picker and StartStaffPreview never disagree.
func (l *AccountLifecycle) ListStaffPreviewCandidates(ctx context.Context, tenantID, excludeAccountID int64) ([]domain.StaffPreviewCandidate, error) {
	const op = "list staff preview candidates"
	accounts, _, err := l.store.ListTenantAccounts(ctx, tenantID)
	if err != nil {
		return nil, failed(op, err)
	}

	candidates := make([]domain.StaffPreviewCandidate, 0, len(accounts))
	ids := make([]int64, 0, len(accounts))
	for _, info := range accounts {
		if info.AccountID == 0 || info.AccountID == excludeAccountID {
			continue
		}
		if !info.Active || info.Status != "active" {
			continue
		}
		roles := info.RoleNames
		if len(roles) == 0 || domain.IsGuardianOnly(roles) {
			continue
		}
		if isLehrkraftOnlyByName(roles) {
			schoolPortalOnly, err := l.isSchoolPortalOnlyAtTenant(ctx, info.AccountID, tenantID)
			if err != nil {
				return nil, failed(op, err)
			}
			if schoolPortalOnly {
				continue
			}
		}
		candidates = append(candidates, domain.StaffPreviewCandidate{AccountID: info.AccountID, Email: info.Email, Roles: roles})
		ids = append(ids, info.AccountID)
	}
	if len(ids) == 0 {
		return candidates, nil
	}
	names, err := l.staff.FindPersonNames(ctx, ids)
	if err != nil {
		return nil, failed(op, err)
	}
	for i := range candidates {
		if name, ok := names[candidates[i].AccountID]; ok {
			candidates[i].FirstName = name.FirstName
			candidates[i].LastName = name.LastName
		}
	}
	return candidates, nil
}

// isSchoolPortalOnlyAtTenant answers the start path's question for a listing
// row: does this account hold ONLY school-portal roles at the tenant? An
// account with no assignment row is not school-portal-only.
func (l *AccountLifecycle) isSchoolPortalOnlyAtTenant(ctx context.Context, accountID, tenantID int64) (bool, error) {
	roles, _, err := l.logins.ListAccountRolesAtTenant(ctx, accountID, tenantID, false)
	if err != nil {
		return false, err
	}
	return domain.IsSchoolPortalOnly(roles), nil
}

// isLehrkraftOnlyByName is the cheap name-based prefilter: it decides nothing
// on its own, it only marks the rows worth one extra role lookup.
func isLehrkraftOnlyByName(roleNames []string) bool {
	if len(roleNames) == 0 {
		return false
	}
	for _, name := range roleNames {
		if !strings.EqualFold(name, domain.LehrkraftRoleName) {
			return false
		}
	}
	return true
}

func staffPreviewDisplayName(claims *domain.AccountClaimsPayload, email string) string {
	name := strings.TrimSpace(strings.TrimSpace(claims.FirstName) + " " + strings.TrimSpace(claims.LastName))
	if name != "" {
		return name
	}
	if claims.Username != "" {
		return claims.Username
	}
	return email
}
