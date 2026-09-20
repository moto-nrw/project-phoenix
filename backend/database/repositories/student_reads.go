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
	ListStudentRoster(context.Context, string) ([]peopleModule.StudentRosterEntry, error)
	ListStudentRosterByGroup(context.Context, []int64, string) ([]peopleModule.StudentRosterEntry, error)
	ListStudentRosterOverlapping(context.Context, string, string, string) ([]peopleModule.StudentRosterEntry, error)
	FindStudentRecordForMutation(context.Context, int64) (peopleModule.StudentRecord, error)
	FindStudentRecordForMutationNoWait(context.Context, int64) (peopleModule.StudentRecord, error)
	LockEnrollmentClassWrites(context.Context) error
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
		return r.directory.ListStudentRecordsByGroup(ctx, []int64{groupID}, peopleModule.StudentScopeEnrolled)
	})
}

// ListByGroupIDsIncludingAlumni backs the care-participation candidate set,
// which decides per child whether a graduate still counts and therefore must
// see them.
func (r *StudentReads) ListByGroupIDsIncludingAlumni(ctx context.Context, groupIDs []int64) ([]*userModels.Student, error) {
	return r.listRecords(ctx, "list by group ids including alumni", func() ([]peopleModule.StudentRecord, error) {
		return r.directory.ListStudentRecordsByGroup(ctx, groupIDs, peopleModule.StudentScopeAll)
	})
}

// ListClassRoster is the class-roster report's candidate set: one class, or
// every child when the report spans all of them. Graduates are included
// because the shared participation rule filters the set afterwards.
func (r *StudentReads) ListClassRoster(ctx context.Context, schoolClass string) ([]*userModels.Student, error) {
	return r.listRecords(ctx, "list class roster", func() ([]peopleModule.StudentRecord, error) {
		if schoolClass == "" {
			return r.directory.ListStudentRecords(ctx, peopleModule.StudentScopeAll)
		}
		return r.directory.ListStudentRecordsByClass(ctx, []string{schoolClass}, peopleModule.StudentScopeAll)
	})
}

// CountEnrolled is the platform statistic: how many children a school has,
// graduates excluded.
func (r *StudentReads) CountEnrolled(ctx context.Context) (int, error) {
	ids, err := r.directory.ListStudentDirectoryIDs(ctx)
	if err != nil {
		return 0, translateStudentReadError("count enrolled students", err)
	}
	return len(ids), nil
}

func (r *StudentReads) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*userModels.Student, error) {
	return r.listRecords(ctx, "find by group ids", func() ([]peopleModule.StudentRecord, error) {
		return r.directory.ListStudentRecordsByGroup(ctx, groupIDs, peopleModule.StudentScopeEnrolled)
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

// FindByIDForUpdate takes the row lock the caller will write under. The owner
// takes the shared class-writes gate first, which is what keeps the
// acquisition order gate-then-rows for every writer.
func (r *StudentReads) FindByIDForUpdate(ctx context.Context, id int64) (*userModels.Student, error) {
	record, err := r.directory.FindStudentRecordForMutation(ctx, id)
	if err != nil {
		return nil, translateStudentReadError("find by id for update", err)
	}
	return studentRecordToModel(record), nil
}

// FindByIDForUpdateNoWait refuses rather than waits, which is what makes a
// downward acquisition in the companion graph safe.
func (r *StudentReads) FindByIDForUpdateNoWait(ctx context.Context, id int64) (*userModels.Student, error) {
	record, err := r.directory.FindStudentRecordForMutationNoWait(ctx, id)
	if err != nil {
		if errors.Is(err, peopleModule.ErrStudentLockBusy) {
			return nil, userModels.ErrCompanionLockBusy
		}
		return nil, translateStudentReadError("find by id for update nowait", err)
	}
	return studentRecordToModel(record), nil
}

// LockStudentClassWritesShared exposes the owner's shared class-writes gate for
// the caller that has to take it before another tenant-wide gate.
func (r *StudentReads) LockStudentClassWritesShared(ctx context.Context) error {
	return translateStudentReadError("lock student class writes", r.directory.LockEnrollmentClassWrites(ctx))
}

// The roster reads. The teacher-scoped ones resolve their groups through
// School Membership first — the assignment is that owner's — and then ask this
// owner for the children of those groups.
func (r *StudentReads) FindByTeacherIDWithGroups(
	ctx context.Context,
	groupIDs []int64,
	today userModels.CalendarDate,
) ([]*userModels.StudentWithGroupInfo, error) {
	return r.roster(ctx, "find by teacher id with groups", func() ([]peopleModule.StudentRosterEntry, error) {
		return r.directory.ListStudentRosterByGroup(ctx, groupIDs, today.String())
	})
}

func (r *StudentReads) FindAllWithGroups(
	ctx context.Context,
	today userModels.CalendarDate,
) ([]*userModels.StudentWithGroupInfo, error) {
	return r.roster(ctx, "find all with groups", func() ([]peopleModule.StudentRosterEntry, error) {
		return r.directory.ListStudentRoster(ctx, today.String())
	})
}

func (r *StudentReads) FindOverlappingWithGroups(
	ctx context.Context,
	from, to, today userModels.CalendarDate,
) ([]*userModels.StudentWithGroupInfo, error) {
	return r.roster(ctx, "find overlapping with groups", func() ([]peopleModule.StudentRosterEntry, error) {
		return r.directory.ListStudentRosterOverlapping(ctx, from.String(), to.String(), today.String())
	})
}

func (r *StudentReads) roster(
	ctx context.Context,
	operation string,
	read func() ([]peopleModule.StudentRosterEntry, error),
) ([]*userModels.StudentWithGroupInfo, error) {
	entries, err := read()
	if err != nil {
		return nil, translateStudentReadError(operation, err)
	}
	result := make([]*userModels.StudentWithGroupInfo, 0, len(entries))
	for _, entry := range entries {
		student := studentRecordToModel(entry.Record)
		person := &userModels.Person{
			FirstName: entry.FirstName, LastName: entry.LastName,
			TagID: entry.TagID, AccountID: entry.AccountID,
		}
		person.ID = entry.Record.PersonID
		person.SetTenantID(entry.Record.TenantID)
		student.Person = person
		result = append(result, &userModels.StudentWithGroupInfo{Student: student})
	}
	return result, nil
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
	// nil, not an empty slice: the retained contract distinguishes them, and a
	// caller that ranges over the result sees no difference either way.
	if len(records) == 0 {
		return nil, nil
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
