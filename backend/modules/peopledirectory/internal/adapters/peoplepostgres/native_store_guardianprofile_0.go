package users

import (
	context "context"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	models "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
	ports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

type nativeGuardianProfileRepository struct{ rawGuardianProfileRepository }

func NewGuardianProfileRepository(runtime Runtime, students StudentDirectoryRows, links GuardianLinkRows, options ...GuardianProfileOption) ports.GuardianProfileRepository {
	return nativeGuardianProfileRepository{rawGuardianProfileRepository: newRawGuardianProfileRepository(runtime, students, links, options...)}
}

func (r nativeGuardianProfileRepository) Create(ctx context.Context, profile *directory.GuardianProfileSnapshot) error {
	rawProfile := GuardianProfileRow(profile)
	out0 := r.rawGuardianProfileRepository.Create(ctx, rawProfile)
	if profile != nil && rawProfile != nil {
		*profile = *NativeGuardianProfile(rawProfile)
	}
	return out0
}

func (r nativeGuardianProfileRepository) FindByID(ctx context.Context, id int64) (*directory.GuardianProfileSnapshot, error) {

	out0, out1 := r.rawGuardianProfileRepository.FindByID(ctx, id)

	return NativeGuardianProfile(out0), out1
}

func (r nativeGuardianProfileRepository) FindByEmail(ctx context.Context, email string) (*directory.GuardianProfileSnapshot, error) {

	out0, out1 := r.rawGuardianProfileRepository.FindByEmail(ctx, email)

	return NativeGuardianProfile(out0), out1
}

func (r nativeGuardianProfileRepository) FindByAccountID(ctx context.Context, accountID int64) (*directory.GuardianProfileSnapshot, error) {

	out0, out1 := r.rawGuardianProfileRepository.FindByAccountID(ctx, accountID)

	return NativeGuardianProfile(out0), out1
}

func (r nativeGuardianProfileRepository) FindWithoutAccount(ctx context.Context) ([]*directory.GuardianProfileSnapshot, error) {

	out0, out1 := r.rawGuardianProfileRepository.FindWithoutAccount(ctx)

	return mapRecordSlice(out0, func(value *models.GuardianProfile) *directory.GuardianProfileSnapshot {
		return NativeGuardianProfile(value)
	}), out1
}

func (r nativeGuardianProfileRepository) FindInvitable(ctx context.Context) ([]*directory.GuardianProfileSnapshot, error) {

	out0, out1 := r.rawGuardianProfileRepository.FindInvitable(ctx)

	return mapRecordSlice(out0, func(value *models.GuardianProfile) *directory.GuardianProfileSnapshot {
		return NativeGuardianProfile(value)
	}), out1
}

func (r nativeGuardianProfileRepository) ListWithOptions(ctx context.Context, options *ports.QueryOptions) ([]*directory.GuardianProfileSnapshot, error) {

	out0, out1 := r.rawGuardianProfileRepository.ListWithOptions(ctx, options)

	return mapRecordSlice(out0, func(value *models.GuardianProfile) *directory.GuardianProfileSnapshot {
		return NativeGuardianProfile(value)
	}), out1
}

func (r nativeGuardianProfileRepository) FindByEmails(ctx context.Context, emails []string) ([]*directory.GuardianProfileSnapshot, error) {

	out0, out1 := r.rawGuardianProfileRepository.FindByEmails(ctx, emails)

	return mapRecordSlice(out0, func(value *models.GuardianProfile) *directory.GuardianProfileSnapshot {
		return NativeGuardianProfile(value)
	}), out1
}

func (r nativeGuardianProfileRepository) SearchByText(ctx context.Context, searchText string, limit int) ([]*directory.GuardianProfileSnapshot, error) {

	out0, out1 := r.rawGuardianProfileRepository.SearchByText(ctx, searchText, limit)

	return mapRecordSlice(out0, func(value *models.GuardianProfile) *directory.GuardianProfileSnapshot {
		return NativeGuardianProfile(value)
	}), out1
}

func (r nativeGuardianProfileRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*directory.GuardianProfileSnapshot, error) {

	out0, out1 := r.rawGuardianProfileRepository.FindByIDs(ctx, ids)

	return mapRecordValues(out0, func(value *models.GuardianProfile) *directory.GuardianProfileSnapshot {
		return NativeGuardianProfile(value)
	}), out1
}

func (r nativeGuardianProfileRepository) FindActivePortalProfilesByIDs(ctx context.Context, ids []int64) (map[int64]*directory.GuardianProfileSnapshot, error) {

	out0, out1 := r.rawGuardianProfileRepository.FindActivePortalProfilesByIDs(ctx, ids)

	return mapRecordValues(out0, func(value *models.GuardianProfile) *directory.GuardianProfileSnapshot {
		return NativeGuardianProfile(value)
	}), out1
}

func (r nativeGuardianProfileRepository) FindLoginReadyPortalProfilesByIDs(ctx context.Context, ids []int64) (map[int64]*directory.GuardianProfileSnapshot, error) {

	out0, out1 := r.rawGuardianProfileRepository.FindLoginReadyPortalProfilesByIDs(ctx, ids)

	return mapRecordValues(out0, func(value *models.GuardianProfile) *directory.GuardianProfileSnapshot {
		return NativeGuardianProfile(value)
	}), out1
}

func (r nativeGuardianProfileRepository) Update(ctx context.Context, profile *directory.GuardianProfileSnapshot) error {
	rawProfile := GuardianProfileRow(profile)
	out0 := r.rawGuardianProfileRepository.Update(ctx, rawProfile)
	if profile != nil && rawProfile != nil {
		*profile = *NativeGuardianProfile(rawProfile)
	}
	return out0
}

func (r nativeGuardianProfileRepository) LoadProfileWithChildren(ctx context.Context, accountID int64) (*directory.GuardianProfileWithChildren, error) {

	out0, out1 := r.rawGuardianProfileRepository.LoadProfileWithChildren(ctx, accountID)

	return NativeGuardianProfileWithChildren(out0), out1
}
