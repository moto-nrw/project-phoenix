package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// Completing a passkey ceremony writes twice: it consumes the ceremony and
// then stores the credential or its use. Both portals run that pair in one
// administrative transaction under the same rule, so the rule lives here and
// both flows use it.

// ceremonyRejection marks a ceremony the verification refused.
type ceremonyRejection struct{ cause error }

func (r ceremonyRejection) Error() string { return r.cause.Error() }

func (r ceremonyRejection) Unwrap() error { return r.cause }

// rejectCeremony marks cause as a refusal of the ceremony rather than a
// failure of the request: the consumption commits and cause reaches the
// caller unchanged.
func rejectCeremony(cause error) error { return ceremonyRejection{cause: cause} }

// completeCeremony runs a ceremony completion in one administrative
// transaction. A rejection commits: the consumed ceremony stays spent, so
// the same challenge cannot be tried again, and the caller receives the
// rejection's cause. Any other failure rolls back, so the ceremony can be
// completed again after a failed read or write.
func completeCeremony(ctx context.Context, runtime ports.Runtime, fn func(context.Context) error) error {
	var rejected error
	err := runtime.WithAdminTx(ctx, func(txCtx context.Context) error {
		rejected = nil
		err := fn(txCtx)
		var rejection ceremonyRejection
		if errors.As(err, &rejection) {
			rejected = rejection.cause
			return nil
		}
		return err
	})
	if err != nil {
		return err
	}
	return rejected
}

// webAuthnUser adapts an account or an operator to the go-webauthn user
// interface. ID is the account or operator primary key, read by the flows
// outside the webauthn.User method set.
type webAuthnUser struct {
	ID          int64
	UserHandle  []byte
	Name        string
	DisplayName string
	Credentials []webauthn.Credential
}

func (u *webAuthnUser) WebAuthnID() []byte                         { return u.UserHandle }
func (u *webAuthnUser) WebAuthnName() string                       { return u.Name }
func (u *webAuthnUser) WebAuthnDisplayName() string                { return u.DisplayName }
func (u *webAuthnUser) WebAuthnCredentials() []webauthn.Credential { return u.Credentials }

// newWebAuthnForOrigin builds a relying party scoped to a single origin with
// the ceremony timeouts both portals share.
func newWebAuthnForOrigin(rpID, rpName, origin string) (*webauthn.WebAuthn, error) {
	return webauthn.New(&webauthn.Config{
		RPID:          rpID,
		RPDisplayName: rpName,
		RPOrigins:     []string{origin},
		Timeouts: webauthn.TimeoutsConfig{
			Login: webauthn.TimeoutConfig{
				Enforce:    true,
				Timeout:    domain.PasskeyCeremonyTimeout,
				TimeoutUVD: domain.PasskeyCeremonyTimeout,
			},
			Registration: webauthn.TimeoutConfig{
				Enforce:    true,
				Timeout:    domain.PasskeyCeremonyTimeout,
				TimeoutUVD: domain.PasskeyCeremonyTimeout,
			},
		},
	})
}

// parsePasskeyCreation and parsePasskeyAssertion read the browser's
// credential response. The library also offers request-shaped entry points;
// the flows parse the body directly so the application layer stays free of
// HTTP.
func parsePasskeyCreation(raw json.RawMessage) (*protocol.ParsedCredentialCreationData, error) {
	if len(raw) == 0 || !json.Valid(raw) {
		return nil, domain.ErrPasskeySessionInvalid
	}
	return protocol.ParseCredentialCreationResponseBody(bytes.NewReader(raw))
}

func parsePasskeyAssertion(raw json.RawMessage) (*protocol.ParsedCredentialAssertionData, error) {
	if len(raw) == 0 || !json.Valid(raw) {
		return nil, domain.ErrPasskeySessionInvalid
	}
	return protocol.ParseCredentialRequestResponseBody(bytes.NewReader(raw))
}

// passkeyCredentials decodes the stored credential state of one identity and
// reports the user handle its credentials share. A ceremony for an identity
// without credentials falls back to the handle the session carried and mints
// a fresh one otherwise.
func passkeyCredentials(stored []json.RawMessage, handles [][]byte, sessionHandle []byte) ([]webauthn.Credential, []byte, error) {
	credentials := make([]webauthn.Credential, 0, len(stored))
	var userHandle []byte
	for i, raw := range stored {
		if len(userHandle) == 0 {
			userHandle = handles[i]
		}
		var credential webauthn.Credential
		if err := json.Unmarshal(raw, &credential); err != nil {
			return nil, nil, err
		}
		credentials = append(credentials, credential)
	}
	if len(userHandle) > 0 {
		return credentials, userHandle, nil
	}
	if len(sessionHandle) > 0 {
		return credentials, sessionHandle, nil
	}
	handle, err := domain.GeneratePasskeyUserHandle()
	if err != nil {
		return nil, nil, err
	}
	return credentials, handle, nil
}
