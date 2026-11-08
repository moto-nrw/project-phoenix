package sessiontoken

// RecoveryProofHeader carries an independent, high-entropy secret that is
// kept only in the encrypted, HTTP-only Auth.js session cookie and sent on
// the server-to-server refresh request. It is deliberately not an access or
// refresh token. The backend stores only its SHA-256 digest and never logs
// either value. The frontend sends this exact name.
const RecoveryProofHeader = "X-Refresh-Recovery-Proof"
