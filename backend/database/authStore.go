package database

import (
	"context"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"time"

	"github.com/uptrace/bun"
)

// AuthStore implements database operations for account authentication.
type AuthStore struct {
	db *bun.DB
}

// NewAuthStore return an AuthStore.
func NewAuthStore(db *bun.DB) *AuthStore {
	return &AuthStore{
		db: db,
	}
}

// GetAccount returns an account by ID.
func (s *AuthStore) GetAccount(id int) (*userpass.Account, error) {
	a := &userpass.Account{ID: id}
	err := s.db.NewSelect().
		Model(a).
		Where("id = ?", id).
		Scan(context.Background())
	return a, err
}

// GetAccountByEmail returns an account by email.
func (s *AuthStore) GetAccountByEmail(e string) (*userpass.Account, error) {
	a := &userpass.Account{Email: e}
	err := s.db.NewSelect().
		Model(a).
		Column("id", "active", "email", "username", "name", "password_hash").
		Where("email = ?", e).
		Scan(context.Background())
	return a, err
}

// GetAccountByUsername returns an account by username.
func (s *AuthStore) GetAccountByUsername(u string) (*userpass.Account, error) {
	a := &userpass.Account{Username: u}
	err := s.db.NewSelect().
		Model(a).
		Column("id", "active", "email", "username", "name", "password_hash").
		Where("username = ?", u).
		Scan(context.Background())
	return a, err
}

// CreateAccount creates a new account.
func (s *AuthStore) CreateAccount(a *userpass.Account) error {
	_, err := s.db.NewInsert().
		Model(a).
		Exec(context.Background())
	return err
}

// UpdateAccount updates account data related to authentication.
func (s *AuthStore) UpdateAccount(a *userpass.Account) error {
	_, err := s.db.NewUpdate().
		Model(a).
		Column("last_login").
		WherePK().
		Exec(context.Background())
	return err
}

// UpdateAccountPassword updates an account's password hash.
func (s *AuthStore) UpdateAccountPassword(id int, passwordHash string) error {
	_, err := s.db.NewUpdate().
		Model((*userpass.Account)(nil)).
		Set("password_hash = ?", passwordHash).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(context.Background())
	return err
}

// GetToken returns refresh token by token identifier.
func (s *AuthStore) GetToken(t string) (*jwt.Token, error) {
	token := &jwt.Token{Token: t}
	err := s.db.NewSelect().
		Model(token).
		Where("token = ?", t).
		Scan(context.Background())
	return token, err
}

// CreateOrUpdateToken creates or updates an existing refresh token.
func (s *AuthStore) CreateOrUpdateToken(t *jwt.Token) error {
	if t.ID == 0 {
		_, err := s.db.NewInsert().
			Model(t).
			Exec(context.Background())
		return err
	}
	_, err := s.db.NewUpdate().
		Model(t).
		WherePK().
		Exec(context.Background())
	return err
}

// DeleteToken deletes a refresh token.
func (s *AuthStore) DeleteToken(t *jwt.Token) error {
	_, err := s.db.NewDelete().
		Model(t).
		WherePK().
		Exec(context.Background())
	return err
}

// PurgeExpiredToken deletes expired refresh token.
func (s *AuthStore) PurgeExpiredToken() error {
	_, err := s.db.NewDelete().
		Model((*jwt.Token)(nil)).
		Where("expiry < ?", time.Now()).
		Exec(context.Background())
	return err
}
