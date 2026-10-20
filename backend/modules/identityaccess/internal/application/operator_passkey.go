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

// OperatorPasskey serves the operator passkey credentials and ceremony
// sessions (#2724). The rows are platform-wide, so every operation runs
// through RunPlatform: it joins the administrative transaction a caller
// opened and otherwise executes on the root connection.
type OperatorPasskey struct {
	service *Service
	store   ports.OperatorPasskeyStore
}

func NewOperatorPasskey(service *Service, store ports.OperatorPasskeyStore) *OperatorPasskey {
	if service == nil || store == nil {
		panic("identity access application: operator passkey requires the service and its store")
	}
	return &OperatorPasskey{service: service, store: store}
}

func (p *OperatorPasskey) run(ctx context.Context, operation string, fn func(context.Context, *domain.OperationStats) error) error {
	return p.service.run(ctx, p.service.tx.RunAdmin, operation, fn)
}

func (p *OperatorPasskey) CreateCredential(ctx context.Context, credential domain.OperatorPasskeyCredential) (result domain.OperatorPasskeyCredential, err error) {
	err = p.run(ctx, "create_operator_passkey", func(txCtx context.Context, stats *domain.OperationStats) error {
		if validateErr := credential.Validate(); validateErr != nil {
			return validateErr
		}
		stored, queryStats, insertErr := p.store.InsertOperatorPasskey(txCtx, credential)
		stats.Add(queryStats)
		if insertErr != nil {
			return insertErr
		}
		result = stored
		return nil
	})
	return result, err
}

func (p *OperatorPasskey) ListActiveCredentials(ctx context.Context, operatorID int64) (result []domain.OperatorPasskeyCredential, err error) {
	err = p.run(ctx, "list_active_operator_passkeys", func(txCtx context.Context, stats *domain.OperationStats) error {
		credentials, queryStats, listErr := p.store.ListActiveOperatorPasskeys(txCtx, operatorID)
		stats.Add(queryStats)
		if listErr != nil {
			return listErr
		}
		result = credentials
		return nil
	})
	return result, err
}

func (p *OperatorPasskey) FindActiveCredential(ctx context.Context, credentialID, userHandle []byte) (result domain.OperatorPasskeyCredential, err error) {
	err = p.run(ctx, "find_active_operator_passkey", func(txCtx context.Context, stats *domain.OperationStats) error {
		if len(credentialID) == 0 || len(userHandle) == 0 {
			return identityaccess.ErrOperatorPasskeyNotFound
		}
		credential, found, queryStats, findErr := p.store.FindActiveOperatorPasskey(txCtx, credentialID, userHandle)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return identityaccess.ErrOperatorPasskeyNotFound
		}
		result = credential
		return nil
	})
	return result, err
}

// RecordUse stores the credential state after a verified assertion. A
// credential revoked in the meantime is reported, never updated.
func (p *OperatorPasskey) RecordUse(ctx context.Context, id int64, credentialJSON json.RawMessage, usedAt time.Time) error {
	return p.run(ctx, "record_operator_passkey_use", func(txCtx context.Context, stats *domain.OperationStats) error {
		if !json.Valid(credentialJSON) {
			return domain.ErrPasskeyJSONInvalid
		}
		changed, queryStats, err := p.store.UpdateOperatorPasskeyAfterUse(txCtx, id, credentialJSON, usedAt)
		stats.Add(queryStats)
		return stateChange(changed, err, identityaccess.ErrOperatorPasskeyNotFound)
	})
}

// RevokeCredential revokes an active credential of that operator; another
// operator's credential is reported as not found.
func (p *OperatorPasskey) RevokeCredential(ctx context.Context, operatorID, id int64, revokedAt time.Time) error {
	return p.run(ctx, "revoke_operator_passkey", func(txCtx context.Context, stats *domain.OperationStats) error {
		changed, queryStats, err := p.store.RevokeOperatorPasskey(txCtx, operatorID, id, revokedAt)
		stats.Add(queryStats)
		return stateChange(changed, err, identityaccess.ErrOperatorPasskeyNotFound)
	})
}

func (p *OperatorPasskey) CreateSession(ctx context.Context, session domain.OperatorPasskeySession) (result domain.OperatorPasskeySession, err error) {
	err = p.run(ctx, "create_operator_passkey_session", func(txCtx context.Context, stats *domain.OperationStats) error {
		if validateErr := session.Validate(); validateErr != nil {
			return validateErr
		}
		stored, queryStats, insertErr := p.store.InsertOperatorPasskeySession(txCtx, session)
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
// concurrent completions receives ErrOperatorPasskeySessionNotFound and must
// neither register a credential nor mint a session.
func (p *OperatorPasskey) ConsumeSession(ctx context.Context, id, purpose string, consumedAt time.Time) (result domain.OperatorPasskeySession, err error) {
	err = p.run(ctx, "consume_operator_passkey_session", func(txCtx context.Context, stats *domain.OperationStats) error {
		if id == "" {
			return identityaccess.ErrOperatorPasskeySessionNotFound
		}
		session, found, queryStats, consumeErr := p.store.ConsumeOperatorPasskeySession(txCtx, id, purpose, consumedAt)
		stats.Add(queryStats)
		if consumeErr != nil {
			return consumeErr
		}
		if !found {
			return identityaccess.ErrOperatorPasskeySessionNotFound
		}
		result = session
		return nil
	})
	return result, err
}

func (p *OperatorPasskey) ListActiveOperatorPasskeys(ctx context.Context, operatorID int64) ([]identityaccess.OperatorPasskeyCredential, error) {
	values, err := p.ListActiveCredentials(ctx, operatorID)
	if err != nil {
		return nil, fmt.Errorf("identity access: list active operator passkeys: %w", MapError(err))
	}
	result := make([]identityaccess.OperatorPasskeyCredential, 0, len(values))
	for _, value := range values {
		result = append(result, identityaccess.OperatorPasskeyCredential(value))
	}
	return result, nil
}

func (p *OperatorPasskey) FindActiveOperatorPasskey(ctx context.Context, credentialID, userHandle []byte) (identityaccess.OperatorPasskeyCredential, error) {
	value, err := p.FindActiveCredential(ctx, credentialID, userHandle)
	if err != nil {
		return identityaccess.OperatorPasskeyCredential{}, fmt.Errorf("identity access: find active operator passkey: %w", MapError(err))
	}
	return identityaccess.OperatorPasskeyCredential(value), nil
}

func (p *OperatorPasskey) CreateOperatorPasskey(ctx context.Context, credential identityaccess.OperatorPasskeyCredential) (identityaccess.OperatorPasskeyCredential, error) {
	value, err := p.CreateCredential(ctx, domain.OperatorPasskeyCredential(credential))
	if err != nil {
		return identityaccess.OperatorPasskeyCredential{}, fmt.Errorf("identity access: create operator passkey: %w", MapError(err))
	}
	return identityaccess.OperatorPasskeyCredential(value), nil
}

func (p *OperatorPasskey) RecordOperatorPasskeyUse(ctx context.Context, id int64, credentialJSON json.RawMessage, usedAt time.Time) error {
	if err := p.RecordUse(ctx, id, credentialJSON, usedAt); err != nil {
		return fmt.Errorf("identity access: record operator passkey use: %w", MapError(err))
	}
	return nil
}

func (p *OperatorPasskey) RevokeOperatorPasskey(ctx context.Context, operatorID, id int64, revokedAt time.Time) error {
	if err := p.RevokeCredential(ctx, operatorID, id, revokedAt); err != nil {
		return fmt.Errorf("identity access: revoke operator passkey: %w", MapError(err))
	}
	return nil
}

func (p *OperatorPasskey) CreateOperatorPasskeySession(ctx context.Context, session identityaccess.OperatorPasskeySession) (identityaccess.OperatorPasskeySession, error) {
	value, err := p.CreateSession(ctx, domain.OperatorPasskeySession(session))
	if err != nil {
		return identityaccess.OperatorPasskeySession{}, fmt.Errorf("identity access: create operator passkey session: %w", MapError(err))
	}
	return identityaccess.OperatorPasskeySession(value), nil
}

func (p *OperatorPasskey) ConsumeOperatorPasskeySession(ctx context.Context, id, purpose string, consumedAt time.Time) (identityaccess.OperatorPasskeySession, error) {
	value, err := p.ConsumeSession(ctx, id, purpose, consumedAt)
	if err != nil {
		return identityaccess.OperatorPasskeySession{}, fmt.Errorf("identity access: consume operator passkey session: %w", MapError(err))
	}
	return identityaccess.OperatorPasskeySession(value), nil
}
