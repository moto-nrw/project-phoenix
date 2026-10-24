package application

import (
	"context"

	owner0 "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// StudentRecords binds the retained boundary vocabulary directly to the native
// student service. Same-owner callers do not re-enter the public capability.
type StudentRecords struct {
	peoplePorts.StudentRepository
	directory            *StudentService
	membership           StudentEnrollmentStatus
	teacherGroupIDs      func(context.Context, int64) ([]int64, error)
	teacherStaffGroupIDs func(context.Context, []int64) ([]int64, error)
}

func (r *StudentRecords) FindByID(ctx context.Context, id any) (*peopleModule.StudentState, error) {
	studentID, ok := studentIDOf(id)
	if !ok || studentID <= 0 {
		return nil, peoplePorts.MissingStudentError("find by id")
	}
	record, err := r.directory.FindRecord(ctx, studentID, "")
	if err != nil {
		return nil, RetainedStudentReadError("find by id", MapError(err))
	}
	return peopleModule.StudentStateFromRecord(peopleModule.StudentRecord(record)), nil
}

func (r *StudentRecords) FindByPersonID(ctx context.Context, personID int64) (*peopleModule.StudentState, error) {
	if personID <= 0 {
		return nil, peoplePorts.MissingStudentError("find by person id")
	}
	records, err := r.directory.ListRecordsByPersonIDs(ctx, []int64{personID})
	if err != nil {
		return nil, RetainedStudentReadError("find by person id", MapError(err))
	}
	if len(records) == 0 {
		return nil, peoplePorts.MissingStudentError("find by person id")
	}
	return peopleModule.StudentStateFromRecord(peopleModule.StudentRecord(records[0])), nil
}

func (r *StudentRecords) FindByIDs(ctx context.Context, ids []int64) (map[int64]*peopleModule.StudentState, error) {
	ids = peopleModule.PositiveIDs(ids)
	if len(ids) == 0 {
		return map[int64]*peopleModule.StudentState{}, nil
	}
	records, err := r.directory.ListRecordsByIDs(ctx, ids)
	if err != nil {
		return nil, RetainedStudentReadError("find by ids", MapError(err))
	}
	return studentRecordsByID(records), nil
}

// FindReadScopeByIDs answers only what an access decision needs: the child's
// group, identity and class. It reads the same rows as FindByIDs; the narrower
// name is the contract with its callers, not a different query.
func (r *StudentRecords) FindReadScopeByIDs(ctx context.Context, ids []int64) (map[int64]*peopleModule.StudentState, error) {
	return r.FindByIDs(ctx, ids)
}

func (r *StudentRecords) FindByGroupID(ctx context.Context, groupID int64) ([]*peopleModule.StudentState, error) {
	if groupID <= 0 {
		return nil, nil
	}
	return r.listRecords(ctx, "find by group id", func() ([]domain.StudentRecord, error) {
		return r.directory.ListRecordsByGroups(ctx, []int64{groupID}, string(peopleModule.StudentScopeEnrolled))
	})
}

// ListByGroupIDsIncludingAlumni backs the care-participation candidate set,
// which decides per child whether a graduate still counts and therefore must
// see them.
func (r *StudentRecords) ListByGroupIDsIncludingAlumni(ctx context.Context, groupIDs []int64) ([]*peopleModule.StudentState, error) {
	groupIDs = peopleModule.PositiveIDs(groupIDs)
	if len(groupIDs) == 0 {
		return nil, nil
	}
	return r.listRecords(ctx, "list by group ids including alumni", func() ([]domain.StudentRecord, error) {
		return r.directory.ListRecordsByGroups(ctx, groupIDs, string(peopleModule.StudentScopeAll))
	})
}

// ListClassRoster is the class-roster report's candidate set: one class, or
// every child when the report spans all of them. Graduates are included
// because the shared participation rule filters the set afterwards.
func (r *StudentRecords) ListClassRoster(ctx context.Context, schoolClass string) ([]*peopleModule.StudentState, error) {
	return r.listRecords(ctx, "list class roster", func() ([]domain.StudentRecord, error) {
		if schoolClass == "" {
			return r.directory.ListRecords(ctx, string(peopleModule.StudentScopeAll))
		}
		classes := peopleModule.SchoolClasses([]string{schoolClass})
		if len(classes) == 0 {
			return []domain.StudentRecord{}, nil
		}
		return r.directory.ListRecordsByClasses(ctx, classes, string(peopleModule.StudentScopeAll))
	})
}

// CountEnrolled is the platform statistic: how many children a school has,
// graduates excluded.
func (r *StudentRecords) CountEnrolled(ctx context.Context) (int, error) {
	ids, err := r.directory.ListDirectoryIDs(ctx)
	if err != nil {
		return 0, RetainedStudentReadError("count enrolled students", MapError(err))
	}
	return len(ids), nil
}

func (r *StudentRecords) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*peopleModule.StudentState, error) {
	groupIDs = peopleModule.PositiveIDs(groupIDs)
	if len(groupIDs) == 0 {
		return nil, nil
	}
	return r.listRecords(ctx, "find by group ids", func() ([]domain.StudentRecord, error) {
		return r.directory.ListRecordsByGroups(ctx, groupIDs, string(peopleModule.StudentScopeEnrolled))
	})
}

func (r *StudentRecords) FindPendingDueForActivation(
	ctx context.Context,
	asOf owner0.Date,
) ([]*peopleModule.StudentState, error) {
	if _, err := owner0.ParseDate(asOf.String()); err != nil {
		return nil, RetainedStudentReadError("find pending due for activation", &peopleModule.InvalidStudentError{Reason: "the day must be a calendar date in YYYY-MM-DD format"})
	}
	return r.listRecords(ctx, "find pending due for activation", func() ([]domain.StudentRecord, error) {
		return r.directory.ListRecordsDueForStatus(
			ctx, string(peopleModule.StudentStatusPending), peopleModule.StudentBoundCareStart, asOf.String())
	})
}

func (r *StudentRecords) FindActiveDueForDeactivation(
	ctx context.Context,
	asOf owner0.Date,
) ([]*peopleModule.StudentState, error) {
	if _, err := owner0.ParseDate(asOf.String()); err != nil {
		return nil, RetainedStudentReadError("find active due for deactivation", &peopleModule.InvalidStudentError{Reason: "the day must be a calendar date in YYYY-MM-DD format"})
	}
	return r.listRecords(ctx, "find active due for deactivation", func() ([]domain.StudentRecord, error) {
		return r.directory.ListRecordsDueForStatus(
			ctx, string(peopleModule.StudentStatusActive), peopleModule.StudentBoundCareEnd, asOf.String())
	})
}

func (r *StudentRecords) FindByIDsForUpdate(ctx context.Context, ids []int64) (map[int64]*peopleModule.StudentState, error) {
	ids = peopleModule.PositiveIDs(ids)
	if len(ids) == 0 {
		return map[int64]*peopleModule.StudentState{}, nil
	}
	records, err := r.directory.LockRecordsByIDs(ctx, ids)
	if err != nil {
		return nil, RetainedStudentReadError("find by ids for update", MapError(err))
	}
	return studentRecordsByID(records), nil
}

func (r *StudentRecords) CountByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64]int, error) {
	groupIDs = peopleModule.PositiveIDs(groupIDs)
	if len(groupIDs) == 0 {
		return map[int64]int{}, nil
	}
	counts, err := r.directory.CountByGroups(ctx, groupIDs)
	if err != nil {
		return nil, RetainedStudentReadError("count by group ids", MapError(err))
	}
	return counts, nil
}

func (r *StudentRecords) ExistsEnrolledByNameAndBirthday(
	ctx context.Context,
	tenantID int64,
	firstName, lastName string,
	birthday owner0.Date,
) (bool, error) {
	if err := peopleModule.ValidateStudentMatch(tenantID, firstName, lastName, birthday.String()); err != nil {
		return false, RetainedStudentReadError("exists enrolled by name and birthday", err)
	}
	ids, err := r.directory.ListEnrolledIDsByNameAndBirthday(
		ctx, tenantID, firstName, lastName, birthday.String())
	if err != nil {
		return false, RetainedStudentReadError("exists enrolled by name and birthday", MapError(err))
	}
	return len(ids) > 0, nil
}

// FindEnrolledStudentIDByNameAndBirthday resolves the re-enrollment reference
// only when exactly one child matches. An ambiguous match stores no reference,
// so approval falls back to creating a fresh child rather than renewing an
// arbitrary one.
func (r *StudentRecords) FindEnrolledStudentIDByNameAndBirthday(
	ctx context.Context,
	tenantID int64,
	firstName, lastName string,
	birthday owner0.Date,
) (*int64, error) {
	if err := peopleModule.ValidateStudentMatch(tenantID, firstName, lastName, birthday.String()); err != nil {
		return nil, RetainedStudentReadError("find enrolled student id by name and birthday", err)
	}
	ids, err := r.directory.ListEnrolledIDsByNameAndBirthday(
		ctx, tenantID, firstName, lastName, birthday.String())
	if err != nil {
		return nil, RetainedStudentReadError("find enrolled student id by name and birthday", MapError(err))
	}
	if len(ids) != 1 {
		return nil, nil
	}
	return &ids[0], nil
}

func (r *StudentRecords) ListSchoolClasses(ctx context.Context) ([]string, error) {
	classes, err := r.directory.ListClasses(ctx)
	if err != nil {
		return nil, RetainedStudentReadError("list school classes", MapError(err))
	}
	return classes, nil
}

// FindByIDForUpdate takes the row lock the caller will write under. The owner
// takes the shared class-writes gate first, which is what keeps the
// acquisition order gate-then-rows for every writer.
func (r *StudentRecords) FindByIDForUpdate(ctx context.Context, id int64) (*peopleModule.StudentState, error) {
	if id <= 0 {
		return nil, peoplePorts.MissingStudentError("find by id for update")
	}
	record, err := r.directory.FindRecord(ctx, id, "UPDATE")
	if err != nil {
		return nil, RetainedStudentReadError("find by id for update", MapError(err))
	}
	return peopleModule.StudentStateFromRecord(peopleModule.StudentRecord(record)), nil
}

// The roster reads. The teacher-scoped ones resolve their groups through
// School Membership first — the assignment is that owner's — and then ask this
// owner for the children of those groups.
func (r *StudentRecords) FindAllWithGroups(
	ctx context.Context,
) ([]*peopleModule.StudentRosterEntry, error) {
	return r.roster(ctx, "find all with groups", func() ([]domain.StudentRosterEntry, error) {
		return r.directory.ListRoster(ctx, owner0.TodayDate().String())
	})
}

func (r *StudentRecords) FindOverlappingWithGroups(
	ctx context.Context,
	from, to, today owner0.Date,
) ([]*peopleModule.StudentRosterEntry, error) {
	for _, day := range []owner0.Date{from, to, today} {
		if err := peopleModule.ValidateRosterDay(day.String()); err != nil {
			return nil, RetainedStudentReadError("find overlapping with groups", err)
		}
	}
	return r.roster(ctx, "find overlapping with groups", func() ([]domain.StudentRosterEntry, error) {
		return r.directory.ListRosterOverlapping(ctx, from.String(), to.String(), today.String())
	})
}

func (r *StudentRecords) roster(
	ctx context.Context,
	operation string,
	read func() ([]domain.StudentRosterEntry, error),
) ([]*peopleModule.StudentRosterEntry, error) {
	entries, err := read()
	if err != nil {
		return nil, RetainedStudentReadError(operation, MapError(err))
	}
	result := make([]*peopleModule.StudentRosterEntry, 0, len(entries))
	for _, entry := range entries {
		student := peopleModule.StudentStateFromRecord(peopleModule.StudentRecord(entry.Record))
		person := &peopleModule.DirectoryPersonSnapshot{
			FirstName: entry.FirstName, LastName: entry.LastName,
			TagID: entry.TagID, AccountID: entry.AccountID,
		}
		person.ID = entry.Record.PersonID
		person.SetTenantID(entry.Record.TenantID)
		student.Person = person
		result = append(result, &peopleModule.StudentRosterEntry{Record: student})
	}
	return result, nil
}

func (r *StudentRecords) listRecords(
	ctx context.Context,
	operation string,
	read func() ([]domain.StudentRecord, error),
) ([]*peopleModule.StudentState, error) {
	records, err := read()
	if err != nil {
		return nil, RetainedStudentReadError(operation, MapError(err))
	}
	// nil, not an empty slice: the retained contract distinguishes them, and a
	// caller that ranges over the result sees no difference either way.
	if len(records) == 0 {
		return nil, nil
	}
	result := make([]*peopleModule.StudentState, 0, len(records))
	for _, record := range records {
		result = append(result, peopleModule.StudentStateFromRecord(peopleModule.StudentRecord(record)))
	}
	return result, nil
}

func studentRecordsByID(records []domain.StudentRecord) map[int64]*peopleModule.StudentState {
	result := make(map[int64]*peopleModule.StudentState, len(records))
	for _, record := range records {
		result[record.ID] = peopleModule.StudentStateFromRecord(peopleModule.StudentRecord(record))
	}
	return result
}
