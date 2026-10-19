package users

import (
	context "context"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	models "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
	ports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

type nativePersonRepository struct{ rawPersonRepository }

func NewPersonRepository(runtime Runtime, options ...PersonOption) ports.PersonRepository {
	return nativePersonRepository{rawPersonRepository: newRawPersonRepository(runtime, options...)}
}

func (r nativePersonRepository) Create(ctx context.Context, entity *directory.DirectoryPersonSnapshot) error {
	rawEntity := DirectoryPersonRow(entity)
	out0 := r.rawPersonRepository.Create(ctx, rawEntity)
	if entity != nil && rawEntity != nil {
		*entity = *NativeDirectoryPerson(rawEntity)
	}
	return out0
}

func (r nativePersonRepository) FindByID(ctx context.Context, id any) (*directory.DirectoryPersonSnapshot, error) {

	out0, out1 := r.rawPersonRepository.FindByID(ctx, id)

	return NativeDirectoryPerson(out0), out1
}

func (r nativePersonRepository) Update(ctx context.Context, entity *directory.DirectoryPersonSnapshot) error {
	rawEntity := DirectoryPersonRow(entity)
	out0 := r.rawPersonRepository.Update(ctx, rawEntity)
	if entity != nil && rawEntity != nil {
		*entity = *NativeDirectoryPerson(rawEntity)
	}
	return out0
}

func (r nativePersonRepository) List(ctx context.Context, filters map[string]any) ([]*directory.DirectoryPersonSnapshot, error) {

	out0, out1 := r.rawPersonRepository.List(ctx, filters)

	return mapRecordSlice(out0, func(value *models.Person) *directory.DirectoryPersonSnapshot { return NativeDirectoryPerson(value) }), out1
}

func (r nativePersonRepository) FindByIDForUpdate(ctx context.Context, id int64) (*directory.DirectoryPersonSnapshot, error) {

	out0, out1 := r.rawPersonRepository.FindByIDForUpdate(ctx, id)

	return NativeDirectoryPerson(out0), out1
}

func (r nativePersonRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*directory.DirectoryPersonSnapshot, error) {

	out0, out1 := r.rawPersonRepository.FindByIDs(ctx, ids)

	return mapRecordValues(out0, func(value *models.Person) *directory.DirectoryPersonSnapshot { return NativeDirectoryPerson(value) }), out1
}

func (r nativePersonRepository) FindByTagID(ctx context.Context, tagID string) (*directory.DirectoryPersonSnapshot, error) {

	out0, out1 := r.rawPersonRepository.FindByTagID(ctx, tagID)

	return NativeDirectoryPerson(out0), out1
}

func (r nativePersonRepository) FindByAccountID(ctx context.Context, accountID int64) (*directory.DirectoryPersonSnapshot, error) {

	out0, out1 := r.rawPersonRepository.FindByAccountID(ctx, accountID)

	return NativeDirectoryPerson(out0), out1
}

func (r nativePersonRepository) FindByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]*directory.DirectoryPersonSnapshot, error) {

	out0, out1 := r.rawPersonRepository.FindByAccountIDs(ctx, accountIDs)

	return mapRecordValues(out0, func(value *models.Person) *directory.DirectoryPersonSnapshot { return NativeDirectoryPerson(value) }), out1
}

func (r nativePersonRepository) ListWithOptions(ctx context.Context, options *ports.QueryOptions) ([]*directory.DirectoryPersonSnapshot, error) {

	out0, out1 := r.rawPersonRepository.ListWithOptions(ctx, options)

	return mapRecordSlice(out0, func(value *models.Person) *directory.DirectoryPersonSnapshot { return NativeDirectoryPerson(value) }), out1
}

func (r nativePersonRepository) FindWithAccount(ctx context.Context, id int64) (*directory.DirectoryPersonSnapshot, error) {

	out0, out1 := r.rawPersonRepository.FindWithAccount(ctx, id)

	return NativeDirectoryPerson(out0), out1
}
