package users

import (
	context "context"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	models "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (r nativeStudentRepository) UpdateStatus(ctx context.Context, studentID int64, newStatus string) error {
	rawNewStatus := models.StudentStatus(newStatus)
	out0 := r.rawStudentRepository.UpdateStatus(ctx, studentID, rawNewStatus)

	return out0
}

func (r nativeStudentRepository) TransitionStatus(ctx context.Context, studentID int64, expected string, next string) (bool, error) {
	rawExpected := models.StudentStatus(expected)
	rawNext := models.StudentStatus(next)
	out0, out1 := r.rawStudentRepository.TransitionStatus(ctx, studentID, rawExpected, rawNext)

	return out0, out1
}

func (r nativeStudentRepository) FindPendingDueForActivation(ctx context.Context, asOf calendar.Date) ([]*directory.StudentState, error) {

	out0, out1 := r.rawStudentRepository.FindPendingDueForActivation(ctx, asOf)

	return mapRecordSlice(out0, func(value *models.Student) *directory.StudentState { return NativeStudentState(value) }), out1
}

func (r nativeStudentRepository) FindActiveDueForDeactivation(ctx context.Context, asOf calendar.Date) ([]*directory.StudentState, error) {

	out0, out1 := r.rawStudentRepository.FindActiveDueForDeactivation(ctx, asOf)

	return mapRecordSlice(out0, func(value *models.Student) *directory.StudentState { return NativeStudentState(value) }), out1
}

func (r nativeStudentRepository) FindByIDForUpdate(ctx context.Context, id int64) (*directory.StudentState, error) {

	out0, out1 := r.rawStudentRepository.FindByIDForUpdate(ctx, id)

	return NativeStudentState(out0), out1
}

func (r nativeStudentRepository) FindByIDsForUpdate(ctx context.Context, ids []int64) (map[int64]*directory.StudentState, error) {

	out0, out1 := r.rawStudentRepository.FindByIDsForUpdate(ctx, ids)

	return mapRecordValues(out0, func(value *models.Student) *directory.StudentState { return NativeStudentState(value) }), out1
}
