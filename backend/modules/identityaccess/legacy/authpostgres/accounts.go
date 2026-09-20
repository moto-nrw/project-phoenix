package authpostgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authmodels"
	"github.com/uptrace/bun"
)

const (
	accountTable      = "auth.accounts"
	accountTableAlias = `auth.accounts AS "account"`
)

// AccountRepository implements auth.AccountRepository interface
type AccountRepository struct {
	*base.Repository[*authmodels.Account]
	db *bun.DB
}

// NewAccountRepository creates a new AccountRepository
func NewAccountRepository(db *bun.DB) authmodels.AccountRepository {
	return &AccountRepository{
		Repository: base.NewRepository[*authmodels.Account](db, accountTable, "Account"),
		db:         db,
	}
}

// FindByEmail retrieves an account by email address
func (r *AccountRepository) FindByEmail(ctx context.Context, email string) (*authmodels.Account, error) {
	account := new(authmodels.Account)

	// Explicitly specify the schema and table
	err := base.GetDB(ctx, r.db).NewSelect().
		ModelTableExpr(accountTable).
		Where("LOWER(email) = LOWER(?)", email).
		Scan(ctx, account)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by email",
			Err: base.TranslateNotFound(err),
		}
	}

	return account, nil
}

// List retrieves accounts matching the provided filters without applying an
// account-management boundary. Internal authentication flows remain global.
func (r *AccountRepository) List(ctx context.Context, filters map[string]interface{}) ([]*authmodels.Account, error) {
	return r.list(ctx, filters)
}

func (r *AccountRepository) list(ctx context.Context, filters map[string]interface{}) ([]*authmodels.Account, error) {
	var accounts []*authmodels.Account
	query := base.GetDB(ctx, r.db).NewSelect().Model(&accounts).ModelTableExpr(accountTableAlias)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = r.applyAccountFilter(ctx, query, field, value)
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: base.TranslateNotFound(err),
		}
	}

	return accounts, nil
}

// applyAccountFilter applies a single filter to the query
func (r *AccountRepository) applyAccountFilter(ctx context.Context, query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	switch field {
	case "email":
		return r.applyStringEqualFilter(query, bun.Safe("email"), value)
	case "username":
		return r.applyStringEqualFilter(query, bun.Safe("username"), value)
	case "email_like":
		return r.applyStringLikeFilter(query, bun.Safe("email"), value)
	case "username_like":
		return r.applyStringLikeFilter(query, bun.Safe("username"), value)
	case "active":
		return query.Where("active = ?", value)
	default:
		return query.Where("? = ?", bun.Ident(field), value)
	}
}

// applyStringEqualFilter applies case-insensitive equality filter for string fields
// The field is a column written in this file, never request input.
func (r *AccountRepository) applyStringEqualFilter(query *bun.SelectQuery, field bun.Safe, value interface{}) *bun.SelectQuery {
	if strValue, ok := value.(string); ok {
		return query.Where("LOWER(?) = LOWER(?)", field, strValue)
	}
	return query.Where("? = ?", field, value)
}

// applyStringLikeFilter applies case-insensitive LIKE filter for string fields
func (r *AccountRepository) applyStringLikeFilter(query *bun.SelectQuery, field bun.Safe, value interface{}) *bun.SelectQuery {
	if strValue, ok := value.(string); ok {
		return query.Where("LOWER(?) LIKE LOWER(?)", field, "%"+strValue+"%")
	}
	return query
}

// Update overrides the base Update method to handle email normalization.
func (r *AccountRepository) Update(ctx context.Context, account *authmodels.Account) error {
	return r.update(ctx, account)
}

func (r *AccountRepository) update(ctx context.Context, account *authmodels.Account) error {
	if account == nil {
		return fmt.Errorf("account cannot be nil")
	}

	// Validate account - this will also normalize the email
	if err := account.Validate(); err != nil {
		return err
	}

	// Execute the query using GetDB for transaction support
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(account).
		ModelTableExpr(accountTableAlias).
		Where(`"account".id = ?`, account.ID)

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: base.TranslateNotFound(err),
		}
	}
	return base.AssertRowsAffected(result, 1, "update account")
}
