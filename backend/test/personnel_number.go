package test

import (
	"context"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

func PersonnelNumberCounter(repo userModels.StaffRepository) func(context.Context) (int, int, error) {
	return func(ctx context.Context) (int, int, error) {
		staff, err := repo.List(ctx, nil)
		if err != nil {
			return 0, 0, err
		}
		withoutPersonnelNumber := 0
		for _, member := range staff {
			if member.PersonnelNumber == nil || *member.PersonnelNumber == "" {
				withoutPersonnelNumber++
			}
		}
		return len(staff), withoutPersonnelNumber, nil
	}
}
