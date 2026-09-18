package compose

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Engine methods for the operator passkey records (#2724). They need no
// dependencies beyond the database, so every composition serves them.

func (e engine) ListActiveOperatorPasskeys(ctx context.Context, operatorID int64) ([]identityaccess.OperatorPasskeyCredential, error) {
	values, err := e.passkeys.ListActiveCredentials(ctx, operatorID)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]identityaccess.OperatorPasskeyCredential, 0, len(values))
	for _, value := range values {
		result = append(result, identityaccess.OperatorPasskeyCredential(value))
	}
	return result, nil
}

func (e engine) FindActiveOperatorPasskey(ctx context.Context, credentialID, userHandle []byte) (identityaccess.OperatorPasskeyCredential, error) {
	value, err := e.passkeys.FindActiveCredential(ctx, credentialID, userHandle)
	return identityaccess.OperatorPasskeyCredential(value), mapError(err)
}

func (e engine) CreateOperatorPasskey(ctx context.Context, credential identityaccess.OperatorPasskeyCredential) (identityaccess.OperatorPasskeyCredential, error) {
	value, err := e.passkeys.CreateCredential(ctx, domain.OperatorPasskeyCredential(credential))
	return identityaccess.OperatorPasskeyCredential(value), mapError(err)
}

func (e engine) RecordOperatorPasskeyUse(ctx context.Context, id int64, credentialJSON json.RawMessage, usedAt time.Time) error {
	return mapError(e.passkeys.RecordUse(ctx, id, credentialJSON, usedAt))
}

func (e engine) RevokeOperatorPasskey(ctx context.Context, operatorID, id int64, revokedAt time.Time) error {
	return mapError(e.passkeys.RevokeCredential(ctx, operatorID, id, revokedAt))
}

func (e engine) CreateOperatorPasskeySession(ctx context.Context, session identityaccess.OperatorPasskeySession) (identityaccess.OperatorPasskeySession, error) {
	value, err := e.passkeys.CreateSession(ctx, domain.OperatorPasskeySession(session))
	return identityaccess.OperatorPasskeySession(value), mapError(err)
}

func (e engine) ConsumeOperatorPasskeySession(ctx context.Context, id, purpose string, consumedAt time.Time) (identityaccess.OperatorPasskeySession, error) {
	value, err := e.passkeys.ConsumeSession(ctx, id, purpose, consumedAt)
	return identityaccess.OperatorPasskeySession(value), mapError(err)
}
