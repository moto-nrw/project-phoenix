package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/parentrequestfeedprojection"
	"github.com/uptrace/bun"
)

type Database func(context.Context) (bun.IDB, error)

type Store struct{ resolve Database }

type subscriptionRow struct {
	bun.BaseModel `bun:"table:parent_request_rss_feeds,alias:feed"`
	TenantID      int64  `bun:"tenant_id"`
	AccountID     int64  `bun:"account_id"`
	TokenHash     string `bun:"token_hash"`
}

func New(resolve Database) *Store {
	if resolve == nil {
		panic("request feed postgres: database is required")
	}
	return &Store{resolve: resolve}
}

func (s *Store) database(ctx context.Context) (bun.IDB, error) {
	return s.resolve(ctx)
}

func (s *Store) Active(ctx context.Context, tenantID, accountID int64) (bool, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, err
	}
	return db.NewSelect().TableExpr(`users.parent_request_rss_feeds AS "feed"`).
		Where(`"feed".tenant_id = ? AND "feed".account_id = ?`, tenantID, accountID).Exists(ctx)
}

func (s *Store) Create(ctx context.Context, tenantID, accountID int64, tokenHash string) (bool, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, err
	}
	row := subscriptionRow{TenantID: tenantID, AccountID: accountID, TokenHash: tokenHash}
	result, err := db.NewInsert().Model(&row).
		ModelTableExpr("users.parent_request_rss_feeds").
		On("CONFLICT (tenant_id, account_id) DO NOTHING").Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("create request feed subscription: %w", err)
	}
	rows, err := result.RowsAffected()
	return rows == 1, err
}

func (s *Store) Rotate(ctx context.Context, tenantID, accountID int64, tokenHash string) (bool, error) {
	db, err := s.database(ctx)
	if err != nil {
		return false, err
	}
	row := subscriptionRow{TokenHash: tokenHash}
	result, err := db.NewUpdate().Model(&row).
		ModelTableExpr("users.parent_request_rss_feeds").
		Column("token_hash").
		Where("tenant_id = ? AND account_id = ?", tenantID, accountID).Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("rotate request feed subscription: %w", err)
	}
	rows, err := result.RowsAffected()
	return rows == 1, err
}

func (s *Store) Resolve(ctx context.Context, tokenHash string) (domain.Subscription, bool, error) {
	db, err := s.database(ctx)
	if err != nil {
		return domain.Subscription{}, false, err
	}
	row := struct {
		TenantID  int64 `bun:"tenant_id"`
		AccountID int64 `bun:"account_id"`
	}{}
	err = db.NewSelect().TableExpr(`users.parent_request_rss_feeds AS "feed"`).
		ColumnExpr(`"feed".tenant_id, "feed".account_id`).
		Where(`"feed".token_hash = ?`, tokenHash).Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Subscription{}, false, nil
	}
	if err != nil {
		return domain.Subscription{}, false, fmt.Errorf("resolve request feed subscription: %w", err)
	}
	return domain.Subscription{TenantID: row.TenantID, AccountID: row.AccountID}, true, nil
}

func (s *Store) List(ctx context.Context, tenantID int64, since time.Time, access domain.Access) ([]domain.Item, error) {
	db, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	projected, err := parentrequestfeedprojection.ListNewParentRequests(
		ctx, db, tenantID, since, access.GeneralRequests, access.EnrollmentRequests,
	)
	if err != nil {
		return nil, fmt.Errorf("list request feed items: %w", err)
	}
	rows := make([]domain.Item, 0, len(projected))
	for _, item := range projected {
		rows = append(rows, domain.Item{Kind: item.Kind, ID: item.ID, CreatedAt: item.CreatedAt})
	}
	return rows, nil
}
