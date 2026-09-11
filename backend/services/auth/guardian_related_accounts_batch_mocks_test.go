package auth_test

import (
	"context"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// These adapters keep the narrow error-path doubles aligned with the batch
// reads used by ListPendingApprovalsDetailed without changing their scenarios.
func (m mockProfileRepo) FindByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.GuardianProfile, error) {
	result := make(map[int64]*userModels.GuardianProfile, len(ids))
	for _, id := range ids {
		profile, err := m.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if profile != nil {
			result[id] = profile
		}
	}
	return result, nil
}

func (m mockStudentRepo) FindByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Student, error) {
	result := make(map[int64]*userModels.Student, len(ids))
	for _, id := range ids {
		student, err := m.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if student != nil {
			result[id] = student
		}
	}
	return result, nil
}

func (m mockAccountRepo) FindEmailsByAccountIDs(ctx context.Context, ids []int64) (map[int64]string, error) {
	result := make(map[int64]string, len(ids))
	for _, id := range ids {
		account, err := m.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if account != nil {
			result[id] = account.Email
		}
	}
	return result, nil
}

var _ authModels.AccountRepository = mockAccountRepo{}
