package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/application/groups"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
)

func legacyTeacher(value *schoolstructure.Teacher) *groups.Teacher {
	if value == nil {
		return nil
	}
	result := &groups.Teacher{ID: value.ID, StaffID: value.StaffID, Specialization: value.Specialization, Role: value.Role, Qualifications: value.Qualifications, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
	if value.Person != nil {
		person := groups.TeacherPerson(*value.Person)
		result.Person = &person
	}
	return result
}

func legacyGroupError(err error) error {
	value, _ := retainedGroupError(err)
	return value
}
func retainedGroupError(err error) (error, bool) {
	if err == nil {
		return nil, false
	}
	switch value := err.(type) {
	case *schoolstructure.EducationError:
		return &groups.EducationError{Op: value.Op, Err: legacyGroupError(value.Err)}, true
	case *groupStoreError:
		return &domain.StoreError{Op: value.op, Err: legacyGroupError(value.err)}, true
	}
	switch err {
	case groupRecordNotFound:
		return domain.RecordNotFound, true
	case schoolstructure.ErrEducationGroupNotFound:
		return groups.ErrGroupNotFound, true
	case schoolstructure.ErrTeacherNotFound:
		return groups.ErrTeacherNotFound, true
	case schoolstructure.ErrStaffNotFound:
		return groups.ErrStaffNotFound, true
	case schoolstructure.ErrEmptySchoolClass:
		return groups.ErrEmptySchoolClass, true
	case schoolstructure.ErrGroupTeacherNotFound:
		return groups.ErrGroupTeacherNotFound, true
	case schoolstructure.ErrSubstitutionNotFound:
		return groups.ErrSubstitutionNotFound, true
	case schoolstructure.ErrRoomNotFound:
		return groups.ErrRoomNotFound, true
	case schoolstructure.ErrDuplicateGroup:
		return groups.ErrDuplicateGroup, true
	case schoolstructure.ErrDuplicateTeacherInGroup:
		return groups.ErrDuplicateTeacherInGroup, true
	case schoolstructure.ErrSubstitutionConflict:
		return groups.ErrSubstitutionConflict, true
	case schoolstructure.ErrInvalidDateRange:
		return groups.ErrInvalidDateRange, true
	case schoolstructure.ErrSubstitutionBackdated:
		return groups.ErrSubstitutionBackdated, true
	case schoolstructure.ErrGroupHasStudents:
		return groups.ErrGroupHasStudents, true
	case schoolstructure.ErrGroupHasHandover:
		return groups.ErrGroupHasHandover, true
	}
	return mapGroupErrorCauses(err, retainedGroupError)
}

func legacyGroupQuery(query *domain.GroupListQuery) *schoolstructure.GroupListQuery {
	if query == nil {
		return nil
	}
	result := schoolstructure.GroupListQuery(*query)
	return &result
}

// Legacy storage vocabulary belongs only to the retained suite seam.
type legacyGroupStorageTestError = domain.StoreError

var legacyGroupRecordTestNotFound = domain.RecordNotFound
