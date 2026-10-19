package application

import (
	"context"
	"strconv"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (e Engine) ListStudentsByIDs(ctx context.Context, ids []int64) ([]peopledirectory.Student, error) {
	values, err := e.students.ListByIDs(ctx, ids)
	return toPublicStudents(values), MapError(err)
}

func (e Engine) ListStudentNamesByIDs(ctx context.Context, ids []int64) ([]peopledirectory.StudentName, error) {
	values, err := e.students.ListNamesByIDs(ctx, ids)
	result := make([]peopledirectory.StudentName, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.StudentName(value))
	}
	return result, MapError(err)
}

func (e Engine) ListStudentsAcrossTenantsByIDs(ctx context.Context, ids []int64) ([]peopledirectory.Student, error) {
	values, err := e.students.ListAcrossTenantsByIDs(ctx, ids)
	return toPublicStudents(values), MapError(err)
}

func (e Engine) ListStudentsByClasses(ctx context.Context, classes []string) ([]peopledirectory.Student, error) {
	values, err := e.students.ListByClasses(ctx, classes)
	return toPublicStudents(values), MapError(err)
}

func (e Engine) ListStudentsByPersonIDs(ctx context.Context, personIDs []int64) ([]peopledirectory.Student, error) {
	values, err := e.students.ListByPersonIDs(ctx, personIDs)
	return toPublicStudents(values), MapError(err)
}

func (e Engine) ListEnrolledStudents(ctx context.Context) ([]peopledirectory.Student, error) {
	values, err := e.students.ListEnrolled(ctx)
	return toPublicStudents(values), MapError(err)
}

func (e Engine) ListSchoolClasses(ctx context.Context) ([]string, error) {
	values, err := e.students.ListClasses(ctx)
	return values, MapError(err)
}

func (e Engine) ListStudentsWithStatusFlag(ctx context.Context, status string) ([]peopledirectory.Student, error) {
	values, err := e.students.ListByStatusFlag(ctx, status)
	return toPublicStudents(values), MapError(err)
}

func (e Engine) LockStudent(ctx context.Context, id int64) error {
	return MapError(e.students.Lock(ctx, id))
}

func (e Engine) SetFamilyProtection(ctx context.Context, input peopledirectory.SetFamilyProtection) (bool, error) {
	enabled, err := e.students.SetFamilyProtection(ctx, domain.FamilyProtectionChange{
		StudentID: input.StudentID, Enabled: input.Enabled,
		Reason: input.Reason, ActorAccountID: input.ActorAccountID,
	})
	return enabled, MapError(err)
}

func (e Engine) ListStudentDirectory(
	ctx context.Context,
	filter peopledirectory.StudentDirectoryFilter,
) ([]peopledirectory.StudentRecord, error) {
	values, err := e.students.ListDirectory(ctx, toDomainStudentDirectoryFilter(filter))
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]peopledirectory.StudentRecord, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.StudentRecord(value))
	}
	return result, nil
}

func (e Engine) CountStudentDirectory(
	ctx context.Context,
	filter peopledirectory.StudentDirectoryFilter,
) (int, error) {
	total, err := e.students.CountDirectory(ctx, toDomainStudentDirectoryFilter(filter))
	return total, MapError(err)
}

func (e Engine) ListStudentDirectoryIDs(ctx context.Context) ([]int64, error) {
	ids, err := e.students.ListDirectoryIDs(ctx)
	return ids, MapError(err)
}

func (e Engine) FindStudentRecordForMutation(ctx context.Context, studentID int64) (peopledirectory.StudentRecord, error) {
	record, err := e.students.FindRecord(ctx, studentID, "UPDATE")
	return peopledirectory.StudentRecord(record), MapError(err)
}

func (e Engine) ListStudentRecordsByID(ctx context.Context, ids []int64) ([]peopledirectory.StudentRecord, error) {
	values, err := e.students.ListRecordsByIDs(ctx, ids)
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]peopledirectory.StudentRecord, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.StudentRecord(value))
	}
	return result, nil
}

func (e Engine) LockStudentPhotoFeature(ctx context.Context) error {
	return mapPhotoError(e.studentPhotos.LockFeature(ctx))
}

// toDomainStudentDirectoryFilter renders the grade levels the SQL compares
// against: the column holds free text, so the match is on the decimal spelling
// of each level.
func toDomainStudentDirectoryFilter(filter peopledirectory.StudentDirectoryFilter) domain.StudentDirectoryFilter {
	levels := make([]string, 0, len(filter.GradeLevels))
	for _, level := range filter.GradeLevels {
		levels = append(levels, strconv.Itoa(level))
	}
	return domain.StudentDirectoryFilter{
		IDs: filter.IDs, SchoolClasses: filter.SchoolClasses, GradeLevels: levels,
		KeepAlumni: filter.KeepAlumni,
		CareStatus: filter.CareStatus, CareStatusOn: filter.CareStatusOn,
		Page: filter.Page, PageSize: filter.PageSize,
	}
}

func toPublicStudents(values []domain.Student) []peopledirectory.Student {
	result := make([]peopledirectory.Student, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.Student{
			ID: value.ID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, TenantID: value.TenantID,
			PersonID: value.PersonID, SchoolClass: value.SchoolClass, GroupID: value.GroupID, Status: value.Status,
			EnrolledFrom: value.EnrolledFrom, EnrolledUntil: value.EnrolledUntil,
			Sick: value.Sick, SickSince: value.SickSince, Excused: value.Excused, ExcusedSince: value.ExcusedSince,
			PhotoPath: value.PhotoPath,
		})
	}
	return result
}

func (e Engine) FindStudentRecord(ctx context.Context, studentID int64) (peopledirectory.StudentRecord, error) {
	record, err := e.students.FindRecord(ctx, studentID, "")
	return peopledirectory.StudentRecord(record), MapError(err)
}

func (e Engine) ListStudentRecordsByPerson(ctx context.Context, personIDs []int64) ([]peopledirectory.StudentRecord, error) {
	values, err := e.students.ListRecordsByPersonIDs(ctx, personIDs)
	return toPublicStudentRecords(values), MapError(err)
}

func (e Engine) ListStudentRecordsByGroup(
	ctx context.Context,
	groupIDs []int64,
	scope peopledirectory.StudentScope,
) ([]peopledirectory.StudentRecord, error) {
	values, err := e.students.ListRecordsByGroups(ctx, groupIDs, string(scope))
	return toPublicStudentRecords(values), MapError(err)
}

func (e Engine) ListStudentRecords(
	ctx context.Context,
	scope peopledirectory.StudentScope,
) ([]peopledirectory.StudentRecord, error) {
	values, err := e.students.ListRecords(ctx, string(scope))
	return toPublicStudentRecords(values), MapError(err)
}

func (e Engine) ListStudentRecordsByClass(
	ctx context.Context,
	classes []string,
	scope peopledirectory.StudentScope,
) ([]peopledirectory.StudentRecord, error) {
	values, err := e.students.ListRecordsByClasses(ctx, classes, string(scope))
	return toPublicStudentRecords(values), MapError(err)
}

func (e Engine) ListStudentRecordsDueForStatus(ctx context.Context, status, bound, asOf string) ([]peopledirectory.StudentRecord, error) {
	values, err := e.students.ListRecordsDueForStatus(ctx, status, bound, asOf)
	return toPublicStudentRecords(values), MapError(err)
}

func (e Engine) LockStudentRecordsByID(ctx context.Context, ids []int64) ([]peopledirectory.StudentRecord, error) {
	values, err := e.students.LockRecordsByIDs(ctx, ids)
	return toPublicStudentRecords(values), MapError(err)
}

func (e Engine) CountStudentsByGroup(ctx context.Context, groupIDs []int64) (map[int64]int, error) {
	counts, err := e.students.CountByGroups(ctx, groupIDs)
	return counts, MapError(err)
}

func (e Engine) ListEnrolledStudentIDsByNameAndBirthday(
	ctx context.Context,
	tenantID int64,
	firstName, lastName, birthday string,
) ([]int64, error) {
	ids, err := e.students.ListEnrolledIDsByNameAndBirthday(ctx, tenantID, firstName, lastName, birthday)
	return ids, MapError(err)
}

func toPublicStudentRecords(values []domain.StudentRecord) []peopledirectory.StudentRecord {
	result := make([]peopledirectory.StudentRecord, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.StudentRecord(value))
	}
	return result
}

func (e Engine) ListAllStudentIDs(ctx context.Context) ([]int64, error) {
	ids, err := e.students.ListAllIDs(ctx)
	return ids, MapError(err)
}

func (e Engine) FindStudentRecordForMutationNoWait(
	ctx context.Context,
	studentID int64,
) (peopledirectory.StudentRecord, error) {
	record, err := e.students.FindRecord(ctx, studentID, "UPDATE NOWAIT")
	return peopledirectory.StudentRecord(record), MapError(err)
}

func (e Engine) ListStudentRoster(ctx context.Context, today string) ([]peopledirectory.StudentRosterEntry, error) {
	values, err := e.students.ListRoster(ctx, today)
	return toPublicRoster(values), MapError(err)
}

func (e Engine) ListStudentRosterByGroup(ctx context.Context, groupIDs []int64, today string) ([]peopledirectory.StudentRosterEntry, error) {
	values, err := e.students.ListRosterByGroups(ctx, groupIDs, today)
	return toPublicRoster(values), MapError(err)
}

func (e Engine) ListStudentRosterOverlapping(ctx context.Context, from, to, today string) ([]peopledirectory.StudentRosterEntry, error) {
	values, err := e.students.ListRosterOverlapping(ctx, from, to, today)
	return toPublicRoster(values), MapError(err)
}

func toPublicRoster(values []domain.StudentRosterEntry) []peopledirectory.StudentRosterEntry {
	result := make([]peopledirectory.StudentRosterEntry, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.StudentRosterEntry{
			Record:    peopledirectory.StudentStateFromRecord(peopledirectory.StudentRecord(value.Record)),
			FirstName: value.FirstName, LastName: value.LastName,
			TagID: value.TagID, AccountID: value.AccountID,
		})
	}
	return result
}
