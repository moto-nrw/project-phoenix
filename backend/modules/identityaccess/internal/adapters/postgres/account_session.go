package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/uptrace/bun"
)

// accountSessionRow mirrors auth.tokens. The default tags keep the insert
// semantics of the retained model: zero timestamps take the database clock
// and an unset portal scope stores the column default.
type accountSessionRow struct {
	bun.BaseModel     `bun:"table:auth.tokens,alias:token"`
	ID                int64      `bun:"id,pk,autoincrement"`
	TenantID          int64      `bun:"tenant_id,notnull"`
	AccountID         int64      `bun:"account_id,notnull"`
	Token             string     `bun:"token,notnull"`
	Expiry            time.Time  `bun:"expiry,notnull"`
	Mobile            bool       `bun:"mobile,notnull,default:false"`
	Identifier        *string    `bun:"identifier"`
	PortalScope       string     `bun:"portal_scope,notnull,default:'unknown'"`
	FamilyID          string     `bun:"family_id"`
	FamilyExpiryCap   *time.Time `bun:"family_expiry_cap"`
	Generation        int        `bun:"generation,default:0"`
	RotatedAt         *time.Time `bun:"rotated_at"`
	ReplacementToken  *string    `bun:"replacement_token"`
	RecoveryProofHash []byte     `bun:"recovery_proof_hash"`
	CreatedAt         time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt         time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (r accountSessionRow) toDomain() domain.AccountSession {
	return domain.AccountSession{
		ID: r.ID, TenantID: r.TenantID, AccountID: r.AccountID, Token: r.Token, Expiry: r.Expiry, Mobile: r.Mobile,
		Identifier: r.Identifier, PortalScope: r.PortalScope, FamilyID: r.FamilyID, FamilyExpiryCap: r.FamilyExpiryCap,
		Generation: r.Generation, RotatedAt: r.RotatedAt, ReplacementToken: r.ReplacementToken, RecoveryProofHash: r.RecoveryProofHash,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func accountSessionRowFromDomain(session domain.AccountSession) accountSessionRow {
	return accountSessionRow{
		ID: session.ID, TenantID: session.TenantID, AccountID: session.AccountID, Token: session.Token, Expiry: session.Expiry, Mobile: session.Mobile,
		Identifier: session.Identifier, PortalScope: session.PortalScope, FamilyID: session.FamilyID, FamilyExpiryCap: session.FamilyExpiryCap,
		Generation: session.Generation, RotatedAt: session.RotatedAt, ReplacementToken: session.ReplacementToken, RecoveryProofHash: session.RecoveryProofHash,
		CreatedAt: session.CreatedAt, UpdatedAt: session.UpdatedAt,
	}
}

func accountSessionsToDomain(rows []accountSessionRow) []domain.AccountSession {
	result := make([]domain.AccountSession, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	return result
}

type tenantFilterable[Q any] interface {
	Where(string, ...any) Q
}

// withTenantFilter narrows the statement to the caller's tenant when the
// runtime scoped the caller to one. Tenantless callers (login, refresh and
// logout run before a tenant is known) see every row, as the retained
// repository's defense-in-depth filter did.
func withTenantFilter[Q tenantFilterable[Q]](scope TenantScope, query Q) Q {
	if scope.TenantID > 0 {
		return query.Where(`"token".tenant_id = ?`, scope.TenantID)
	}
	return query
}

// withTenantFilterUnlessAdmin is withTenantFilter for the cap and retirement
// writes: inside an administrative transaction they act on every school the
// account is mapped to.
func withTenantFilterUnlessAdmin[Q tenantFilterable[Q]](scope TenantScope, query Q) Q {
	if scope.AdminTransaction {
		return query
	}
	return withTenantFilter(scope, query)
}

func (s *Store) FindAccountSessionByToken(ctx context.Context, token string, forUpdate bool) (domain.AccountSession, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.AccountSession{}, false, domain.OperationStats{}, err
	}
	var row accountSessionRow
	query := withTenantFilter(s.scope(ctx), db.NewSelect().Model(&row).Where(`"token".token = ?`, token))
	if forUpdate {
		query = query.For("UPDATE")
	}
	started := time.Now()
	err = query.Scan(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AccountSession{}, false, stats, nil
	}
	if err != nil {
		return domain.AccountSession{}, false, stats, fmt.Errorf("identity access postgres: find account session: %w", err)
	}
	return row.toDomain(), true, stats, nil
}

func (s *Store) LatestAccountSessionInFamily(ctx context.Context, familyID string) (domain.AccountSession, bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.AccountSession{}, false, domain.OperationStats{}, err
	}
	var row accountSessionRow
	query := withTenantFilter(s.scope(ctx), db.NewSelect().Model(&row).Where(`"token".family_id = ?`, familyID)).
		OrderExpr(`"token".generation DESC`).Limit(1)
	started := time.Now()
	err = query.Scan(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AccountSession{}, false, stats, nil
	}
	if err != nil {
		return domain.AccountSession{}, false, stats, fmt.Errorf("identity access postgres: latest account session in family: %w", err)
	}
	return row.toDomain(), true, stats, nil
}

func (s *Store) ListAccountSessions(ctx context.Context, filter domain.AccountSessionFilter, now time.Time) ([]domain.AccountSession, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []accountSessionRow
	query := withTenantFilter(s.scope(ctx), db.NewSelect().Model(&rows))
	if filter.AccountID > 0 {
		query = query.Where(`"token".account_id = ?`, filter.AccountID)
	}
	if filter.FamilyID != "" {
		query = query.Where(`"token".family_id = ?`, filter.FamilyID)
	}
	if filter.Mobile != nil {
		query = query.Where(`"token".mobile = ?`, *filter.Mobile)
	}
	switch filter.Liveness {
	case domain.AccountSessionsLive:
		query = query.Where(`"token".expiry > ?`, now)
	case domain.AccountSessionsExpired:
		query = query.Where(`"token".expiry <= ?`, now)
	}
	started := time.Now()
	err = query.Scan(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list account sessions: %w", err)
	}
	return accountSessionsToDomain(rows), stats, nil
}

func (s *Store) CountExpiredAccountSessions(ctx context.Context, now time.Time) (int, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	started := time.Now()
	count, err := db.NewSelect().Model((*accountSessionRow)(nil)).Where(`"token".expiry < ?`, now).Count(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return 0, stats, fmt.Errorf("identity access postgres: count expired account sessions: %w", err)
	}
	return count, stats, nil
}

func (s *Store) ListInactiveAccountIDsWithLiveSessions(ctx context.Context, now time.Time) ([]int64, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var ids []int64
	started := time.Now()
	err = db.NewSelect().
		ColumnExpr(`DISTINCT "token".account_id`).
		TableExpr(`auth.tokens AS "token"`).
		Join(`INNER JOIN auth.accounts AS "account" ON "account".id = "token".account_id`).
		Where(`"account".active = ?`, false).
		Where(`"token".rotated_at IS NULL`).
		Where(`"token".expiry > ?`, now).
		Scan(ctx, &ids)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list inactive accounts with live sessions: %w", err)
	}
	return ids, stats, nil
}

func (s *Store) HasLiveAccountSessionsCreatedAfter(ctx context.Context, accountID int64, since, now time.Time) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	started := time.Now()
	exists, err := db.NewSelect().Model((*accountSessionRow)(nil)).
		Where(`"token".account_id = ?`, accountID).
		Where(`"token".rotated_at IS NULL`).
		Where(`"token".expiry > ?`, now).
		Where(`"token".created_at > ?`, since).
		Exists(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: check live account sessions created after: %w", err)
	}
	return exists, stats, nil
}

func (s *Store) InsertAccountSession(ctx context.Context, session domain.AccountSession) (domain.AccountSession, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.AccountSession{}, domain.OperationStats{}, err
	}
	row := accountSessionRowFromDomain(session)
	started := time.Now()
	result, err := db.NewInsert().Model(&row).Returning("id, created_at, updated_at").Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return domain.AccountSession{}, stats, fmt.Errorf("identity access postgres: insert account session: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return row.toDomain(), stats, nil
}

func (s *Store) MarkAccountSessionRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) (bool, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	query := withTenantFilter(s.scope(ctx), db.NewUpdate().Model((*accountSessionRow)(nil)).
		Set(`rotated_at = ?`, rotatedAt).
		Set(`replacement_token = ?`, replacementToken).
		Set(`recovery_proof_hash = ?`, recoveryProofHash).
		Where(`"token".id = ?`, id).
		Where(`"token".rotated_at IS NULL`))
	started := time.Now()
	result, err := query.Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return false, stats, fmt.Errorf("identity access postgres: mark account session rotated: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats.Rows == 1, stats, nil
}

// DeleteExpiredRotatedAccountSessions removes predecessor rows only after
// their refresh JWTs expire. Until then they are replay-detection evidence
// and must remain attributable to their token family.
func (s *Store) DeleteExpiredRotatedAccountSessions(ctx context.Context, accountID int64, now time.Time) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	scope := s.scope(ctx)
	candidates := withTenantFilter(scope, db.NewSelect().Model((*accountSessionRow)(nil)).
		ColumnExpr(`"token".id`).
		Where(`"token".account_id = ?`, accountID).
		Where(`"token".rotated_at IS NOT NULL`).
		Where(`"token".expiry <= ?`, now)).
		For("UPDATE SKIP LOCKED")
	query := withTenantFilter(scope, db.NewDelete().Model((*accountSessionRow)(nil)).Where(`"token".id IN (?)`, candidates))
	started := time.Now()
	result, err := query.Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: delete expired rotated account sessions: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

// RetireAccountSessionFamily caps the expiry of a family's live sessions so a
// tenant switch hands the browser's previous session a bounded grace period
// instead of leaving it alive for the full refresh lifetime. The cap is
// persisted with the live session so refresh successors cannot revive the
// retired family.
func (s *Store) RetireAccountSessionFamily(ctx context.Context, accountID int64, familyID string, expiry time.Time) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenantFilterUnlessAdmin(s.scope(ctx), db.NewUpdate().Model((*accountSessionRow)(nil)).
		Set(`expiry = LEAST("token".expiry, ?)`, expiry).
		Set(`family_expiry_cap = LEAST(COALESCE("token".family_expiry_cap, ?), ?)`, expiry, expiry).
		Where(`"token".account_id = ?`, accountID).
		Where(`"token".family_id = ?`, familyID).
		Where(`"token".rotated_at IS NULL`))
	started := time.Now()
	result, err := query.Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: retire account session family: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

// ListLiveAccountSessionsForCap orders the sessions closest to expiry last.
// Families retired by a tenant switch therefore sit at the end, so a burst
// of switches evicts its own leftovers before it touches a session on
// another device (#2952).
func (s *Store) ListLiveAccountSessionsForCap(ctx context.Context, accountID int64, portalScopes []string, now time.Time) ([]domain.AccountSession, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []accountSessionRow
	query := withTenantFilterUnlessAdmin(s.scope(ctx), db.NewSelect().Model(&rows).
		Where(`"token".account_id = ?`, accountID).
		Where(`"token".portal_scope IN (?)`, bun.List(portalScopes)).
		Where(`"token".rotated_at IS NULL`).
		Where(`"token".expiry > ?`, now)).
		OrderExpr(`"token".expiry DESC, "token".id DESC`)
	started := time.Now()
	err = query.Scan(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: list live account sessions for cap: %w", err)
	}
	return accountSessionsToDomain(rows), stats, nil
}

func (s *Store) DeleteAccountSessionsByID(ctx context.Context, ids []int64) ([]domain.AccountSession, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []accountSessionRow
	query := withTenantFilterUnlessAdmin(s.scope(ctx), db.NewDelete().Model((*accountSessionRow)(nil)).Where(`"token".id IN (?)`, bun.List(ids))).Returning("*")
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: delete account sessions by id: %w", err)
	}
	stats.Rows = int64(len(rows))
	return accountSessionsToDomain(rows), stats, nil
}

func (s *Store) DeleteAccountSession(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	query := withTenantFilter(s.scope(ctx), db.NewDelete().Model((*accountSessionRow)(nil)).Where(`"token".id = ?`, id))
	started := time.Now()
	result, err := query.Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return stats, fmt.Errorf("identity access postgres: delete account session: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

func (s *Store) DeleteAccountSessionsByFamily(ctx context.Context, familyID string) ([]domain.AccountSession, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []accountSessionRow
	query := withTenantFilter(s.scope(ctx), db.NewDelete().Model((*accountSessionRow)(nil)).Where(`"token".family_id = ?`, familyID)).Returning("*")
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: delete account sessions by family: %w", err)
	}
	stats.Rows = int64(len(rows))
	return accountSessionsToDomain(rows), stats, nil
}

// DeleteAccountSessionsByAccount with a cutoff deletes sessions that already
// existed at cutoff: sessions created then, later refresh successors of
// those families, and empty-family legacy rows. New logins after cutoff keep
// a new family id and are left alone.
func (s *Store) DeleteAccountSessionsByAccount(ctx context.Context, accountID int64, tenantScoped bool, cutoff time.Time) ([]domain.AccountSession, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []accountSessionRow
	query := db.NewDelete().Model((*accountSessionRow)(nil)).Where(`"token".account_id = ?`, accountID)
	if tenantScoped {
		query = withTenantFilter(s.scope(ctx), query)
	}
	if !cutoff.IsZero() {
		query = query.Where(`(
			"token".created_at <= ?
			OR "token".family_id = ?
			OR "token".family_id IN (
				SELECT preexisting.family_id
				FROM auth.tokens AS preexisting
				WHERE preexisting.account_id = ?
					AND preexisting.family_id <> ?
					AND preexisting.created_at <= ?
			)
			OR (
				"token".family_id <> ?
				AND "token".generation > 0
				AND NOT EXISTS (
					SELECT 1
					FROM auth.tokens AS origin
					WHERE origin.account_id = "token".account_id
						AND origin.family_id = "token".family_id
						AND origin.generation = 0
						AND origin.created_at > ?
				)
			)
		)`, cutoff, "", accountID, "", cutoff, "", cutoff)
	}
	started := time.Now()
	err = query.Returning("*").Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: delete account sessions by account: %w", err)
	}
	stats.Rows = int64(len(rows))
	return accountSessionsToDomain(rows), stats, nil
}

func (s *Store) DeleteAccountSessionsByTenant(ctx context.Context, tenantID int64) ([]domain.AccountSession, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []accountSessionRow
	started := time.Now()
	err = db.NewDelete().Model((*accountSessionRow)(nil)).Where(`"token".tenant_id = ?`, tenantID).Returning("*").Scan(ctx, &rows)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, fmt.Errorf("identity access postgres: delete account sessions by tenant: %w", err)
	}
	stats.Rows = int64(len(rows))
	return accountSessionsToDomain(rows), stats, nil
}

func (s *Store) DeleteExpiredAccountSessions(ctx context.Context, now time.Time) (int, domain.OperationStats, error) {
	db, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withTenantFilter(s.scope(ctx), db.NewDelete().Model((*accountSessionRow)(nil)).Where(`"token".expiry < ?`, now))
	started := time.Now()
	result, err := query.Exec(ctx)
	stats := domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}
	if err != nil {
		return 0, stats, fmt.Errorf("identity access postgres: delete expired account sessions: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return int(stats.Rows), stats, nil
}
