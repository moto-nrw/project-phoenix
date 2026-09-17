package compose

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Engine methods for the school-portal passkey records (#2724). They need no
// dependencies beyond the database, so every composition serves them.

func (e engine) ListActiveAccountPasskeys(ctx context.Context, accountID int64) ([]identityaccess.AccountPasskeyCredential, error) {
	values, err := e.accountPasskeys.ListActiveCredentials(ctx, accountID)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]identityaccess.AccountPasskeyCredential, 0, len(values))
	for _, value := range values {
		result = append(result, identityaccess.AccountPasskeyCredential(value))
	}
	return result, nil
}

func (e engine) FindActiveAccountPasskey(ctx context.Context, credentialID, userHandle []byte) (identityaccess.AccountPasskeyCredential, error) {
	value, err := e.accountPasskeys.FindActiveCredential(ctx, credentialID, userHandle)
	return identityaccess.AccountPasskeyCredential(value), mapError(err)
}

func (e engine) CreateAccountPasskey(ctx context.Context, credential identityaccess.AccountPasskeyCredential) (identityaccess.AccountPasskeyCredential, error) {
	value, err := e.accountPasskeys.CreateCredential(ctx, domain.AccountPasskeyCredential(credential))
	return identityaccess.AccountPasskeyCredential(value), mapError(err)
}

func (e engine) RecordAccountPasskeyUse(ctx context.Context, id int64, credentialJSON json.RawMessage, usedAt time.Time) error {
	return mapError(e.accountPasskeys.RecordUse(ctx, id, credentialJSON, usedAt))
}

func (e engine) RevokeAccountPasskey(ctx context.Context, accountID, id int64, revokedAt time.Time) error {
	return mapError(e.accountPasskeys.RevokeCredential(ctx, accountID, id, revokedAt))
}

func (e engine) CreateAccountPasskeySession(ctx context.Context, session identityaccess.AccountPasskeySession) (identityaccess.AccountPasskeySession, error) {
	value, err := e.accountPasskeys.CreateSession(ctx, domain.AccountPasskeySession(session))
	return identityaccess.AccountPasskeySession(value), mapError(err)
}

func (e engine) ConsumeAccountPasskeySession(ctx context.Context, id, purpose string, consumedAt time.Time) (identityaccess.AccountPasskeySession, error) {
	value, err := e.accountPasskeys.ConsumeSession(ctx, id, purpose, consumedAt)
	return identityaccess.AccountPasskeySession(value), mapError(err)
}
