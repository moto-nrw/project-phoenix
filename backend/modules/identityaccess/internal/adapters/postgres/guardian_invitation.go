package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

// auth.guardian_invitations as the guardian invitation flows (#2722) and the
// relative access flows (#3225) read and write it. The table is
// tenant-scoped: an administrative transaction — the public validate and
// accept routes run in one — reaches every school's invitation, a
// tenant-scoped caller only its own. The token column holds the hex SHA-256
// of the link token only (#4902): reads by ID or listings return the
// invitation without its token.

type guardianInvitationRow struct {
	bun.BaseModel     `bun:"table:auth.guardian_invitations,alias:guardian_invitation"`
	ID                int64      `bun:"id,pk,autoincrement"`
	TenantID          int64      `bun:"tenant_id,notnull"`
	TokenHash         string     `bun:"token,notnull"`
	GuardianProfileID int64      `bun:"guardian_profile_id,notnull"`
	CreatedBy         int64      `bun:"created_by,notnull"`
	ExpiresAt         time.Time  `bun:"expires_at,notnull"`
	AcceptedAt        *time.Time `bun:"accepted_at"`
	EmailSentAt       *time.Time `bun:"email_sent_at"`
	EmailError        *string    `bun:"email_error"`
	// EmailRetryCount is mapped so a row scan covers every column; the
	// guardian mail records its attempts in platform.email_outbox, so
	// nothing here writes it.
	EmailRetryCount             int        `bun:"email_retry_count"`
	StudentID                   *int64     `bun:"student_id"`
	RequestedByAccountID        *int64     `bun:"requested_by_account_id"`
	ApprovalStatus              string     `bun:"approval_status,notnull"`
	ApprovedBy                  *int64     `bun:"approved_by"`
	ApprovedAt                  *time.Time `bun:"approved_at"`
	ProfileCreatedForInvitation bool       `bun:"profile_created_for_invitation,notnull"`
	RoleUpgrade                 bool       `bun:"role_upgrade,notnull"`
	// Email is the address the link in the token column was issued for
	// (#6294); it changes only together with the token.
	Email     string    `bun:"email,nullzero"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

// LockGuardianAccountEmail closes the missing-row gap before the account lookup.
// The key is global because an account can belong to several schools. Keep the
// subsequent FOR UPDATE lookup too: account retention locks that existing row.
func (s *Store) LockGuardianAccountEmail(ctx context.Context, email string) error {
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('guardian_account:' || LOWER(BTRIM(?)), 0))`, email)
	if err != nil {
		return fmt.Errorf("identity access postgres: lock guardian account email: %w", err)
	}
	return nil
}

func (r guardianInvitationRow) toDomain() domain.GuardianInvitation {
	return domain.GuardianInvitation{
		ID: r.ID, TenantID: r.TenantID, GuardianProfileID: r.GuardianProfileID, CreatedBy: r.CreatedBy,
		ExpiresAt: r.ExpiresAt, AcceptedAt: r.AcceptedAt, EmailSentAt: r.EmailSentAt, EmailError: r.EmailError,
		StudentID: r.StudentID, RequestedByAccountID: r.RequestedByAccountID, ApprovalStatus: r.ApprovalStatus,
		ApprovedBy: r.ApprovedBy, ApprovedAt: r.ApprovedAt, ProfileCreatedForInvitation: r.ProfileCreatedForInvitation,
		RoleUpgrade: r.RoleUpgrade, CreatedAt: r.CreatedAt, Email: r.Email,
	}
}

// guardianInvitationRowOf hashes the raw token the invitation carries. An
// invitation read back from the store carries none; its row has no hash.
func guardianInvitationRowOf(invitation domain.GuardianInvitation) guardianInvitationRow {
	tokenHash := ""
	if invitation.Token != "" {
		tokenHash = domain.HashToken(invitation.Token)
	}
	return guardianInvitationRow{
		ID: invitation.ID, TenantID: invitation.TenantID, TokenHash: tokenHash, GuardianProfileID: invitation.GuardianProfileID,
		CreatedBy: invitation.CreatedBy, ExpiresAt: invitation.ExpiresAt, AcceptedAt: invitation.AcceptedAt,
		EmailSentAt: invitation.EmailSentAt, EmailError: invitation.EmailError, StudentID: invitation.StudentID,
		RequestedByAccountID: invitation.RequestedByAccountID, ApprovalStatus: invitation.ApprovalStatus,
		ApprovedBy: invitation.ApprovedBy, ApprovedAt: invitation.ApprovedAt,
		ProfileCreatedForInvitation: invitation.ProfileCreatedForInvitation, RoleUpgrade: invitation.RoleUpgrade,
		Email: invitation.Email,
	}
}

// errGuardianInvitationWithoutAddress refuses a link that names no address:
// nobody could redeem it (#6294).
var errGuardianInvitationWithoutAddress = errors.New("identity access postgres: a guardian invitation link needs the address it is issued for")

func guardianTenantFilter[Q tenantFilterable[Q]](scope TenantScope, query Q) Q {
	return scopeTenantQuery(scope, query, `"guardian_invitation".tenant_id = ?`)
}

func (s *Store) FindGuardianInvitation(ctx context.Context, id int64) (domain.GuardianInvitation, bool, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.GuardianInvitation{}, false, err
	}
	var row guardianInvitationRow
	started := time.Now()
	err = guardianTenantFilter(s.scope(ctx), db.NewSelect().Model(&row).Where(`"guardian_invitation".id = ?`, id)).Scan(ctx)
	found, _, err := scanOne("find guardian invitation", started, err)
	return row.toDomain(), found, err
}

func (s *Store) FindGuardianInvitationByToken(ctx context.Context, token string) (domain.GuardianInvitation, bool, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.GuardianInvitation{}, false, err
	}
	var row guardianInvitationRow
	started := time.Now()
	err = guardianTenantFilter(s.scope(ctx), db.NewSelect().Model(&row).Where(`"guardian_invitation".token = ?`, domain.HashToken(token))).Scan(ctx)
	found, _, err := scanOne("find guardian invitation by token", started, err)
	invitation := row.toDomain()
	invitation.Token = token
	return invitation, found, err
}

func (s *Store) ListGuardianInvitationsByProfile(ctx context.Context, guardianProfileID int64) ([]domain.GuardianInvitation, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []guardianInvitationRow
	err = guardianTenantFilter(s.scope(ctx), db.NewSelect().Model(&rows).
		Where(`"guardian_invitation".guardian_profile_id = ?`, guardianProfileID)).
		OrderExpr(`"guardian_invitation".created_at DESC`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("identity access postgres: list guardian invitations by profile: %w", err)
	}
	return guardianInvitations(rows), nil
}

// ListPendingGuardianApprovals returns the requests of the school in context
// that still wait for a staff decision, newest first. An expired or already
// accepted request is not one of them: staff must not approve a stale row
// that cleanup has yet to remove into a live child link.
func (s *Store) ListPendingGuardianApprovals(ctx context.Context) ([]domain.GuardianInvitation, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []guardianInvitationRow
	err = guardianTenantFilter(s.scope(ctx), db.NewSelect().Model(&rows).
		Where(`"guardian_invitation".approval_status = ?`, domain.GuardianInvitationApprovalPending).
		Where(`"guardian_invitation".accepted_at IS NULL`).
		Where(`"guardian_invitation".expires_at > ?`, time.Now())).
		OrderExpr(`"guardian_invitation".created_at DESC`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("identity access postgres: list pending guardian approvals: %w", err)
	}
	return guardianInvitations(rows), nil
}

// ListOpenGuardianInvitations returns every invitation of the requested
// profiles that is still going somewhere: not accepted, not expired, and
// either redeemable or waiting for a staff decision. One tenant-scoped read
// backs the whole guardian list. A legacy link without a recorded recipient
// counts too, also after it ran out: the address binding (1.15.865) ended it,
// nobody can redeem it, and the list shows it as a dead link until the
// sign-in data retention removes it (#6294).
func (s *Store) ListOpenGuardianInvitations(ctx context.Context, guardianProfileIDs []int64, now time.Time) ([]domain.GuardianInvitation, error) {
	if len(guardianProfileIDs) == 0 {
		return nil, nil
	}
	db, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []guardianInvitationRow
	err = guardianTenantFilter(s.scope(ctx), db.NewSelect().Model(&rows).
		Where(`"guardian_invitation".guardian_profile_id IN (?)`, bun.List(guardianProfileIDs)).
		Where(`"guardian_invitation".accepted_at IS NULL`).
		Where(`("guardian_invitation".expires_at > ? OR "guardian_invitation".email IS NULL)`, now).
		Where(`"guardian_invitation".approval_status IN (?)`, bun.List([]string{
			domain.GuardianInvitationApprovalNotRequired,
			domain.GuardianInvitationApprovalApproved,
			domain.GuardianInvitationApprovalPending,
		}))).
		OrderExpr(`"guardian_invitation".created_at DESC`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("identity access postgres: list open guardian invitations: %w", err)
	}
	return guardianInvitations(rows), nil
}

// ListRedeemableGuardianInvitations returns the invitations of the school in
// context whose link can still be spent, newest first. A request awaiting or
// refused by a staff decision has no usable link and is left out.
func (s *Store) ListRedeemableGuardianInvitations(ctx context.Context, now time.Time) ([]domain.GuardianInvitation, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []guardianInvitationRow
	err = guardianTenantFilter(s.scope(ctx), db.NewSelect().Model(&rows).
		Where(`"guardian_invitation".accepted_at IS NULL`).
		Where(`"guardian_invitation".expires_at > ?`, now).
		Where(`"guardian_invitation".approval_status IN (?)`, bun.List([]string{
			domain.GuardianInvitationApprovalNotRequired,
			domain.GuardianInvitationApprovalApproved,
		}))).
		OrderExpr(`"guardian_invitation".created_at DESC`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("identity access postgres: list redeemable guardian invitations: %w", err)
	}
	return guardianInvitations(rows), nil
}

func guardianInvitations(rows []guardianInvitationRow) []domain.GuardianInvitation {
	result := make([]domain.GuardianInvitation, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result
}

func (s *Store) InsertGuardianInvitation(ctx context.Context, invitation domain.GuardianInvitation) (domain.GuardianInvitation, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.GuardianInvitation{}, err
	}
	if invitation.Email == "" {
		return domain.GuardianInvitation{}, errGuardianInvitationWithoutAddress
	}
	row := guardianInvitationRowOf(invitation)
	row.ID = 0
	if row.ApprovalStatus == "" {
		row.ApprovalStatus = domain.GuardianInvitationApprovalNotRequired
	}
	if _, err := db.NewInsert().Model(&row).Returning("*").Exec(ctx); err != nil {
		return domain.GuardianInvitation{}, fmt.Errorf("identity access postgres: insert guardian invitation: %w", err)
	}
	created := row.toDomain()
	created.Token = invitation.Token
	return created, nil
}

// UpdateGuardianInvitation writes every mutable column of the invitation. The
// token and the address it is bound to are replaced only when the invitation
// carries a new raw token; an invitation read from the store carries none and
// keeps its link.
//
// Callers read the invitation without a lock, so the write is conditional on
// the state they read: the row must still be unaccepted and still carry
// expectedApprovalStatus and expectedExpiresAt. Otherwise an acceptance,
// approval, rejection or invalidation committed meanwhile would be overwritten;
// the write then matches no row and reports ErrGuardianInvitationNotFound.
func (s *Store) UpdateGuardianInvitation(ctx context.Context, invitation domain.GuardianInvitation, expectedApprovalStatus string, expectedExpiresAt time.Time) error {
	row := guardianInvitationRowOf(invitation)
	columns := []string{"guardian_profile_id", "expires_at", "accepted_at", "email_sent_at", "email_error",
		"student_id", "requested_by_account_id", "approval_status", "approved_by", "approved_at",
		"profile_created_for_invitation", "role_upgrade"}
	if row.TokenHash != "" {
		// A new link names the address it is mailed to (#6294).
		if row.Email == "" {
			return errGuardianInvitationWithoutAddress
		}
		columns = append(columns, "token", "email")
	}
	scope := s.scope(ctx)
	rows, _, err := s.exec(ctx, "update guardian invitation", func(db bun.IDB) executor {
		return guardianTenantFilter(scope, db.NewUpdate().Model(&row).
			Column(columns...).
			Set("updated_at = NOW()").
			Where(`"guardian_invitation".id = ?`, invitation.ID).
			Where(`"guardian_invitation".accepted_at IS NULL`).
			Where(`"guardian_invitation".expires_at = ?`, expectedExpiresAt).
			Where(`"guardian_invitation".approval_status = ?`, expectedApprovalStatus))
	})
	if err != nil {
		return err
	}
	if rows != 1 {
		return domain.ErrGuardianInvitationNotFound
	}
	return nil
}

// AcceptGuardianInvitation stamps accepted_at on an unaccepted invitation
// that still carries the link token the acceptance looked it up by, and
// reports whether such a row existed. A resend, approval or new invitation
// that replaced the link after the lookup leaves no matching row (#6495).
func (s *Store) AcceptGuardianInvitation(ctx context.Context, id int64, token string, acceptedAt time.Time) (bool, error) {
	scope := s.scope(ctx)
	rows, _, err := s.exec(ctx, "accept guardian invitation", func(db bun.IDB) executor {
		return guardianTenantFilter(scope, db.NewUpdate().Model((*guardianInvitationRow)(nil)).
			Set("accepted_at = ?", acceptedAt).
			Set("updated_at = NOW()").
			Where(`"guardian_invitation".id = ?`, id).
			Where(`"guardian_invitation".token = ?`, domain.HashToken(token)).
			Where(`"guardian_invitation".accepted_at IS NULL`).
			Where(`"guardian_invitation".expires_at >= ?`, acceptedAt).
			Where(`"guardian_invitation".approval_status IN (?, ?)`, domain.GuardianInvitationApprovalNotRequired, domain.GuardianInvitationApprovalApproved))
	})
	return rows == 1, err
}

// InvalidateGuardianInvitations ends every unspent link for this school's
// profile. Contact changes include pending approvals, rejecting them so they
// cannot refresh an ended link. Link replacement preserves those requests,
// which grant no access yet. Accepted invitations remain untouched.
func (s *Store) InvalidateGuardianInvitations(ctx context.Context, profileID int64, now time.Time, includePendingApprovals bool) error {
	if s.scope(ctx).TenantID <= 0 {
		return domain.ErrTenantRequired
	}
	db, err := s.database(ctx)
	if err != nil {
		return err
	}
	_, err = guardianTenantFilter(s.scope(ctx), db.NewUpdate().Model((*guardianInvitationRow)(nil)).
		Set("expires_at = LEAST(expires_at, ?)", now).
		Set("email = NULL").
		Set("approval_status = CASE WHEN approval_status = ? THEN ? ELSE approval_status END", domain.GuardianInvitationApprovalPending, domain.GuardianInvitationApprovalRejected).
		Set("updated_at = ?", now).
		Where("(? OR approval_status <> ?)", includePendingApprovals, domain.GuardianInvitationApprovalPending).
		Where("guardian_profile_id = ?", profileID).Where("accepted_at IS NULL").Where("expires_at > ?", now)).Exec(ctx)
	return err
}
