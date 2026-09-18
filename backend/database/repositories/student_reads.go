package repositories

import (
	"context"
	"errors"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// This file is the read half of the student translation seam (#3349): People
// Directory owns users.students, and the retained callers still carry
// users.Student rows. Every method here is a value translation; which rows a
// lookup returns — whether alumni are in it, how a name is matched — is the
// owner's decision.

// StudentReadCapability is the owner surface this seam translates to.
type StudentReadCapability interface {
	peopleModule.StudentDirectoryQuery
	// ListSchoolClasses is on the owner's narrow student query; the retained
	// repository serves it from the same rows.
	ListSchoolClasses(context.Context) ([]string, error)
	LockStudentRecordsByID(context.Context, []int64) ([]peopleModule.StudentRecord, error)
}

// StudentReads adapts the owner's child reads to the retained model-typed
// contract, satisfied structurally so the seam does not depend on it.
type StudentReads struct{ directory StudentReadCapability }

func NewStudentReads(directory StudentReadCapability) *StudentReads {
	return &StudentReads{directory: directory}
}

func (r *StudentReads) FindByID(ctx context.Context, id any) (*userModels.Student, error) {
	studentID, ok := studentIDOf(id)
	if !ok {
		return nil, missingStudent("find by id")
	}
	record, err := r.directory.FindStudentRecord(ctx, studentID)
	if err != nil {
		return nil, translateStudentReadError("find by id", err)
	}
	return studentRecordToModel(record), nil
}

func (r *StudentReads) FindByPersonID(ctx context.Context, personID int64) (*userModels.Student, error) {
	records, err := r.directory.ListStudentRecordsByPerson(ctx, []int64{personID})
	if err != nil {
		return nil, translateStudentReadError("find by person id", err)
	}
	if len(records) == 0 {
		return nil, missingStudent("find by person id")
	}
	return studentRecordToModel(records[0]), nil
}

func (r *StudentReads) FindByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Student, error) {
	records, err := r.directory.ListStudentRecordsByID(ctx, ids)
	if err != nil {
		return nil, translateStudentReadError("find by ids", err)
	}
	return studentRecordsByID(records), nil
}

// FindReadScopeByIDs answers only what an access decision needs: the child's
// group, identity and class. It reads the same rows as FindByIDs; the narrower
// name is the contract with its callers, not a different query.
func (r *StudentReads) FindReadScopeByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Student, error) {
	return r.FindByIDs(ctx, ids)
}

func (r *StudentReads) FindByGroupID(ctx context.Context, groupID int64) ([]*userModels.Student, error) {
	return r.listRecords(ctx, "find by group id", func() ([]peopleModule.StudentRecord, error) {
		return r.directory.ListStudentRecordsByGroup(ctx, []int64{groupID})
	})
}

func (r *StudentReads) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*userModels.Student, error) {
	return r.listRecords(ctx, "find by group ids", func() ([]peopleModule.StudentRecord, error) {
		return r.directory.ListStudentRecordsByGroup(ctx, groupIDs)
	})
}

func (r *StudentReads) FindBySchoolClass(ctx context.Context, schoolClass string) ([]*userModels.Student, error) {
	return r.listRecords(ctx, "find by school class", func() ([]peopleModule.StudentRecord, error) {
		return r.directory.ListStudentRecordsByClass(ctx, []string{schoolClass})
	})
}

func (r *StudentReads) FindByGuardianEmail(ctx context.Context, email string) ([]*userModels.Student, error) {
	return r.listRecords(ctx, "find by guardian email", func() ([]peopleModule.StudentRecord, error) {
		return r.directory.ListStudentRecordsByGuardianContact(ctx, email, "")
	})
}

func (r *StudentReads) FindByGuardianPhone(ctx context.Context, phone string) ([]*userModels.Student, error) {
	return r.listRecords(ctx, "find by guardian phone", func() ([]peopleModule.StudentRecord, error) {
		return r.directory.ListStudentRecordsByGuardianContact(ctx, "", phone)
	})
}

func (r *StudentReads) FindPendingDueForActivation(
	ctx context.Context,
	asOf userModels.CalendarDate,
) ([]*userModels.Student, error) {
	return r.listRecords(ctx, "find pending due for activation", func() ([]peopleModule.StudentRecord, error) {
		return r.directory.ListStudentRecordsDueForStatus(
			ctx, string(userModels.StudentStatusPending), peopleModule.StudentBoundCareStart, asOf.String())
	})
}

func (r *StudentReads) FindActiveDueForDeactivation(
	ctx context.Context,
	asOf userModels.CalendarDate,
) ([]*userModels.Student, error) {
	return r.listRecords(ctx, "find active due for deactivation", func() ([]peopleModule.StudentRecord, error) {
		return r.directory.ListStudentRecordsDueForStatus(
			ctx, string(userModels.StudentStatusActive), peopleModule.StudentBoundCareEnd, asOf.String())
	})
}

func (r *StudentReads) FindByIDsForUpdate(ctx context.Context, ids []int64) (map[int64]*userModels.Student, error) {
	records, err := r.directory.LockStudentRecordsByID(ctx, ids)
	if err != nil {
		return nil, translateStudentReadError("find by ids for update", err)
	}
	return studentRecordsByID(records), nil
}

func (r *StudentReads) CountByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64]int, error) {
	counts, err := r.directory.CountStudentsByGroup(ctx, groupIDs)
	if err != nil {
		return nil, translateStudentReadError("count by group ids", err)
	}
	return counts, nil
}

func (r *StudentReads) ExistsEnrolledByNameAndBirthday(
	ctx context.Context,
	tenantID int64,
	firstName, lastName string,
	birthday userModels.CalendarDate,
) (bool, error) {
	ids, err := r.directory.ListEnrolledStudentIDsByNameAndBirthday(
		ctx, tenantID, firstName, lastName, birthday.String())
	if err != nil {
		return false, translateStudentReadError("exists enrolled by name and birthday", err)
	}
	return len(ids) > 0, nil
}

// FindEnrolledStudentIDByNameAndBirthday resolves the re-enrollment reference
// only when exactly one child matches. An ambiguous match stores no reference,
// so approval falls back to creating a fresh child rather than renewing an
// arbitrary one.
func (r *StudentReads) FindEnrolledStudentIDByNameAndBirthday(
	ctx context.Context,
	tenantID int64,
	firstName, lastName string,
	birthday userModels.CalendarDate,
) (*int64, error) {
	ids, err := r.directory.ListEnrolledStudentIDsByNameAndBirthday(
		ctx, tenantID, firstName, lastName, birthday.String())
	if err != nil {
		return nil, translateStudentReadError("find enrolled student id by name and birthday", err)
	}
	if len(ids) != 1 {
		return nil, nil
	}
	return &ids[0], nil
}

func (r *StudentReads) ListSchoolClasses(ctx context.Context) ([]string, error) {
	classes, err := r.directory.ListSchoolClasses(ctx)
	if err != nil {
		return nil, translateStudentReadError("list school classes", err)
	}
	return classes, nil
}

func (r *StudentReads) ListIDs(ctx context.Context) ([]int64, error) {
	// Alumni included: this is the candidate set a presence lookup narrows,
	// and a graduate who is actually present must survive it.
	ids, err := r.directory.ListAllStudentIDs(ctx)
	if err != nil {
		return nil, translateStudentReadError("list student ids", err)
	}
	return ids, nil
}

func (r *StudentReads) listRecords(
	ctx context.Context,
	operation string,
	read func() ([]peopleModule.StudentRecord, error),
) ([]*userModels.Student, error) {
	records, err := read()
	if err != nil {
		return nil, translateStudentReadError(operation, err)
	}
	result := make([]*userModels.Student, 0, len(records))
	for _, record := range records {
		result = append(result, studentRecordToModel(record))
	}
	return result, nil
}

func studentRecordsByID(records []peopleModule.StudentRecord) map[int64]*userModels.Student {
	result := make(map[int64]*userModels.Student, len(records))
	for _, record := range records {
		result[record.ID] = studentRecordToModel(record)
	}
	return result
}

// missingStudent is the shape the retained callers branch on for a child that
// is not there; the model owns it, because the seam may not name the error
// package itself.
func missingStudent(op string) error {
	return userModels.MissingStudentError(op)
}

func translateStudentReadError(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, peopleModule.ErrStudentNotFound) || errors.Is(err, peopleModule.ErrInvalidStudent) {
		return missingStudent(op)
	}
	return err
}
