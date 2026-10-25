package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// The database keeps session, invitation, password-reset and e-mail-change
// tokens only as their hex SHA-256 (#4902): a reader of the database or a
// backup must not be able to take over a session or redeem a link. The raw
// value lives only in the mail link or the refresh JWT that carries it.

// HashToken returns the hex-encoded SHA-256 of a raw token, the value the
// token columns store and the lookups compare. A plain digest is enough
// because every such token carries at least 122 bits of randomness: there is
// no offline-guessing surface.
func HashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

const successorRefreshTokenLabel = "moto refresh-token successor"

// SuccessorRefreshToken returns the raw token of the session that replaces
// the one presented with rawToken. Rotation stores only hashes, so a lost
// rotation response can be recovered only by recomputing the successor from
// the token the client presents again; the predecessor's stored hash does not
// yield it. HMAC keyed with the raw token keeps the successor as
// unpredictable as its predecessor.
func SuccessorRefreshToken(rawToken string) string {
	mac := hmac.New(sha256.New, []byte(rawToken))
	mac.Write([]byte(successorRefreshTokenLabel))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
