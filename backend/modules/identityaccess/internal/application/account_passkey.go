package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// AccountPasskey serves the school-portal passkey credentials and ceremony
// sessions (#2724). Credentials belong to the account and carry no tenant,
// and a login ceremony is completed before a tenant is known, so every
// operation runs through RunPlatform: it joins the administrative
// transaction a caller opened and otherwise opens an administrative
// transaction, even when the caller supplies school context.
type AccountPasskey struct {
	service *Service
	store   ports.AccountPasskeyStore
}

func NewAccountPasskey(service *Service, store ports.AccountPasskeyStore) *AccountPasskey {
	if service == nil || store == nil {
		panic("identity access application: account passkey requires the service and its store")
	}
	return &AccountPasskey{service: service, store: store}
}

func (p *AccountPasskey) run(ctx context.Context, operation string, fn func(context.Context, *domain.OperationStats) error) error {
	return p.service.run(withGlobalCredentialScope(ctx), p.service.tx.RunPlatform, operation, fn)
}

func (p *AccountPasskey) CreateCredential(ctx context.Context, credential domain.AccountPasskeyCredential) (result domain.AccountPasskeyCredential, err error) {
	err = p.run(ctx, "create_account_passkey", func(txCtx context.Context, stats *domain.OperationStats) error {
		if validateErr := credential.Validate(); validateErr != nil {
			return validateErr
		}
		stored, queryStats, insertErr := p.store.InsertAccountPasskey(txCtx, credential)
		stats.Add(queryStats)
		if insertErr != nil {
			return insertErr
		}
		result = stored
		return nil
	})
	return result, err
}

func (p *AccountPasskey) ListActiveCredentials(ctx context.Context, accountID int64) (result []domain.AccountPasskeyCredential, err error) {
	err = p.run(ctx, "list_active_account_passkeys", func(txCtx context.Context, stats *domain.OperationStats) error {
		credentials, queryStats, listErr := p.store.ListActiveAccountPasskeys(txCtx, accountID)
		stats.Add(queryStats)
		if listErr != nil {
			return listErr
		}
		result = credentials
		return nil
	})
	return result, err
}

func (p *AccountPasskey) FindActiveCredential(ctx context.Context, credentialID, userHandle []byte) (result domain.AccountPasskeyCredential, err error) {
	err = p.run(ctx, "find_active_account_passkey", func(txCtx context.Context, stats *domain.OperationStats) error {
		if len(credentialID) == 0 || len(userHandle) == 0 {
			return identityaccess.ErrAccountPasskeyNotFound
		}
		credential, found, queryStats, findErr := p.store.FindActiveAccountPasskey(txCtx, credentialID, userHandle)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return identityaccess.ErrAccountPasskeyNotFound
		}
		result = credential
		return nil
	})
	return result, err
}

// RecordUse stores the credential state after a verified assertion. A
// credential revoked in the meantime is reported, never updated.
func (p *AccountPasskey) RecordUse(ctx context.Context, id int64, credentialJSON json.RawMessage, usedAt time.Time) error {
	return p.run(ctx, "record_account_passkey_use", func(txCtx context.Context, stats *domain.OperationStats) error {
		if !json.Valid(credentialJSON) {
			return domain.ErrPasskeyJSONInvalid
		}
		changed, queryStats, err := p.store.UpdateAccountPasskeyAfterUse(txCtx, id, credentialJSON, usedAt)
		stats.Add(queryStats)
		return stateChange(changed, err, identityaccess.ErrAccountPasskeyNotFound)
	})
}

// RevokeCredential revokes an active credential of that account; another
// account's credential is reported as not found.
func (p *AccountPasskey) RevokeCredential(ctx context.Context, accountID, id int64, revokedAt time.Time) error {
	return p.run(ctx, "revoke_account_passkey", func(txCtx context.Context, stats *domain.OperationStats) error {
		changed, queryStats, err := p.store.RevokeAccountPasskey(txCtx, accountID, id, revokedAt)
		stats.Add(queryStats)
		return stateChange(changed, err, identityaccess.ErrAccountPasskeyNotFound)
	})
}

func (p *AccountPasskey) CreateSession(ctx context.Context, session domain.AccountPasskeySession) (result domain.AccountPasskeySession, err error) {
	err = p.run(ctx, "create_account_passkey_session", func(txCtx context.Context, stats *domain.OperationStats) error {
		if validateErr := session.Validate(); validateErr != nil {
			return validateErr
		}
		stored, queryStats, insertErr := p.store.InsertAccountPasskeySession(txCtx, session)
		stats.Add(queryStats)
		if insertErr != nil {
			return insertErr
		}
		result = stored
		return nil
	})
	return result, err
}

// ConsumeSession completes a ceremony exactly once: the loser of two
// concurrent completions receives ErrAccountPasskeySessionNotFound and must
// neither register a credential nor mint a session.
func (p *AccountPasskey) ConsumeSession(ctx context.Context, id, purpose string, consumedAt time.Time) (result domain.AccountPasskeySession, err error) {
	err = p.run(ctx, "consume_account_passkey_session", func(txCtx context.Context, stats *domain.OperationStats) error {
		if id == "" {
			return identityaccess.ErrAccountPasskeySessionNotFound
		}
		session, found, queryStats, consumeErr := p.store.ConsumeAccountPasskeySession(txCtx, id, purpose, consumedAt)
		stats.Add(queryStats)
		if consumeErr != nil {
			return consumeErr
		}
		if !found {
			return identityaccess.ErrAccountPasskeySessionNotFound
		}
		result = session
		return nil
	})
	return result, err
}

func (p *AccountPasskey) ListActiveAccountPasskeys(ctx context.Context, accountID int64) ([]identityaccess.AccountPasskeyCredential, error) {
	values, err := p.ListActiveCredentials(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("identity access: list active account passkeys: %w", MapError(err))
	}
	result := make([]identityaccess.AccountPasskeyCredential, 0, len(values))
	for _, value := range values {
		result = append(result, identityaccess.AccountPasskeyCredential(value))
	}
	return result, nil
}

func (p *AccountPasskey) FindActiveAccountPasskey(ctx context.Context, credentialID, userHandle []byte) (identityaccess.AccountPasskeyCredential, error) {
	value, err := p.FindActiveCredential(ctx, credentialID, userHandle)
	if err != nil {
		return identityaccess.AccountPasskeyCredential{}, fmt.Errorf("identity access: find active account passkey: %w", MapError(err))
	}
	return identityaccess.AccountPasskeyCredential(value), nil
}

func (p *AccountPasskey) CreateAccountPasskey(ctx context.Context, credential identityaccess.AccountPasskeyCredential) (identityaccess.AccountPasskeyCredential, error) {
	value, err := p.CreateCredential(ctx, domain.AccountPasskeyCredential(credential))
	if err != nil {
		return identityaccess.AccountPasskeyCredential{}, fmt.Errorf("identity access: create account passkey: %w", MapError(err))
	}
	return identityaccess.AccountPasskeyCredential(value), nil
}

func (p *AccountPasskey) RecordAccountPasskeyUse(ctx context.Context, id int64, credentialJSON json.RawMessage, usedAt time.Time) error {
	if err := p.RecordUse(ctx, id, credentialJSON, usedAt); err != nil {
		return fmt.Errorf("identity access: record account passkey use: %w", MapError(err))
	}
	return nil
}

func (p *AccountPasskey) RevokeAccountPasskey(ctx context.Context, accountID, id int64, revokedAt time.Time) error {
	if err := p.RevokeCredential(ctx, accountID, id, revokedAt); err != nil {
		return fmt.Errorf("identity access: revoke account passkey: %w", MapError(err))
	}
	return nil
}

func (p *AccountPasskey) CreateAccountPasskeySession(ctx context.Context, session identityaccess.AccountPasskeySession) (identityaccess.AccountPasskeySession, error) {
	value, err := p.CreateSession(ctx, domain.AccountPasskeySession(session))
	if err != nil {
		return identityaccess.AccountPasskeySession{}, fmt.Errorf("identity access: create account passkey session: %w", MapError(err))
	}
	return identityaccess.AccountPasskeySession(value), nil
}

func (p *AccountPasskey) ConsumeAccountPasskeySession(ctx context.Context, id, purpose string, consumedAt time.Time) (identityaccess.AccountPasskeySession, error) {
	value, err := p.ConsumeSession(ctx, id, purpose, consumedAt)
	if err != nil {
		return identityaccess.AccountPasskeySession{}, fmt.Errorf("identity access: consume account passkey session: %w", MapError(err))
	}
	return identityaccess.AccountPasskeySession(value), nil
}
