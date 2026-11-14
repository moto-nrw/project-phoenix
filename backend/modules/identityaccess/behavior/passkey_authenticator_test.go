package behavior_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// passkeyAuthenticator is a software WebAuthn authenticator holding one
// discoverable ES256 credential. It answers the options a ceremony hands the
// browser the way a platform authenticator does (attestation "none", user
// present and verified), so a test can complete the real ceremonies.
//
// A test may change CredentialID and UserHandle to present what a
// misbehaving client would: the same credential a second time, or a
// credential under another identity's handle.
type passkeyAuthenticator struct {
	CredentialID []byte
	UserHandle   []byte
	key          *ecdsa.PrivateKey
	signCount    uint32
}

// newPasskeyAuthenticator creates an authenticator with a fresh key pair and
// credential ID.
func newPasskeyAuthenticator(tb testing.TB) *passkeyAuthenticator {
	tb.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(tb, err)
	credentialID := make([]byte, 16)
	_, err = rand.Read(credentialID)
	require.NoError(tb, err)
	return &passkeyAuthenticator{CredentialID: credentialID, key: key}
}

const (
	passkeyFlagUserPresent  = 0x01
	passkeyFlagUserVerified = 0x04
	passkeyFlagAttestedData = 0x40
)

// Attest answers registration options: it adopts the user handle the relying
// party named and returns the credential response of navigator.credentials.create.
func (a *passkeyAuthenticator) Attest(tb testing.TB, options any, origin string) json.RawMessage {
	tb.Helper()
	var creation struct {
		PublicKey struct {
			Challenge string `json:"challenge"`
			RP        struct {
				ID string `json:"id"`
			} `json:"rp"`
			User struct {
				ID string `json:"id"`
			} `json:"user"`
		} `json:"publicKey"`
	}
	decodePasskeyOptions(tb, options, &creation)
	handle, err := base64.RawURLEncoding.DecodeString(creation.PublicKey.User.ID)
	require.NoError(tb, err)
	a.UserHandle = handle

	authData := a.authenticatorData(creation.PublicKey.RP.ID, passkeyFlagUserPresent|passkeyFlagUserVerified|passkeyFlagAttestedData)
	authData = append(authData, make([]byte, 16)...) // AAGUID
	authData = binary.BigEndian.AppendUint16(authData, uint16(len(a.CredentialID)))
	authData = append(authData, a.CredentialID...)
	authData = append(authData, a.coseKey()...)

	// CBOR map {"fmt": "none", "attStmt": {}, "authData": bytes}.
	attestation := []byte{0xa3, 0x63, 'f', 'm', 't', 0x64, 'n', 'o', 'n', 'e', 0x67, 'a', 't', 't', 'S', 't', 'm', 't', 0xa0, 0x68, 'a', 'u', 't', 'h', 'D', 'a', 't', 'a', 0x59}
	attestation = binary.BigEndian.AppendUint16(attestation, uint16(len(authData)))
	attestation = append(attestation, authData...)

	return a.credentialResponse(tb, map[string]string{
		"clientDataJSON":    passkeyClientData(tb, "webauthn.create", creation.PublicKey.Challenge, origin),
		"attestationObject": base64.RawURLEncoding.EncodeToString(attestation),
	})
}

// Assert answers login options with a signed assertion of the credential,
// under UserHandle, and advances the signature counter.
func (a *passkeyAuthenticator) Assert(tb testing.TB, options any, origin string) json.RawMessage {
	tb.Helper()
	var request struct {
		PublicKey struct {
			Challenge string `json:"challenge"`
			RPID      string `json:"rpId"`
		} `json:"publicKey"`
	}
	decodePasskeyOptions(tb, options, &request)
	a.signCount++
	authData := a.authenticatorData(request.PublicKey.RPID, passkeyFlagUserPresent|passkeyFlagUserVerified)
	clientData := passkeyClientData(tb, "webauthn.get", request.PublicKey.Challenge, origin)
	rawClientData, err := base64.RawURLEncoding.DecodeString(clientData)
	require.NoError(tb, err)
	clientDataHash := sha256.Sum256(rawClientData)
	digest := sha256.Sum256(append(append([]byte(nil), authData...), clientDataHash[:]...))
	signature, err := ecdsa.SignASN1(rand.Reader, a.key, digest[:])
	require.NoError(tb, err)

	return a.credentialResponse(tb, map[string]string{
		"clientDataJSON":    clientData,
		"authenticatorData": base64.RawURLEncoding.EncodeToString(authData),
		"signature":         base64.RawURLEncoding.EncodeToString(signature),
		"userHandle":        base64.RawURLEncoding.EncodeToString(a.UserHandle),
	})
}

func (a *passkeyAuthenticator) authenticatorData(rpID string, flags byte) []byte {
	rpIDHash := sha256.Sum256([]byte(rpID))
	data := append(rpIDHash[:], flags)
	return binary.BigEndian.AppendUint32(data, a.signCount)
}

// coseKey is the public key as a COSE EC2 map:
// {1: 2 (EC2), 3: -7 (ES256), -1: 1 (P-256), -2: x, -3: y}.
func (a *passkeyAuthenticator) coseKey() []byte {
	x := a.key.X.FillBytes(make([]byte, 32))
	y := a.key.Y.FillBytes(make([]byte, 32))
	key := []byte{0xa5, 0x01, 0x02, 0x03, 0x26, 0x20, 0x01, 0x21, 0x58, 0x20}
	key = append(key, x...)
	key = append(key, 0x22, 0x58, 0x20)
	return append(key, y...)
}

func (a *passkeyAuthenticator) credentialResponse(tb testing.TB, response map[string]string) json.RawMessage {
	tb.Helper()
	id := base64.RawURLEncoding.EncodeToString(a.CredentialID)
	raw, err := json.Marshal(map[string]any{"id": id, "rawId": id, "type": "public-key", "response": response})
	require.NoError(tb, err)
	return raw
}

func decodePasskeyOptions(tb testing.TB, options, into any) {
	tb.Helper()
	raw, err := json.Marshal(options)
	require.NoError(tb, err)
	require.NoError(tb, json.Unmarshal(raw, into))
}

func passkeyClientData(tb testing.TB, ceremony, challenge, origin string) string {
	tb.Helper()
	raw, err := json.Marshal(map[string]string{"type": ceremony, "challenge": challenge, "origin": origin})
	require.NoError(tb, err)
	return base64.RawURLEncoding.EncodeToString(raw)
}
