package groups

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
)

func PublicGroup(row *domain.Group) *schoolstructure.Group {
	if row == nil {
		return nil
	}
	result := &schoolstructure.Group{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Name: row.Name, RoomID: row.RoomID}
	if row.Room != nil {
		value := schoolstructure.GroupRoom(*row.Room)
		result.Room = &value
	}
	return result
}

func LegacyGroup(value *schoolstructure.Group) *domain.Group {
	if value == nil {
		return nil
	}
	row := &domain.Group{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, Name: value.Name, RoomID: value.RoomID}
	if value.Room != nil {
		room := domain.GroupRoom(*value.Room)
		row.Room = &room
	}
	return row
}

func publicGroupQuery(query *schoolstructure.GroupListQuery) *domain.GroupListQuery {
	if query == nil {
		return nil
	}
	result := domain.GroupListQuery(*query)
	return &result
}

func publicTeacher(row *Teacher) *schoolstructure.Teacher {
	if row == nil {
		return nil
	}
	result := &schoolstructure.Teacher{ID: row.ID, StaffID: row.StaffID, Specialization: row.Specialization, Role: row.Role, Qualifications: row.Qualifications, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	if row.Person != nil {
		person := schoolstructure.TeacherPerson(*row.Person)
		result.Person = &person
	}
	return result
}

func PublicGroupError(err error) error {
	value, _ := neutralGroupError(err)
	return value
}

func neutralGroupError(err error) (error, bool) {
	if err == nil {
		return nil, false
	}
	switch value := err.(type) {
	case *OperationError:
		cause, changed := neutralGroupError(value.Cause)
		if !changed {
			return err, false
		}
		result := *value
		result.Cause = cause
		return &result, true
	case *EducationError:
		return &schoolstructure.EducationError{Op: value.Op, Err: PublicGroupError(value.Err)}, true
	case *domain.StoreError:
		return &GroupStoreError{Op: value.Op, Err: PublicGroupError(value.Err)}, true
	}
	switch err {
	case domain.RecordNotFound:
		return GroupRecordNotFound, true
	case ErrGroupNotFound:
		return schoolstructure.ErrEducationGroupNotFound, true
	case ErrTeacherNotFound:
		return schoolstructure.ErrTeacherNotFound, true
	case ErrStaffNotFound:
		return schoolstructure.ErrStaffNotFound, true
	case ErrEmptySchoolClass:
		return schoolstructure.ErrEmptySchoolClass, true
	case ErrGroupTeacherNotFound:
		return schoolstructure.ErrGroupTeacherNotFound, true
	case ErrSubstitutionNotFound:
		return schoolstructure.ErrSubstitutionNotFound, true
	case ErrRoomNotFound:
		return schoolstructure.ErrRoomNotFound, true
	case ErrDuplicateGroup:
		return schoolstructure.ErrDuplicateGroup, true
	case ErrDuplicateTeacherInGroup:
		return schoolstructure.ErrDuplicateTeacherInGroup, true
	case ErrSubstitutionConflict:
		return schoolstructure.ErrSubstitutionConflict, true
	case ErrInvalidDateRange:
		return schoolstructure.ErrInvalidDateRange, true
	case ErrSubstitutionBackdated:
		return schoolstructure.ErrSubstitutionBackdated, true
	case ErrGroupHasStudents:
		return schoolstructure.ErrGroupHasStudents, true
	case ErrGroupHasHandover:
		return schoolstructure.ErrGroupHasHandover, true
	}
	return MapGroupErrorCauses(err, neutralGroupError)
}

// Map only changed branches. Preserve wrapper text and every joined cause,
// including sql.ErrNoRows, while retaining unrelated error identities.
func MapGroupErrorCauses(err error, convert func(error) (error, bool)) (error, bool) {
	switch value := err.(type) {
	case interface{ Unwrap() []error }:
		causes := value.Unwrap()
		result := make([]error, len(causes))
		changed := false
		for i, cause := range causes {
			mapped, replaced := convert(cause)
			result[i] = mapped
			changed = changed || replaced
		}
		if changed {
			return &groupJoinedError{message: err.Error(), causes: result}, true
		}
	case interface{ Unwrap() error }:
		cause, changed := convert(value.Unwrap())
		if changed {
			return &groupWrappedError{message: err.Error(), cause: cause}, true
		}
	}
	return err, false
}

type groupWrappedError struct {
	message string
	cause   error
}

func (e *groupWrappedError) Error() string { return e.message }

func (e *groupWrappedError) Unwrap() error { return e.cause }

type groupJoinedError struct {
	message string
	causes  []error
}

func (e *groupJoinedError) Error() string { return e.message }

func (e *groupJoinedError) Unwrap() []error { return e.causes }

// Storage failures preserve their wire text, driver cause, and classification
// without carrying a legacy model value across the native contract.
type GroupStoreError struct {
	Op  string
	Err error
}

func (e *GroupStoreError) Error() string {
	if e.Err == nil {
		return "database error during " + e.Op
	}
	return "database error during " + e.Op + ": " + e.Err.Error()
}

func (e *GroupStoreError) Unwrap() error { return e.Err }

func (e *GroupStoreError) StoreFailure() bool { return true }

var GroupRecordNotFound error = groupNotFoundError{}

type groupNotFoundError struct{}

func (groupNotFoundError) Error() string { return "repository: not found" }

func (groupNotFoundError) RepositoryNotFound() {}
