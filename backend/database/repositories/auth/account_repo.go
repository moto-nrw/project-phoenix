package auth

import (
	"context"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// AccountRepository implements auth.AccountRepository
type AccountRepository struct {
	db *bun.DB
}

// NewAccountRepository creates a new account repository
func NewAccountRepository(db *bun.DB) auth.AccountRepository {
	return &AccountRepository{db: db}
}

// Create inserts a new account into the database
func (r *AccountRepository) Create(ctx context.Context, account *auth.Account) error {
	if err := account.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(account).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves an account by its ID
func (r *AccountRepository) FindByID(ctx context.Context, id interface{}) (*auth.Account, error) {
	account := new(auth.Account)
	err := r.db.NewSelect().Model(account).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return account, nil
}

// FindByEmail retrieves an account by email
func (r *AccountRepository) FindByEmail(ctx context.Context, email string) (*auth.Account, error) {
	account := new(auth.Account)
	err := r.db.NewSelect().Model(account).Where("email = ?", strings.ToLower(email)).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_email", Err: err}
	}
	return account, nil
}

// FindByUsername retrieves an account by username
func (r *AccountRepository) FindByUsername(ctx context.Context, username string) (*auth.Account, error) {
	account := new(auth.Account)
	err := r.db.NewSelect().Model(account).Where("username = ?", username).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_username", Err: err}
	}
	return account, nil
}

// Update updates an existing account
func (r *AccountRepository) Update(ctx context.Context, account *auth.Account) error {
	if err := account.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(account).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes an account
func (r *AccountRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*auth.Account)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves accounts matching the filters
func (r *AccountRepository) List(ctx context.Context, filters map[string]interface{}) ([]*auth.Account, error) {
	var accounts []*auth.Account
	query := r.db.NewSelect().Model(&accounts)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return accounts, nil
}

// UpdateLastLogin updates the last_login field for an account
func (r *AccountRepository) UpdateLastLogin(ctx context.Context, id int64) error {
	now := time.Now()
	_, err := r.db.NewUpdate().
		Model((*auth.Account)(nil)).
		Set("last_login = ?", now).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_last_login", Err: err}
	}
	return nil
}

// UpdatePassword updates the password hash for an account
func (r *AccountRepository) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	_, err := r.db.NewUpdate().
		Model((*auth.Account)(nil)).
		Set("password_hash = ?", passwordHash).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_password", Err: err}
	}
	return nil
}

// FindByRole retrieves accounts with a specific role
func (r *AccountRepository) FindByRole(ctx context.Context, role string) ([]*auth.Account, error) {
	var accounts []*auth.Account
	err := r.db.NewSelect().
		Model(&accounts).
		Where("? = ANY(roles)", role).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_role", Err: err}
	}
	return accounts, nil
}
