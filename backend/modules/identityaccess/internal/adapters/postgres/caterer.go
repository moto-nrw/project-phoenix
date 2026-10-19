package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

// The caterer, its accounts, its schools and the schools' invitations (ADR
// 0074). Everything is written in the school's transaction; the caterer of
// an account and its schools are read through the global identity read.

type catererRow struct {
	bun.BaseModel `bun:"table:auth.caterers,alias:caterer"`
	ID            int64  `bun:"id,pk,autoincrement"`
	Name          string `bun:"name,notnull"`
}

type catererAccountRow struct {
	bun.BaseModel `bun:"table:auth.caterer_accounts,alias:caterer_account"`
	AccountID     int64 `bun:"account_id,pk"`
	CatererID     int64 `bun:"caterer_id,notnull"`
}

type catererSchoolRow struct {
	bun.BaseModel `bun:"table:auth.caterer_schools,alias:caterer_school"`
	CatererID     int64      `bun:"caterer_id,notnull"`
	TenantID      int64      `bun:"tenant_id,notnull"`
	StartedAt     time.Time  `bun:"started_at,notnull"`
	EndedAt       *time.Time `bun:"ended_at"`
}

type catererInvitationRow struct {
	bun.BaseModel     `bun:"table:auth.caterer_invitations,alias:caterer_invitation"`
	ID                int64      `bun:"id,pk,autoincrement"`
	TenantID          int64      `bun:"tenant_id,notnull"`
	TokenHash         string     `bun:"token_hash,notnull"`
	Email             string     `bun:"email,notnull"`
	CatererName       string     `bun:"caterer_name,notnull"`
	DeliveryAddress   string     `bun:"delivery_address,notnull"`
	CreatedBy         *int64     `bun:"created_by_account_id"`
	ExpiresAt         time.Time  `bun:"expires_at,notnull"`
	AcceptedAt        *time.Time `bun:"accepted_at"`
	AcceptedAccountID *int64     `bun:"accepted_account_id"`
	RevokedAt         *time.Time `bun:"revoked_at"`
	CreatedAt         time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
}

func (r catererInvitationRow) toDomain() domain.CatererInvitation {
	return domain.CatererInvitation{
		ID: r.ID, TenantID: r.TenantID, TokenHash: r.TokenHash, Email: r.Email, CatererName: r.CatererName,
		DeliveryAddress: r.DeliveryAddress, CreatedBy: r.CreatedBy, ExpiresAt: r.ExpiresAt, AcceptedAt: r.AcceptedAt,
		AcceptedAccountID: r.AcceptedAccountID, RevokedAt: r.RevokedAt, CreatedAt: r.CreatedAt,
	}
}

func catererInvitationTenantFilter[Q tenantFilterable[Q]](scope TenantScope, query Q) Q {
	return scopeTenantQuery(scope, query, `"caterer_invitation".tenant_id = ?`)
}

func (s *Store) FindCatererOfAccount(ctx context.Context, accountID int64) (domain.Caterer, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.Caterer{}, false, domain.OperationStats{}, err
	}
	var row catererRow
	started := time.Now()
	err = db.NewSelect().Model(&row).
		Join(`JOIN auth.caterer_accounts AS "caterer_account" ON "caterer_account".caterer_id = "caterer".id`).
		Where(`"caterer_account".account_id = ?`, accountID).
		Scan(ctx)
	found, stats, err := scanOne("find caterer of account", started, err)
	return domain.Caterer{ID: row.ID, Name: row.Name}, found, stats, err
}

func (s *Store) InsertCaterer(ctx context.Context, name string) (domain.Caterer, domain.OperationStats, error) {
	row := catererRow{Name: name}
	_, stats, err := s.exec(ctx, "insert caterer", func(db bun.IDB) executor {
		return db.NewInsert().Model(&row).Returning("id")
	})
	return domain.Caterer{ID: row.ID, Name: row.Name}, stats, err
}

func (s *Store) InsertCatererAccount(ctx context.Context, accountID, catererID int64) (domain.OperationStats, error) {
	_, stats, err := s.exec(ctx, "insert caterer account", func(db bun.IDB) executor {
		return db.NewInsert().Model(&catererAccountRow{AccountID: accountID, CatererID: catererID})
	})
	return stats, err
}

func (s *Store) InsertCatererInvitation(ctx context.Context, invitation domain.CatererInvitation) (domain.CatererInvitation, domain.OperationStats, error) {
	row := catererInvitationRow{
		TenantID: invitation.TenantID, TokenHash: invitation.TokenHash, Email: invitation.Email, CatererName: invitation.CatererName,
		DeliveryAddress: invitation.DeliveryAddress, CreatedBy: invitation.CreatedBy, ExpiresAt: invitation.ExpiresAt,
	}
	_, stats, err := s.exec(ctx, "insert caterer invitation", func(db bun.IDB) executor {
		return db.NewInsert().Model(&row).ExcludeColumn("id", "created_at").Returning("id, created_at")
	})
	return row.toDomain(), stats, err
}

func (s *Store) ListCatererInvitations(ctx context.Context) ([]domain.CatererInvitation, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []catererInvitationRow
	started := time.Now()
	err = catererInvitationTenantFilter(s.scope(ctx), db.NewSelect().Model(&rows)).
		OrderExpr(`"caterer_invitation".created_at DESC, "caterer_invitation".id DESC`).
		Scan(ctx)
	_, stats, err := scanOne("list caterer invitations", started, err)
	invitations := make([]domain.CatererInvitation, 0, len(rows))
	for _, row := range rows {
		invitations = append(invitations, row.toDomain())
	}
	stats.Rows = int64(len(rows))
	return invitations, stats, err
}

func (s *Store) RevokeCatererInvitation(ctx context.Context, invitationID int64, at time.Time) (bool, domain.OperationStats, error) {
	rows, stats, err := s.exec(ctx, "revoke caterer invitation", func(db bun.IDB) executor {
		return catererInvitationTenantFilter(s.scope(ctx), db.NewUpdate().Model((*catererInvitationRow)(nil)).
			Set("revoked_at = ?", at).
			Where(`"caterer_invitation".id = ?`, invitationID).
			Where(`"caterer_invitation".accepted_at IS NULL`).
			Where(`"caterer_invitation".revoked_at IS NULL`))
	})
	return rows > 0, stats, err
}

func (s *Store) FindCatererInvitationByTokenHash(ctx context.Context, tokenHash string) (domain.CatererInvitation, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.CatererInvitation{}, false, domain.OperationStats{}, err
	}
	var row catererInvitationRow
	started := time.Now()
	err = catererInvitationTenantFilter(s.scope(ctx), db.NewSelect().Model(&row).
		Where(`"caterer_invitation".token_hash = ?`, tokenHash)).
		For("UPDATE").
		Scan(ctx)
	found, stats, err := scanOne("find caterer invitation by token", started, err)
	return row.toDomain(), found, stats, err
}

func (s *Store) AcceptCatererInvitation(ctx context.Context, invitationID, accountID int64, at time.Time) (bool, domain.OperationStats, error) {
	rows, stats, err := s.exec(ctx, "accept caterer invitation", func(db bun.IDB) executor {
		return db.NewUpdate().Model((*catererInvitationRow)(nil)).
			Set("accepted_at = ?", at).
			Set("accepted_account_id = ?", accountID).
			Where(`"caterer_invitation".id = ?`, invitationID).
			Where(`"caterer_invitation".accepted_at IS NULL`).
			Where(`"caterer_invitation".revoked_at IS NULL`)
	})
	return rows > 0, stats, err
}

func (s *Store) InsertCatererSchool(ctx context.Context, catererID int64, at time.Time) (domain.OperationStats, error) {
	tenantID := s.scope(ctx).TenantID
	if tenantID <= 0 {
		return domain.OperationStats{}, fmt.Errorf("identity access postgres: a caterer school needs the school's transaction")
	}
	_, stats, err := s.exec(ctx, "insert caterer school", func(db bun.IDB) executor {
		return db.NewInsert().Model(&catererSchoolRow{CatererID: catererID, TenantID: tenantID, StartedAt: at})
	})
	return stats, err
}

func (s *Store) EndCatererSchool(ctx context.Context, at time.Time) (bool, domain.OperationStats, error) {
	rows, stats, err := s.exec(ctx, "end caterer school", func(db bun.IDB) executor {
		return scopeTenantQuery(s.scope(ctx), db.NewUpdate().Model((*catererSchoolRow)(nil)).
			Set("ended_at = ?", at).
			Where(`"caterer_school".ended_at IS NULL`), `"caterer_school".tenant_id = ?`)
	})
	return rows > 0, stats, err
}

func (s *Store) ListCatererSchools(ctx context.Context, catererID int64) ([]int64, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []catererSchoolRow
	started := time.Now()
	err = db.NewSelect().Model(&rows).
		Where(`"caterer_school".caterer_id = ?`, catererID).
		Where(`"caterer_school".ended_at IS NULL`).
		OrderExpr(`"caterer_school".started_at, "caterer_school".tenant_id`).
		Scan(ctx)
	_, stats, err := scanOne("list caterer schools", started, err)
	schools := make([]int64, 0, len(rows))
	for _, row := range rows {
		schools = append(schools, row.TenantID)
	}
	stats.Rows = int64(len(rows))
	return schools, stats, err
}

func (s *Store) FindSchoolCaterer(ctx context.Context) (domain.Caterer, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.Caterer{}, false, domain.OperationStats{}, err
	}
	var row catererRow
	started := time.Now()
	err = scopeTenantQuery(s.scope(ctx), db.NewSelect().Model(&row).
		Join(`JOIN auth.caterer_schools AS "caterer_school" ON "caterer_school".caterer_id = "caterer".id`).
		Where(`"caterer_school".ended_at IS NULL`), `"caterer_school".tenant_id = ?`).
		Scan(ctx)
	found, stats, err := scanOne("find school caterer", started, err)
	return domain.Caterer{ID: row.ID, Name: row.Name}, found, stats, err
}
