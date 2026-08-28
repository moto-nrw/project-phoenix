package auth

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

type StaffCalendarFeedTokenRepository struct {
	db *bun.DB
}

func NewStaffCalendarFeedTokenRepository(db *bun.DB) authModels.StaffCalendarFeedTokenRepository {
	return &StaffCalendarFeedTokenRepository{db: db}
}

func (r *StaffCalendarFeedTokenRepository) FindOwnerByTokenHash(ctx context.Context, tokenHash string) (*authModels.StaffCalendarFeedOwner, error) {
	if tokenHash == "" {
		return nil, nil
	}
	owner := new(authModels.StaffCalendarFeedOwner)
	err := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`auth.account_tenants AS "account_tenant"`).
		ColumnExpr(`"account_tenant".account_id`).
		ColumnExpr(`"account_tenant".tenant_id`).
		Where(`"account_tenant".staff_calendar_feed_token = ?`, tokenHash).
		Where(`"account_tenant".status = ?`, authModels.AccountTenantStatusActive).
		Scan(ctx, owner)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find staff calendar feed owner", Err: err}
	}
	return owner, nil
}

func (r *StaffCalendarFeedTokenRepository) EnsureToken(ctx context.Context, accountID, tenantID int64, tokenHash string) (string, error) {
	db := base.GetDB(ctx, r.db)
	result, err := db.NewUpdate().
		TableExpr(`auth.account_tenants AS "account_tenant"`).
		Set("staff_calendar_feed_token = ?", tokenHash).
		Where(`"account_tenant".account_id = ?`, accountID).
		Where(`"account_tenant".tenant_id = ?`, tenantID).
		Where(`"account_tenant".status = ?`, authModels.AccountTenantStatusActive).
		Where(`("account_tenant".staff_calendar_feed_token IS NULL OR "account_tenant".staff_calendar_feed_token = '')`).
		Exec(ctx)
	if err != nil {
		return "", &modelBase.DatabaseError{Op: "ensure staff calendar feed token", Err: err}
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr == nil && rows > 0 {
		return tokenHash, nil
	}

	var stored string
	err = db.NewSelect().
		TableExpr(`auth.account_tenants AS "account_tenant"`).
		ColumnExpr(`COALESCE("account_tenant".staff_calendar_feed_token, '')`).
		Where(`"account_tenant".account_id = ?`, accountID).
		Where(`"account_tenant".tenant_id = ?`, tenantID).
		Where(`"account_tenant".status = ?`, authModels.AccountTenantStatusActive).
		Scan(ctx, &stored)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", &modelBase.DatabaseError{Op: "read staff calendar feed token", Err: err}
	}
	return stored, nil
}

func (r *StaffCalendarFeedTokenRepository) SetToken(ctx context.Context, accountID, tenantID int64, tokenHash string) (bool, error) {
	result, err := base.GetDB(ctx, r.db).NewUpdate().
		TableExpr(`auth.account_tenants AS "account_tenant"`).
		Set("staff_calendar_feed_token = ?", tokenHash).
		Where(`"account_tenant".account_id = ?`, accountID).
		Where(`"account_tenant".tenant_id = ?`, tenantID).
		Where(`"account_tenant".status = ?`, authModels.AccountTenantStatusActive).
		Exec(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "set staff calendar feed token", Err: err}
	}
	rows, err := result.RowsAffected()
	return err == nil && rows > 0, err
}
