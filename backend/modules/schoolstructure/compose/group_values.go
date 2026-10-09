package compose

import (
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/application/groups"
)

func publicGroup(row *educationModels.Group) *schoolstructure.Group {
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

func legacyGroup(value *schoolstructure.Group) *educationModels.Group {
	if value == nil {
		return nil
	}
	row := &educationModels.Group{Model: educationModels.Model{ID: value.ID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}, TenantModel: educationModels.TenantModel{TenantID: value.TenantID}, Name: value.Name, RoomID: value.RoomID}
	if value.Room != nil {
		room := educationModels.GroupRoom(*value.Room)
		row.Room = &room
	}
	return row
}

func publicGroupQuery(query *schoolstructure.GroupListQuery) *educationModels.GroupListQuery {
	if query == nil {
		return nil
	}
	result := educationModels.GroupListQuery(*query)
	return &result
}

func publicTeacher(row *groups.Teacher) *schoolstructure.Teacher {
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

func publicGroupError(err error) error {
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
	case *groups.EducationError:
		return &schoolstructure.EducationError{Op: value.Op, Err: publicGroupError(value.Err)}, true
	case *educationModels.DatabaseError:
		return &groupStoreError{op: value.Op, err: publicGroupError(value.Err)}, true
	}
	switch err {
	case educationModels.ErrNotFound:
		return groupRecordNotFound, true
	case groups.ErrGroupNotFound:
		return schoolstructure.ErrEducationGroupNotFound, true
	case groups.ErrTeacherNotFound:
		return schoolstructure.ErrTeacherNotFound, true
	case groups.ErrStaffNotFound:
		return schoolstructure.ErrStaffNotFound, true
	case groups.ErrEmptySchoolClass:
		return schoolstructure.ErrEmptySchoolClass, true
	case groups.ErrGroupTeacherNotFound:
		return schoolstructure.ErrGroupTeacherNotFound, true
	case groups.ErrSubstitutionNotFound:
		return schoolstructure.ErrSubstitutionNotFound, true
	case groups.ErrRoomNotFound:
		return schoolstructure.ErrRoomNotFound, true
	case groups.ErrDuplicateGroup:
		return schoolstructure.ErrDuplicateGroup, true
	case groups.ErrDuplicateTeacherInGroup:
		return schoolstructure.ErrDuplicateTeacherInGroup, true
	case groups.ErrSubstitutionConflict:
		return schoolstructure.ErrSubstitutionConflict, true
	case groups.ErrInvalidDateRange:
		return schoolstructure.ErrInvalidDateRange, true
	case groups.ErrSubstitutionBackdated:
		return schoolstructure.ErrSubstitutionBackdated, true
	case groups.ErrGroupHasStudents:
		return schoolstructure.ErrGroupHasStudents, true
	case groups.ErrGroupHasHandover:
		return schoolstructure.ErrGroupHasHandover, true
	}
	return mapGroupErrorCauses(err, neutralGroupError)
}

// Map only changed branches. Preserve wrapper text and every joined cause,
// including sql.ErrNoRows, while retaining unrelated error identities.
func mapGroupErrorCauses(err error, convert func(error) (error, bool)) (error, bool) {
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

func (e *groupJoinedError) Error() string   { return e.message }
func (e *groupJoinedError) Unwrap() []error { return e.causes }

// Storage failures preserve their wire text, driver cause, and classification
// without carrying a legacy model value across the native contract.
type groupStoreError struct {
	op  string
	err error
}

func (e *groupStoreError) Error() string {
	if e.err == nil {
		return "database error during " + e.op
	}
	return "database error during " + e.op + ": " + e.err.Error()
}
func (e *groupStoreError) Unwrap() error      { return e.err }
func (e *groupStoreError) StoreFailure() bool { return true }

var groupRecordNotFound error = groupNotFoundError{}

type groupNotFoundError struct{}

func (groupNotFoundError) Error() string       { return "repository: not found" }
func (groupNotFoundError) RepositoryNotFound() {}
