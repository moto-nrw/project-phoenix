package parent_test

import (
	"context"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

func (s *stubGuardianProfileRepo) FindByEmails(_ context.Context, emails []string) (map[string]*userModels.GuardianProfile, error) {
	return make(map[string]*userModels.GuardianProfile, len(emails)), nil
}
