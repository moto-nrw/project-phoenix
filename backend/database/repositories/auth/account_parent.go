package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// AccountParentRepository implements auth.AccountParentRepository interface
type AccountParentRepository struct {
	*base.Repository[*auth.AccountParent]
	db *bun.DB
}

// NewAccountParentRepository creates a new AccountParentRepository
func NewAccountParentRepository(db *bun.DB) auth.AccountParentRepository {
	return &AccountParentRepository{
		Repository: base.NewRepository[*auth.AccountParent](db, "auth.accounts_parents", "AccountParent"),
		db:         db,
	}
}

// FindByEmail retrieves a parent account by email address
func (r *AccountParentRepository) FindByEmail(ctx context.Context, email string) (*auth.AccountParent, error) {
	account := new(auth.AccountParent)
	err := r.db.NewSelect().
		Model(account).
		Where("LOWER(email) = LOWER(?)", email).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by email",
			Err: err,
		}
	}

	return account, nil
}

// FindByUsername retrieves a parent account by username
func (r *AccountParentRepository) FindByUsername(ctx context.Context, username string) (*auth.AccountParent, error) {
	account := new(auth.AccountParent)
	err := r.db.NewSelect().
		Model(account).
		Where("LOWER(username) = LOWER(?)", username).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by username",
			Err: err,
		}
	}

	return account, nil
}

// UpdateLastLogin updates the last login timestamp for a parent account
func (r *AccountParentRepository) UpdateLastLogin(ctx context.Context, id int64) error {
	_, err := r.db.NewUpdate().
		Model((*auth.AccountParent)(nil)).
		Set("last_login = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update last login",
			Err: err,
		}
	}

	return nil
}

// UpdatePassword updates the password hash for a parent account
func (r *AccountParentRepository) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	_, err := r.db.NewUpdate().
		Model((*auth.AccountParent)(nil)).
		Set("password_hash = ?", passwordHash).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update password",
			Err: err,
		}
	}

	return nil
}

// List retrieves parent accounts matching the provided filters
func (r *AccountParentRepository) List(ctx context.Context, filters map[string]interface{}) ([]*auth.AccountParent, error) {
	var accounts []*auth.AccountParent
	query := r.db.NewSelect().Model(&accounts)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			switch field {
			case "email":
				// Case-insensitive email search
				if strValue, ok := value.(string); ok {
					query = query.Where("LOWER(email) = LOWER(?)", strValue)
				} else {
					query = query.Where("email = ?", value)
				}
			case "username":
				// Case-insensitive username search
				if strValue, ok := value.(string); ok {
					query = query.Where("LOWER(username) = LOWER(?)", strValue)
				} else {
					query = query.Where("username = ?", value)
				}
			case "email_like":
				// Case-insensitive email pattern search
				if strValue, ok := value.(string); ok {
					query = query.Where("LOWER(email) LIKE LOWER(?)", "%"+strValue+"%")
				}
			case "username_like":
				// Case-insensitive username pattern search
				if strValue, ok := value.(string); ok {
					query = query.Where("LOWER(username) LIKE LOWER(?)", "%"+strValue+"%")
				}
			case "active":
				query = query.Where("active = ?", value)
			default:
				// Default to exact match for other fields
				query = query.Where("? = ?", bun.Ident(field), value)
			}
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return accounts, nil
}

// Create overrides the base Create method to handle email normalization
func (r *AccountParentRepository) Create(ctx context.Context, account *auth.AccountParent) error {
	if account == nil {
		return fmt.Errorf("account parent cannot be nil")
	}

	// Validate account - this will also normalize the email
	if err := account.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, account)
}

// Update overrides the base Update method to handle email normalization
func (r *AccountParentRepository) Update(ctx context.Context, account *auth.AccountParent) error {
	if account == nil {
		return fmt.Errorf("account parent cannot be nil")
	}

	// Validate account - this will also normalize the email
	if err := account.Validate(); err != nil {
		return err
	}

	// Use the base Update method
	return r.Repository.Update(ctx, account)
}
