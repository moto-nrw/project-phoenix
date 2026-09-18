package auth

import "errors"

// The account registration and school linking flows live in Identity &
// Access (#3332) and the registration routes call the public capability
// directly. The sentinels below are what remains here: the operator
// provisioning routes of Organisation & Tenancy classify a refused account
// creation on them and may not name the owner's contract, and the retained
// parent account flows still report them. The composition root translates
// the public outcomes into these; their texts are the wire contract.
var (
	// ErrEmailAlreadyExists reports an address that is already registered.
	ErrEmailAlreadyExists = errors.New("Diese E-Mail-Adresse ist bereits registriert") //nolint:staticcheck // ST1005: user-facing German message

	// ErrUsernameAlreadyExists reports a name that is already taken.
	ErrUsernameAlreadyExists = errors.New("Dieser Benutzername ist bereits vergeben") //nolint:staticcheck // ST1005: user-facing German message
)
