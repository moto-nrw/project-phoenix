package users

import (
	context "context"
	time "time"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	models "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
	ports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type nativeStudentRepository struct{ rawStudentRepository }

func NewStudentRepository(runtime Runtime, students StudentDirectoryRows) ports.StudentRepository {
	return nativeStudentRepository{rawStudentRepository: newRawStudentRepository(runtime, students)}
}

func (r nativeStudentRepository) Create(ctx context.Context, entity *directory.StudentState) error {
	rawEntity := StudentStateRow(entity)
	out0 := r.rawStudentRepository.Create(ctx, rawEntity)
	if entity != nil && rawEntity != nil {
		*entity = *NativeStudentState(rawEntity)
	}
	return out0
}

func (r nativeStudentRepository) FindByID(ctx context.Context, id any) (*directory.StudentState, error) {

	out0, out1 := r.rawStudentRepository.FindByID(ctx, id)

	return NativeStudentState(out0), out1
}

func (r nativeStudentRepository) Update(ctx context.Context, entity *directory.StudentState) error {
	rawEntity := StudentStateRow(entity)
	out0 := r.rawStudentRepository.Update(ctx, rawEntity)
	if entity != nil && rawEntity != nil {
		*entity = *NativeStudentState(rawEntity)
	}
	return out0
}

func (r nativeStudentRepository) List(ctx context.Context, filters map[string]any) ([]*directory.StudentState, error) {

	out0, out1 := r.rawStudentRepository.List(ctx, filters)

	return mapRecordSlice(out0, func(value *models.Student) *directory.StudentState { return NativeStudentState(value) }), out1
}

func (r nativeStudentRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*directory.StudentState, error) {

	out0, out1 := r.rawStudentRepository.FindByIDs(ctx, ids)

	return mapRecordValues(out0, func(value *models.Student) *directory.StudentState { return NativeStudentState(value) }), out1
}

func (r nativeStudentRepository) FindReadScopeByIDs(ctx context.Context, ids []int64) (map[int64]*directory.StudentState, error) {

	out0, out1 := r.rawStudentRepository.FindReadScopeByIDs(ctx, ids)

	return mapRecordValues(out0, func(value *models.Student) *directory.StudentState { return NativeStudentState(value) }), out1
}

func (r nativeStudentRepository) FindByPersonID(ctx context.Context, personID int64) (*directory.StudentState, error) {

	out0, out1 := r.rawStudentRepository.FindByPersonID(ctx, personID)

	return NativeStudentState(out0), out1
}

func (r nativeStudentRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*directory.StudentState, error) {

	out0, out1 := r.rawStudentRepository.FindByGroupID(ctx, groupID)

	return mapRecordSlice(out0, func(value *models.Student) *directory.StudentState { return NativeStudentState(value) }), out1
}

func (r nativeStudentRepository) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*directory.StudentState, error) {

	out0, out1 := r.rawStudentRepository.FindByGroupIDs(ctx, groupIDs)

	return mapRecordSlice(out0, func(value *models.Student) *directory.StudentState { return NativeStudentState(value) }), out1
}

func (r nativeStudentRepository) ListByGroupIDsIncludingAlumni(ctx context.Context, groupIDs []int64) ([]*directory.StudentState, error) {

	out0, out1 := r.rawStudentRepository.ListByGroupIDsIncludingAlumni(ctx, groupIDs)

	return mapRecordSlice(out0, func(value *models.Student) *directory.StudentState { return NativeStudentState(value) }), out1
}

func (r nativeStudentRepository) ListClassRoster(ctx context.Context, schoolClass string) ([]*directory.StudentState, error) {

	out0, out1 := r.rawStudentRepository.ListClassRoster(ctx, schoolClass)

	return mapRecordSlice(out0, func(value *models.Student) *directory.StudentState { return NativeStudentState(value) }), out1
}

func (r nativeStudentRepository) FindByTeacherIDWithGroups(ctx context.Context, teacherID int64) ([]*directory.StudentRosterEntry, error) {

	out0, out1 := r.rawStudentRepository.FindByTeacherIDWithGroups(ctx, teacherID)

	return mapRecordSlice(out0, func(value *models.StudentWithGroupInfo) *directory.StudentRosterEntry {
		return NativeStudentRosterEntry(value)
	}), out1
}

func (r nativeStudentRepository) FindByTeacherStaffIDsWithGroups(ctx context.Context, staffIDs []int64) ([]*directory.StudentRosterEntry, error) {

	out0, out1 := r.rawStudentRepository.FindByTeacherStaffIDsWithGroups(ctx, staffIDs)

	return mapRecordSlice(out0, func(value *models.StudentWithGroupInfo) *directory.StudentRosterEntry {
		return NativeStudentRosterEntry(value)
	}), out1
}

func (r nativeStudentRepository) FindAllWithGroups(ctx context.Context) ([]*directory.StudentRosterEntry, error) {

	out0, out1 := r.rawStudentRepository.FindAllWithGroups(ctx)

	return mapRecordSlice(out0, func(value *models.StudentWithGroupInfo) *directory.StudentRosterEntry {
		return NativeStudentRosterEntry(value)
	}), out1
}

func (r nativeStudentRepository) FindOverlappingWithGroups(ctx context.Context, from calendar.Date, to calendar.Date, today calendar.Date) ([]*directory.StudentRosterEntry, error) {

	out0, out1 := r.rawStudentRepository.FindOverlappingWithGroups(ctx, from, to, today)

	return mapRecordSlice(out0, func(value *models.StudentWithGroupInfo) *directory.StudentRosterEntry {
		return NativeStudentRosterEntry(value)
	}), out1
}

func (r nativeStudentRepository) FindOverlappingWithGroupsOnDate(ctx context.Context, date string, now time.Time) ([]*directory.StudentRosterEntry, error) {

	out0, out1 := r.rawStudentRepository.FindOverlappingWithGroupsOnDate(ctx, date, now)

	return mapRecordSlice(out0, func(value *models.StudentWithGroupInfo) *directory.StudentRosterEntry {
		return NativeStudentRosterEntry(value)
	}), out1
}

// Locked student reads belong to the native StudentRecords boundary.
func (r nativeStudentRepository) FindByPersonIDForUpdate(context.Context, int64) (*directory.StudentState, error) {
	return nil, errStudentWritesMoved
}
