package users

import (
	context "context"

	models "github.com/moto-nrw/project-phoenix/models/users"
	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	ports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

type nativeGuardianPhoneNumberRepository struct {
	rawGuardianPhoneNumberRepository
}

func NewGuardianPhoneNumberRepository(runtime Runtime) ports.GuardianPhoneNumberRepository {
	return nativeGuardianPhoneNumberRepository{rawGuardianPhoneNumberRepository: newRawGuardianPhoneNumberRepository(runtime)}
}

func (r nativeGuardianPhoneNumberRepository) Create(ctx context.Context, phone *directory.GuardianProfilePhoneSnapshot) error {
	rawPhone := GuardianPhoneRow(phone)
	out0 := r.rawGuardianPhoneNumberRepository.Create(ctx, rawPhone)
	if phone != nil && rawPhone != nil {
		*phone = *NativeGuardianPhone(rawPhone)
	}
	return out0
}

func (r nativeGuardianPhoneNumberRepository) FindByID(ctx context.Context, id int64) (*directory.GuardianProfilePhoneSnapshot, error) {

	out0, out1 := r.rawGuardianPhoneNumberRepository.FindByID(ctx, id)

	return NativeGuardianPhone(out0), out1
}

func (r nativeGuardianPhoneNumberRepository) FindByGuardianID(ctx context.Context, guardianProfileID int64) ([]*directory.GuardianProfilePhoneSnapshot, error) {

	out0, out1 := r.rawGuardianPhoneNumberRepository.FindByGuardianID(ctx, guardianProfileID)

	return mapRecordSlice(out0, func(value *models.GuardianPhoneNumber) *directory.GuardianProfilePhoneSnapshot {
		return NativeGuardianPhone(value)
	}), out1
}

func (r nativeGuardianPhoneNumberRepository) FindByGuardianIDs(ctx context.Context, guardianProfileIDs []int64) (map[int64][]*directory.GuardianProfilePhoneSnapshot, error) {

	out0, out1 := r.rawGuardianPhoneNumberRepository.FindByGuardianIDs(ctx, guardianProfileIDs)

	return mapRecordValues(out0, func(value []*models.GuardianPhoneNumber) []*directory.GuardianProfilePhoneSnapshot {
		return mapRecordSlice(value, func(value *models.GuardianPhoneNumber) *directory.GuardianProfilePhoneSnapshot {
			return NativeGuardianPhone(value)
		})
	}), out1
}

func (r nativeGuardianPhoneNumberRepository) Update(ctx context.Context, phone *directory.GuardianProfilePhoneSnapshot) error {
	rawPhone := GuardianPhoneRow(phone)
	out0 := r.rawGuardianPhoneNumberRepository.Update(ctx, rawPhone)
	if phone != nil && rawPhone != nil {
		*phone = *NativeGuardianPhone(rawPhone)
	}
	return out0
}
