package services

import (
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/uptrace/bun"
)

// NewOperatorDirectoryForTests serves the retained operator directory port
// over the Identity & Access module composed for repository fixtures, so the
// retained MFA, passkey, invitation and e-mail change tests reach operator
// rows the way the service root binds them (#3252).
func NewOperatorDirectoryForTests(db *bun.DB) (platform.OperatorDirectory, error) {
	module, err := repositories.NewIdentityAccessForTests(db)
	if err != nil {
		return nil, err
	}
	return newOperatorDirectory(module), nil
}

// NewOperatorMFARecordsForTests serves the retained operator MFA records
// port over the Identity & Access module composed for repository fixtures,
// the way the service root binds it (#2723).
func NewOperatorMFARecordsForTests(db *bun.DB) (platform.OperatorMFARecords, error) {
	module, err := repositories.NewIdentityAccessForTests(db)
	if err != nil {
		return nil, err
	}
	return newOperatorMFARecords(module), nil
}

// NewOperatorInvitationTokensForTests serves the retained operator
// invitation link port over the Identity & Access module composed for
// repository fixtures, the way the service root binds it (#2722).
func NewOperatorInvitationTokensForTests(db *bun.DB) (platform.OperatorInvitationTokens, error) {
	module, err := repositories.NewIdentityAccessForTests(db)
	if err != nil {
		return nil, err
	}
	return newOperatorInvitationTokens(module), nil
}

// NewOperatorEmailChangeTokensForTests serves the retained operator e-mail
// change link port the same way (#2722).
func NewOperatorEmailChangeTokensForTests(db *bun.DB) (platform.OperatorEmailChangeTokens, error) {
	module, err := repositories.NewIdentityAccessForTests(db)
	if err != nil {
		return nil, err
	}
	return newOperatorEmailChangeTokens(module), nil
}

// NewOperatorPasskeyRecordsForTests serves the retained operator passkey
// records port over the Identity & Access module composed for repository
// fixtures, the way the service root binds it (#2724).
func NewOperatorPasskeyRecordsForTests(db *bun.DB) (platform.OperatorPasskeyRecords, error) {
	module, err := repositories.NewIdentityAccessForTests(db)
	if err != nil {
		return nil, err
	}
	return newOperatorPasskeyRecords(module), nil
}
