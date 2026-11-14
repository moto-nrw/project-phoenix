package identityaccess

// Operator passkey records (#2724): the WebAuthn credentials an operator
// registered and the server-side state of the operator portal's passkey
// ceremonies. The rows are platform-wide like the operator itself. Every
// operation joins the caller's transaction when one is active.
var (
	// ErrOperatorPasskeyNotFound reports a lookup without an active
	// credential, or a use or revocation of a credential that is revoked,
	// belongs to another operator or is gone.
	ErrOperatorPasskeyNotFound = notFoundError("operator passkey not found")
	// ErrOperatorPasskeySessionNotFound reports a consumption that found no
	// unconsumed, unexpired ceremony with that ID and purpose. The loser of
	// two concurrent completions receives it.
	ErrOperatorPasskeySessionNotFound = notFoundError("operator passkey session not found")
)
