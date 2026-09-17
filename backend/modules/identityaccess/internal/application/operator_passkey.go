package application

import (
	"context"
	"encoding/json"
	"time"

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
	return p.service.run(ctx, p.service.tx.RunPlatform, operation, fn)
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
			return domain.ErrOperatorPasskeyNotFound
		}
		credential, found, queryStats, findErr := p.store.FindActiveOperatorPasskey(txCtx, credentialID, userHandle)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrOperatorPasskeyNotFound
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
			return domain.ErrOperatorPasskeyJSONInvalid
		}
		changed, queryStats, err := p.store.UpdateOperatorPasskeyAfterUse(txCtx, id, credentialJSON, usedAt)
		stats.Add(queryStats)
		return stateChange(changed, err, domain.ErrOperatorPasskeyNotFound)
	})
}

// RevokeCredential revokes an active credential of that operator; another
// operator's credential is reported as not found.
func (p *OperatorPasskey) RevokeCredential(ctx context.Context, operatorID, id int64, revokedAt time.Time) error {
	return p.run(ctx, "revoke_operator_passkey", func(txCtx context.Context, stats *domain.OperationStats) error {
		changed, queryStats, err := p.store.RevokeOperatorPasskey(txCtx, operatorID, id, revokedAt)
		stats.Add(queryStats)
		return stateChange(changed, err, domain.ErrOperatorPasskeyNotFound)
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
			return domain.ErrOperatorPasskeySessionNotFound
		}
		session, found, queryStats, consumeErr := p.store.ConsumeOperatorPasskeySession(txCtx, id, purpose, consumedAt)
		stats.Add(queryStats)
		if consumeErr != nil {
			return consumeErr
		}
		if !found {
			return domain.ErrOperatorPasskeySessionNotFound
		}
		result = session
		return nil
	})
	return result, err
}
