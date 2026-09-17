package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

// auth.guardian_invitations as the guardian invitation flows (#2722) and the
// relative access flows (#3225) read and write it. The table is
// tenant-scoped: an administrative transaction — the public validate and
// accept routes run in one — reaches every school's invitation, a
// tenant-scoped caller only its own.

type guardianInvitationRow struct {
	bun.BaseModel     `bun:"table:auth.guardian_invitations,alias:guardian_invitation"`
	ID                int64      `bun:"id,pk,autoincrement"`
	TenantID          int64      `bun:"tenant_id,notnull"`
	Token             string     `bun:"token,notnull"`
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
	CreatedAt                   time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt                   time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (r guardianInvitationRow) toDomain() domain.GuardianInvitation {
	return domain.GuardianInvitation{
		ID: r.ID, TenantID: r.TenantID, Token: r.Token, GuardianProfileID: r.GuardianProfileID, CreatedBy: r.CreatedBy,
		ExpiresAt: r.ExpiresAt, AcceptedAt: r.AcceptedAt, EmailSentAt: r.EmailSentAt, EmailError: r.EmailError,
		StudentID: r.StudentID, RequestedByAccountID: r.RequestedByAccountID, ApprovalStatus: r.ApprovalStatus,
		ApprovedBy: r.ApprovedBy, ApprovedAt: r.ApprovedAt, ProfileCreatedForInvitation: r.ProfileCreatedForInvitation,
		RoleUpgrade: r.RoleUpgrade, CreatedAt: r.CreatedAt,
	}
}

func guardianInvitationRowOf(invitation domain.GuardianInvitation) guardianInvitationRow {
	return guardianInvitationRow{
		ID: invitation.ID, TenantID: invitation.TenantID, Token: invitation.Token, GuardianProfileID: invitation.GuardianProfileID,
		CreatedBy: invitation.CreatedBy, ExpiresAt: invitation.ExpiresAt, AcceptedAt: invitation.AcceptedAt,
		EmailSentAt: invitation.EmailSentAt, EmailError: invitation.EmailError, StudentID: invitation.StudentID,
		RequestedByAccountID: invitation.RequestedByAccountID, ApprovalStatus: invitation.ApprovalStatus,
		ApprovedBy: invitation.ApprovedBy, ApprovedAt: invitation.ApprovedAt,
		ProfileCreatedForInvitation: invitation.ProfileCreatedForInvitation, RoleUpgrade: invitation.RoleUpgrade,
	}
}

func guardianTenantFilter[Q tenantFilterable[Q]](scope TenantScope, query Q) Q {
	if scope.TenantID > 0 {
		return query.Where(`"guardian_invitation".tenant_id = ?`, scope.TenantID)
	}
	return query
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
	err = guardianTenantFilter(s.scope(ctx), db.NewSelect().Model(&row).Where(`"guardian_invitation".token = ?`, token)).Scan(ctx)
	found, _, err := scanOne("find guardian invitation by token", started, err)
	return row.toDomain(), found, err
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

// ListPendingGuardianApprovals returns the invitations of the school in
// context that wait for a staff decision, oldest first.
func (s *Store) ListPendingGuardianApprovals(ctx context.Context) ([]domain.GuardianInvitation, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []guardianInvitationRow
	err = guardianTenantFilter(s.scope(ctx), db.NewSelect().Model(&rows).
		Where(`"guardian_invitation".approval_status = ?`, domain.GuardianInvitationApprovalPending)).
		OrderExpr(`"guardian_invitation".created_at ASC`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("identity access postgres: list pending guardian approvals: %w", err)
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
	row := guardianInvitationRowOf(invitation)
	row.ID = 0
	if row.ApprovalStatus == "" {
		row.ApprovalStatus = domain.GuardianInvitationApprovalNotRequired
	}
	if _, err := db.NewInsert().Model(&row).Returning("*").Exec(ctx); err != nil {
		return domain.GuardianInvitation{}, fmt.Errorf("identity access postgres: insert guardian invitation: %w", err)
	}
	return row.toDomain(), nil
}

// UpdateGuardianInvitation writes every mutable column of the invitation.
func (s *Store) UpdateGuardianInvitation(ctx context.Context, invitation domain.GuardianInvitation) error {
	row := guardianInvitationRowOf(invitation)
	scope := s.scope(ctx)
	rows, _, err := s.exec(ctx, "update guardian invitation", func(db bun.IDB) executor {
		return guardianTenantFilter(scope, db.NewUpdate().Model(&row).
			Column("token", "guardian_profile_id", "expires_at", "accepted_at", "email_sent_at", "email_error",
				"student_id", "requested_by_account_id", "approval_status", "approved_by", "approved_at",
				"profile_created_for_invitation", "role_upgrade").
			Set("updated_at = NOW()").
			Where(`"guardian_invitation".id = ?`, invitation.ID))
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
// and reports whether such a row existed.
func (s *Store) AcceptGuardianInvitation(ctx context.Context, id int64, acceptedAt time.Time) (bool, error) {
	scope := s.scope(ctx)
	rows, _, err := s.exec(ctx, "accept guardian invitation", func(db bun.IDB) executor {
		return guardianTenantFilter(scope, db.NewUpdate().Model((*guardianInvitationRow)(nil)).
			Set("accepted_at = ?", acceptedAt).
			Set("updated_at = NOW()").
			Where(`"guardian_invitation".id = ?`, id).
			Where(`"guardian_invitation".accepted_at IS NULL`))
	})
	return rows == 1, err
}
