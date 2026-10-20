package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Parent account management over auth.accounts_parents. The rows are
// tenant-scoped; the store applies the tenant the context carries.

const (
	opCreateParentAccount     = "create parent account"
	opGetParentAccount        = "get parent account"
	opGetParentAccountByEmail = "get parent account by email"
	opUpdateParentAccount     = "update parent account"
	opActivateParentAccount   = "activate parent account"
	opDeactivateParentAccount = "deactivate parent account"
	opListParentAccounts      = "list parent accounts"
	opHashPassword            = "hash password"
)

func (l *AccountLifecycle) CreateParentAccount(ctx context.Context, email, username, password string) (domain.ParentAccount, error) {
	username = strings.TrimSpace(username)

	if err := l.passwords.ValidatePasswordStrength(password); err != nil {
		return domain.ParentAccount{}, failed(opCreateParentAccount, err)
	}
	var err error
	email, err = domain.NormalizeAccountEmail(email)
	if err != nil {
		return domain.ParentAccount{}, failed(opCreateParentAccount, err)
	}
	if _, found, _, err := l.store.FindParentAccountByEmail(ctx, email); err == nil && found {
		return domain.ParentAccount{}, failed(opCreateParentAccount, identityaccess.ErrEmailAlreadyExists)
	}
	if _, found, _, err := l.store.FindParentAccountByUsername(ctx, username); err == nil && found {
		return domain.ParentAccount{}, failed(opCreateParentAccount, identityaccess.ErrUsernameAlreadyExists)
	}
	hash, err := l.passwords.HashPassword(password)
	if err != nil {
		return domain.ParentAccount{}, failed(opHashPassword, err)
	}
	created, _, err := l.store.InsertParentAccount(ctx, domain.ParentAccount{
		TenantID: l.runtime.TenantID(ctx), Email: email, Username: username, Active: true, PasswordHash: hash,
	})
	if err != nil {
		return domain.ParentAccount{}, failed(opCreateParentAccount, err)
	}
	return created, nil
}

func (l *AccountLifecycle) findParentAccount(ctx context.Context, op string, id int64) (domain.ParentAccount, error) {
	account, found, _, err := l.store.FindParentAccount(ctx, id)
	if err != nil {
		return domain.ParentAccount{}, failed(op, err)
	}
	if !found {
		return domain.ParentAccount{}, failed(op, identityaccess.ErrParentAccountNotFound)
	}
	return account, nil
}

func (l *AccountLifecycle) GetParentAccountByID(ctx context.Context, id int64) (domain.ParentAccount, error) {
	return l.findParentAccount(ctx, opGetParentAccount, id)
}

func (l *AccountLifecycle) GetParentAccountByEmail(ctx context.Context, email string) (domain.ParentAccount, error) {
	account, found, _, err := l.store.FindParentAccountByEmail(ctx, strings.TrimSpace(strings.ToLower(email)))
	if err != nil {
		return domain.ParentAccount{}, failed(opGetParentAccountByEmail, err)
	}
	if !found {
		return domain.ParentAccount{}, failed(opGetParentAccountByEmail, identityaccess.ErrParentAccountNotFound)
	}
	return account, nil
}

func (l *AccountLifecycle) UpdateParentAccount(ctx context.Context, account domain.ParentAccount) error {
	existing, err := l.findParentAccount(ctx, opUpdateParentAccount, account.ID)
	if err != nil {
		return err
	}
	account.Email, err = domain.NormalizeAccountEmail(account.Email)
	if err != nil {
		return failed(opUpdateParentAccount, err)
	}
	if account.PasswordHash == "" {
		account.PasswordHash = existing.PasswordHash
	}
	updated, _, err := l.store.UpdateParentAccount(ctx, account)
	if err != nil {
		return failed(opUpdateParentAccount, err)
	}
	if !updated {
		return failed(opUpdateParentAccount, identityaccess.ErrParentAccountNotFound)
	}
	return nil
}

func (l *AccountLifecycle) setParentAccountActive(ctx context.Context, op string, id int64, active bool) error {
	account, err := l.findParentAccount(ctx, op, id)
	if err != nil {
		return err
	}
	account.Active = active
	if _, _, err := l.store.UpdateParentAccount(ctx, account); err != nil {
		return failed(op, err)
	}
	return nil
}

func (l *AccountLifecycle) ActivateParentAccount(ctx context.Context, id int64) error {
	return l.setParentAccountActive(ctx, opActivateParentAccount, id, true)
}

func (l *AccountLifecycle) DeactivateParentAccount(ctx context.Context, id int64) error {
	return l.setParentAccountActive(ctx, opDeactivateParentAccount, id, false)
}

func (l *AccountLifecycle) ListParentAccounts(ctx context.Context, filter domain.ParentAccountFilter) ([]domain.ParentAccount, error) {
	accounts, _, err := l.store.ListParentAccounts(ctx, filter)
	if err != nil {
		return nil, failed(opListParentAccounts, err)
	}
	return accounts, nil
}
