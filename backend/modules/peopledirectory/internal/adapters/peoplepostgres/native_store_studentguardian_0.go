package users

import (
	context "context"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	models "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
	ports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

type nativeStudentGuardianRepository struct{ rawStudentGuardianRepository }

func NewGuardianRelationshipRepository(runtime Runtime, students StudentDirectoryRows, links GuardianLinkRows, options ...GuardianRelationshipOption) ports.StudentGuardianRepository {
	return nativeStudentGuardianRepository{rawStudentGuardianRepository: newRawGuardianRelationshipRepository(runtime, students, links, options...)}
}

func (r nativeStudentGuardianRepository) Create(ctx context.Context, relationship *directory.GuardianRelationshipSnapshot) error {
	rawRelationship, decodeErr := GuardianRelationshipRow(relationship)
	if decodeErr != nil {
		return decodeErr
	}
	out0 := r.rawStudentGuardianRepository.Create(ctx, rawRelationship)
	if relationship != nil && rawRelationship != nil {
		*relationship = *NativeGuardianRelationshipValue(rawRelationship)
	}
	return out0
}

func (r nativeStudentGuardianRepository) FindByID(ctx context.Context, id any) (*directory.GuardianRelationshipSnapshot, error) {

	out0, out1 := r.rawStudentGuardianRepository.FindByID(ctx, id)

	return NativeGuardianRelationshipValue(out0), out1
}

func (r nativeStudentGuardianRepository) Update(ctx context.Context, relationship *directory.GuardianRelationshipSnapshot) error {
	rawRelationship, decodeErr := GuardianRelationshipRow(relationship)
	if decodeErr != nil {
		return decodeErr
	}
	out0 := r.rawStudentGuardianRepository.Update(ctx, rawRelationship)
	if relationship != nil && rawRelationship != nil {
		*relationship = *NativeGuardianRelationshipValue(rawRelationship)
	}
	return out0
}

func (r nativeStudentGuardianRepository) List(ctx context.Context, filters map[string]any) ([]*directory.GuardianRelationshipSnapshot, error) {

	out0, out1 := r.rawStudentGuardianRepository.List(ctx, filters)

	return mapRecordSlice(out0, func(value *models.StudentGuardian) *directory.GuardianRelationshipSnapshot {
		return NativeGuardianRelationshipValue(value)
	}), out1
}

func (r nativeStudentGuardianRepository) ListEmergencyContactRows(ctx context.Context, studentIDs []int64) ([]directory.GuardianEmergencyContactRow, error) {

	out0, out1 := r.rawStudentGuardianRepository.ListEmergencyContactRows(ctx, studentIDs)

	return mapRecordSlice(out0, func(value models.GuardianEmergencyContactRow) directory.GuardianEmergencyContactRow {
		return *NativeGuardianEmergencyContactRow(&value)
	}), out1
}

func (r nativeStudentGuardianRepository) FindByStudentID(ctx context.Context, studentID int64) ([]*directory.GuardianRelationshipSnapshot, error) {

	out0, out1 := r.rawStudentGuardianRepository.FindByStudentID(ctx, studentID)

	return mapRecordSlice(out0, func(value *models.StudentGuardian) *directory.GuardianRelationshipSnapshot {
		return NativeGuardianRelationshipValue(value)
	}), out1
}

func (r nativeStudentGuardianRepository) FindByStudentIDs(ctx context.Context, studentIDs []int64) ([]*directory.GuardianRelationshipSnapshot, error) {

	out0, out1 := r.rawStudentGuardianRepository.FindByStudentIDs(ctx, studentIDs)

	return mapRecordSlice(out0, func(value *models.StudentGuardian) *directory.GuardianRelationshipSnapshot {
		return NativeGuardianRelationshipValue(value)
	}), out1
}

func (r nativeStudentGuardianRepository) FindByGuardianProfileID(ctx context.Context, guardianProfileID int64) ([]*directory.GuardianRelationshipSnapshot, error) {

	out0, out1 := r.rawStudentGuardianRepository.FindByGuardianProfileID(ctx, guardianProfileID)

	return mapRecordSlice(out0, func(value *models.StudentGuardian) *directory.GuardianRelationshipSnapshot {
		return NativeGuardianRelationshipValue(value)
	}), out1
}

func (r nativeStudentGuardianRepository) FindByStudentAndGuardianForUpdate(ctx context.Context, studentID int64, guardianProfileID int64) (*directory.GuardianRelationshipSnapshot, error) {

	out0, out1 := r.rawStudentGuardianRepository.FindByStudentAndGuardianForUpdate(ctx, studentID, guardianProfileID)

	return NativeGuardianRelationshipValue(out0), out1
}

func (r nativeStudentGuardianRepository) LinkIfNotExists(ctx context.Context, rel *directory.GuardianRelationshipSnapshot) (bool, error) {
	rawRel, decodeErr := GuardianRelationshipRow(rel)
	if decodeErr != nil {
		return false, decodeErr
	}
	out0, out1 := r.rawStudentGuardianRepository.LinkIfNotExists(ctx, rawRel)
	if rel != nil && rawRel != nil {
		*rel = *NativeGuardianRelationshipValue(rawRel)
	}
	return out0, out1
}

func (r nativeStudentGuardianRepository) UpdateColumns(ctx context.Context, relationship *directory.GuardianRelationshipSnapshot, columns ...string) (int64, error) {
	rawRelationship, decodeErr := GuardianRelationshipRow(relationship)
	if decodeErr != nil {
		return 0, decodeErr
	}
	out0, out1 := r.rawStudentGuardianRepository.UpdateColumns(ctx, rawRelationship, columns...)
	if relationship != nil && rawRelationship != nil {
		*relationship = *NativeGuardianRelationshipValue(rawRelationship)
	}
	return out0, out1
}
