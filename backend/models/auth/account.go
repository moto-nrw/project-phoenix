package auth

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Account represents a user account in the system
type Account struct {
	base.Model
	LastLogin    *time.Time `bun:"last_login" json:"last_login,omitempty"`
	Email        string     `bun:"email,notnull" json:"email"`
	Username     string     `bun:"username,unique" json:"username,omitempty"`
	Active       bool       `bun:"active,notnull,default:true" json:"active"`
	Roles        []string   `bun:"roles,array" json:"roles"`
	PasswordHash string     `bun:"password_hash" json:"-"`

	// Relations
	Tokens              []*Token              `bun:"rel:has-many,join:id=account_id" json:"-"`
	PasswordResetTokens []*PasswordResetToken `bun:"rel:has-many,join:id=account_id" json:"-"`
}

// TableName returns the table name for the Account model
func (a *Account) TableName() string {
	return "auth.accounts"
}

// GetID returns the account ID
func (a *Account) GetID() interface{} {
	return a.ID
}

// GetCreatedAt returns the creation timestamp
func (a *Account) GetCreatedAt() time.Time {
	return a.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (a *Account) GetUpdatedAt() time.Time {
	return a.UpdatedAt
}

// Validate validates the account fields
func (a *Account) Validate() error {
	// Validate email
	if a.Email == "" {
		return errors.New("email is required")
	}

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(a.Email) {
		return errors.New("invalid email format")
	}

	// Validate username if provided
	if a.Username != "" {
		usernameRegex := regexp.MustCompile(`^[a-zA-Z0-9._\-]{3,30}$`)
		if !usernameRegex.MatchString(a.Username) {
			return errors.New("username must be 3-30 characters and contain only letters, numbers, dots, underscores, or hyphens")
		}
	}

	// Validate roles
	for _, role := range a.Roles {
		if strings.TrimSpace(role) == "" {
			return errors.New("roles cannot be empty strings")
		}
	}

	return nil
}

// BeforeAppend sets default values before saving to the database
func (a *Account) BeforeAppend() error {
	// Call parent's BeforeAppend to set timestamps
	if err := a.Model.BeforeAppend(); err != nil {
		return err
	}

	// Set default roles if empty
	if len(a.Roles) == 0 {
		a.Roles = []string{"user"}
	}

	// Normalize email
	a.Email = strings.ToLower(strings.TrimSpace(a.Email))

	// Normalize username
	if a.Username != "" {
		a.Username = strings.TrimSpace(a.Username)
	}

	return nil
}

// AccountRepository defines operations for working with accounts
type AccountRepository interface {
	base.Repository[*Account]
	FindByEmail(ctx context.Context, email string) (*Account, error)
	FindByUsername(ctx context.Context, username string) (*Account, error)
	UpdateLastLogin(ctx context.Context, id int64) error
	UpdatePassword(ctx context.Context, id int64, passwordHash string) error
	FindByRole(ctx context.Context, role string) ([]*Account, error)
}

// DefaultAccountRepository is the default implementation of AccountRepository
type DefaultAccountRepository struct {
	db *bun.DB
}

// NewAccountRepository creates a new account repository
func NewAccountRepository(db *bun.DB) AccountRepository {
	return &DefaultAccountRepository{db: db}
}

// Create inserts a new account into the database
func (r *DefaultAccountRepository) Create(ctx context.Context, account *Account) error {
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
func (r *DefaultAccountRepository) FindByID(ctx context.Context, id interface{}) (*Account, error) {
	account := new(Account)
	err := r.db.NewSelect().Model(account).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return account, nil
}

// FindByEmail retrieves an account by email
func (r *DefaultAccountRepository) FindByEmail(ctx context.Context, email string) (*Account, error) {
	account := new(Account)
	err := r.db.NewSelect().Model(account).Where("email = ?", strings.ToLower(email)).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_email", Err: err}
	}
	return account, nil
}

// FindByUsername retrieves an account by username
func (r *DefaultAccountRepository) FindByUsername(ctx context.Context, username string) (*Account, error) {
	account := new(Account)
	err := r.db.NewSelect().Model(account).Where("username = ?", username).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_username", Err: err}
	}
	return account, nil
}

// Update updates an existing account
func (r *DefaultAccountRepository) Update(ctx context.Context, account *Account) error {
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
func (r *DefaultAccountRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*Account)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves accounts matching the filters
func (r *DefaultAccountRepository) List(ctx context.Context, filters map[string]interface{}) ([]*Account, error) {
	var accounts []*Account
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
func (r *DefaultAccountRepository) UpdateLastLogin(ctx context.Context, id int64) error {
	now := time.Now()
	_, err := r.db.NewUpdate().
		Model((*Account)(nil)).
		Set("last_login = ?", now).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_last_login", Err: err}
	}
	return nil
}

// UpdatePassword updates the password hash for an account
func (r *DefaultAccountRepository) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	_, err := r.db.NewUpdate().
		Model((*Account)(nil)).
		Set("password_hash = ?", passwordHash).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_password", Err: err}
	}
	return nil
}

// FindByRole retrieves accounts with a specific role
func (r *DefaultAccountRepository) FindByRole(ctx context.Context, role string) ([]*Account, error) {
	var accounts []*Account
	err := r.db.NewSelect().
		Model(&accounts).
		Where("? = ANY(roles)", role).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_role", Err: err}
	}
	return accounts, nil
}
