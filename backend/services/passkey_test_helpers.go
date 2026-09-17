package services

import (
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/uptrace/bun"
)

// NewPasskeyRecordsForTests serves the retained school-portal passkey
// records port over the Identity & Access module composed for repository
// fixtures, the way the service root binds it (#2724).
func NewPasskeyRecordsForTests(db *bun.DB) (auth.PasskeyRecords, error) {
	module, err := repositories.NewIdentityAccessForTests(db)
	if err != nil {
		return nil, err
	}
	return newAccountPasskeyRecords(module), nil
}
